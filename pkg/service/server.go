package service

import (
	"context"
	"fmt"
	"github.com/goph/emperror"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/op/go-logging"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/source"
	"html/template"
	"io"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Notification struct {
	Id      template.JS
	Message string
}

type Status struct {
	Notifications []Notification
	Doc           *source.Document
	User          *User
	MetaPublic    bool
	ContentPublic bool
	MetaOK        bool
	ContentOK     bool
	Self          string
	SelfPath      string
	LoginUrl      string
}

type Server struct {
	mts               *source.MTSolr
	srv               *http.Server
	userCache         *UserCache
	host              string
	port              string
	staticDir         string
	publicPrefix      string
	privatePrefix     string
	staticPrefix      string
	jwtKey            string
	jwtAlg            []string
	loginUrl          string
	loginIssuer       string
	guestGroup        string
	adminGroup        string
	detailTemplate    *template.Template
	errorTemplate     *template.Template
	forbiddenTemplate *template.Template
	mediaserver       string
	mediaserverkey    string
	log               *logging.Logger
	accesslog         io.Writer
}

func NewServer(
	mts *source.MTSolr,
	uc *UserCache,
	detailTemplate,
	errorTemplate,
	forbiddenTemplate []string,
	addr,
	mediaserver,
	mediaserverkey string,
	log *logging.Logger,
	accesslog io.Writer,
	staticPrefix,
	staticDir,
	jwtKey string,
	jwtAlg []string,
	loginUrl,
	loginIssuer string,
	guestGroup string,
	adminGroup string,
	privatePrefix,
	publicPrefix string) (*Server, error) {
	staticPrefix = "/" + strings.Trim(staticPrefix, "/") + "/"
	privatePrefix = "/" + strings.Trim(privatePrefix, "/") + "/"
	publicPrefix = "/" + strings.Trim(publicPrefix, "/") + "/"
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		//log.Panicf("cannot split address %s: %v", addr, err)
		return nil, emperror.Wrapf(err, "cannot split address %s", addr)
	}

	srv := &Server{
		mts:            mts,
		userCache:      uc,
		host:           host,
		port:           port,
		mediaserver:    mediaserver,
		mediaserverkey: mediaserverkey,
		log:            log,
		accesslog:      accesslog,
		staticPrefix:   staticPrefix,
		staticDir:      staticDir,
		jwtKey:         jwtKey,
		jwtAlg:         jwtAlg,
		loginUrl:       loginUrl,
		loginIssuer:    loginIssuer,
		guestGroup:     guestGroup,
		adminGroup:     adminGroup,
		privatePrefix:  privatePrefix,
		publicPrefix:   publicPrefix,
	}
	if err := srv.Init(detailTemplate, errorTemplate, forbiddenTemplate); err != nil {
		return nil, emperror.Wrapf(err, "cannot initialize server")
	}
	return srv, nil
}

func (s *Server) Init(detailTemplate, errorTemplate, forbiddenTemplate []string) (err error) {
	mediaMatch := regexp.MustCompile(`^mediaserver:([^/]+)/([^/]+)$`)
	s.detailTemplate, err = template.New("details.amp.gohtml").
		Funcs(template.FuncMap{
			// incrementing a value
			"add": func(value, increment int) int {
				return value + increment
			},
			"mediachild": func(uri, child string) string {
				matches := mediaMatch.FindStringSubmatch(uri)
				if matches == nil {
					return uri
				}
				return uri + child
			},
			"medialink": func(uri, action, param string, token bool) string {
				matches := mediaMatch.FindStringSubmatch(uri)
				params := strings.Split(param, "/")
				sort.Strings(params)
				// if not matching, just return the uri
				if matches == nil {
					return uri
				}
				collection := matches[1]
				signature := matches[2]
				url := fmt.Sprintf("%s/%s/%s/%s/%s", s.mediaserver, collection, signature, action, param)
				if token {
					jwt, err := NewJWT(
						s.mediaserverkey,
						strings.TrimRight(fmt.Sprintf("mediaserver:%s/%s/%s/%s", collection, signature, action, strings.Join(params, "/")), "/"),
						"HS256",
						3600,
						"mediaserver",
						"mediathek",
						"")
					if err != nil {
						return fmt.Sprintf("ERROR: %v", err)
					}
					url = fmt.Sprintf("%s?token=%s", url, jwt)
				}
				return url
			},
		}).ParseFiles(detailTemplate...)
	//ParseFiles(detailTemplate)
	//s.detailTemplate, err = template.ParseFiles(detailTemplate)
	if err != nil {
		return emperror.Wrapf(err, "cannot parse detail template %v", detailTemplate)
	}
	s.errorTemplate, err = template.New("error.gohtml").ParseFiles(errorTemplate...)
	if err != nil {
		return emperror.Wrapf(err, "cannot parse error template %v", errorTemplate)
	}
	s.forbiddenTemplate, err = template.New("forbidden.amp.gohtml").ParseFiles(forbiddenTemplate...)
	if err != nil {
		return emperror.Wrapf(err, "cannot parse forbidden template %v", forbiddenTemplate)
	}
	return nil
}

func (s *Server) DoPanicf(writer http.ResponseWriter, status int, message string, a ...interface{}) (err error) {
	msg := fmt.Sprintf(message, a...)
	s.DoPanic(writer, status, msg)
	return
}

func (s *Server) DoPanic(writer http.ResponseWriter, status int, message string) (err error) {
	type errData struct {
		Status     int
		StatusText string
		Message    string
	}
	s.log.Error(message)
	data := errData{
		Status:     status,
		StatusText: http.StatusText(status),
		Message:    message,
	}
	writer.WriteHeader(status)
	// if there's no error Template, there's no help...
	s.errorTemplate.Execute(writer, data)
	return
}

func (s *Server) userFromToken(tokenstring, signature string) (*User, error) {

	// jwt valid?
	claims, err := CheckJWTValid(tokenstring, s.jwtKey, s.jwtAlg)
	if err != nil {
		return nil, emperror.Wrapf(err, "invalid access token")
	}

	// check whether token is from login service
	issuer, ok := claims["iss"]
	if !ok {
		return nil, emperror.Wrapf(err, "no iss in token %v", tokenstring)
	}
	issuerstr, ok := issuer.(string)
	if !ok {
		return nil, emperror.Wrapf(err, "iss not a string in token %v", tokenstring)
	}

	// token from login
	var user *User
	if issuerstr == s.loginIssuer {
		user, err = s.GetClaimUser(claims)
		if err != nil {
			return nil, emperror.Wrapf(err, "cannot extract userdata from token %v", tokenstring)
		}
		user.LoggedIn = true
		user.LoggedOut = false

		s.userCache.SetUser(user, user.Id)
	} else {
		// sub given?
		sub, err := GetClaim(claims, "sub")
		if err != nil {
			return nil, emperror.Wrapf(err, "no sub in token")
		}
		// sub correct?
		if strings.ToLower(sub) != strings.ToLower(signature) {
			return nil, emperror.Wrapf(err, "invalid sub %s (should be %s) in token", sub, signature)
		}
		// user given?
		userstr, err := GetClaim(claims, "user")
		if err != nil {
			return nil, emperror.Wrapf(err, "no user in token")
		}
		// user is string?

		user, err = s.userCache.GetUser(userstr)
		// user not found --> log out and become a guest
		if err != nil {
			return nil, emperror.Wrapf(err, "user timed out")
		} else {
			user.LoggedOut = false
			user.LoggedIn = true
		}
	}
	return user, nil
}

func (s *Server) ListenAndServe(cert, key string) error {
	router := mux.NewRouter()

	userRegexp := regexp.MustCompile(fmt.Sprintf("^/(%s|%s)/(.+)/user$",
		strings.Trim(s.publicPrefix, "/"),
		strings.Trim(s.privatePrefix, "/")))
	router.
		MatcherFunc(func(r *http.Request, rm *mux.RouteMatch) bool {
			matches := userRegexp.FindSubmatch([]byte(r.URL.Path))
			if len(matches) == 0 {
				return false
			}
			rm.Vars = map[string]string{}
			rm.Vars["access"] = string(matches[1])
			rm.Vars["signature"] = string(matches[2])
			return true
		}).HandlerFunc(s.userHandler)

	// https://data.mediathek.hgk.fhnw.ch/[access]/[signature]
	mainRegexp := regexp.MustCompile(fmt.Sprintf("^/(%s|%s)/(.+)$",
		strings.Trim(s.publicPrefix, "/"),
		strings.Trim(s.privatePrefix, "/")))
	router.
		MatcherFunc(func(r *http.Request, rm *mux.RouteMatch) bool {
			matches := mainRegexp.FindSubmatch([]byte(r.URL.Path))
			if len(matches) == 0 {
				return false
			}
			rm.Vars = map[string]string{}
			rm.Vars["access"] = string(matches[1])
			rm.Vars["signature"] = string(matches[2])
			return true
		}).HandlerFunc(s.detailHandler)

	// the static fileserver
	router.
		PathPrefix(s.staticPrefix).
		Handler(http.StripPrefix(s.staticPrefix, http.FileServer(http.Dir(s.staticDir))))

	loggedRouter := handlers.LoggingHandler(s.accesslog, router)
	addr := net.JoinHostPort(s.host, s.port)
	s.srv = &http.Server{
		Handler: loggedRouter,
		Addr:    addr,
	}
	if cert != "" && key != "" {
		s.log.Infof("starting HTTPS memoServer at https://%v", addr)
		return s.srv.ListenAndServeTLS(cert, key)
	} else {
		s.log.Infof("starting HTTP memoServer at http://%v", addr)
		return s.srv.ListenAndServe()
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func (s *Server) GetClaimUser(claims map[string]interface{}) (*User, error) {
	id, err := GetClaim(claims, "userId")
	if err != nil {
		return nil, emperror.Wrapf(err, "no userid in key")
	}
	groupstr, err := GetClaim(claims, "groups")
	if err != nil {
		groupstr = "global/guest"
	}
	groups := strings.Split(groupstr, ";")
	firstName, _ := GetClaim(claims, "firstName")
	lastName, _ := GetClaim(claims, "lastName")
	homeOrg, _ := GetClaim(claims, "homeOrg")
	email, _ := GetClaim(claims, "email")
	expval, ok := claims["exp"]
	if !ok {
		return nil, emperror.Wrapf(err, "no exp in key")
	}
	exp, ok := expval.(float64)
	if !ok {
		return nil, emperror.Wrapf(err, "exp not an ")
	}
	u := &User{
		Server:    s,
		Id:        id,
		Groups:    groups,
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
		HomeOrg:   homeOrg,
		Exp:       time.Unix(int64(exp), 0),
	}
	return u, nil
}
