package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/goph/emperror"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/op/go-logging"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/mtsolr"
	"html/template"
	"io"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strings"
)

type Status struct {
	Doc           *mtsolr.Document
	User          *User
	MetaPublic    bool
	ContentPublic bool
	MetaOK        bool
	ContentOK     bool
	Self          string
	LoginUrl      string
}

type Server struct {
	mts               *mtsolr.MTSolr
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
	mts *mtsolr.MTSolr,
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
						"mediathek")
					if err != nil {
						return fmt.Sprintf("ERROR: %v", err)
					}
					url = fmt.Sprintf("%s?token=%s", url, jwt )
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
		return nil, emperror.Wrapf(err, "invalid token %v", tokenstring)
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
		user, err = GetClaimUser(claims)
		if err != nil {
			return nil, emperror.Wrapf(err, "cannot extract userdata from token %v", tokenstring)
		}
		user.LoggedIn = true
		user.LoggedOut = false
	} else {
		// sub given?
		sub, err := GetClaim(claims, "sub")
		if err != nil {
			return nil, emperror.Wrapf(err, "no sub in token %v", tokenstring)
		}
		// sub correct?
		if strings.ToLower(sub) != strings.ToLower(signature) {
			return nil, emperror.Wrapf(err, "invalid sub %s (should be %s) in token %v", sub, signature, tokenstring)
		}
		// user given?
		userstr, err := GetClaim(claims, "user")
		if err != nil {
			return nil, emperror.Wrapf(err, "no user in token %v", tokenstring)
		}
		// user is string?

		user, err = s.userCache.GetUser(userstr)
		// user not found --> log out and become a guest
		if err != nil {
			user = NewGuestUser()
			user.LoggedOut = true
			user.LoggedIn = false
		} else {
			user.LoggedOut = false
			user.LoggedIn = true
		}
	}
	return user, nil
}

func (s *Server) userHandler(w http.ResponseWriter, req *http.Request) {
	// remove prefix and use whole rest of url as signature
	vars := mux.Vars(req)
	_, ok := vars["access"]
	if !ok {
		s.DoPanicf(w, http.StatusBadRequest, "no accesstype in url: %s", req.URL.Path)
		return
	}
	signature, ok := vars["signature"]
	if !ok {
		s.DoPanicf(w, http.StatusBadRequest, "no signature in url: %s", req.URL.Path)
		return
	}

	var user *User
	var err error
	jwt, ok := req.URL.Query()["token"]
	if ok {
		// jwt in parameter?
		if len(jwt) == 0 {
			s.DoPanicf(w, http.StatusForbidden, "invalid token %v", jwt)
			return
		}
		tokenstring := jwt[0]
		user, err = s.userFromToken(tokenstring, signature)
		if err != nil {
			s.DoPanicf(w, http.StatusForbidden, "%v", err)
			return
		}
	}


	js, err := json.Marshal(user)
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot marshal user: %v", user)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func (s *Server) mainHandler(w http.ResponseWriter, req *http.Request) {
	// remove prefix and use whole rest of url as signature
	vars := mux.Vars(req)
	_, ok := vars["access"]
	if !ok {
		s.DoPanicf(w, http.StatusBadRequest, "no accesstype in url: %s", req.URL.Path)
		return
	}
	signature, ok := vars["signature"]
	if !ok {
		s.DoPanicf(w, http.StatusBadRequest, "no signature in url: %s", req.URL.Path)
		return
	}

	var proto string
	if req.TLS != nil {
		proto = "https"
	} else {
		proto = "http"
	}
	status := Status{
		Doc:       nil,
		User:      nil,
		ContentOK: false,
		MetaOK:    false,
		Self:      fmt.Sprintf("%s://%s/%s", proto, req.Host, strings.TrimLeft(req.URL.Path, "/")),
		LoginUrl:  s.loginUrl,
	}
	var err error
	status.Doc, err = s.mts.LoadEntity(signature)
	if err != nil {
		s.DoPanicf(w, http.StatusNotFound, "error loading signature %s: %v", signature, err)
		return
	}

	jwt, ok := req.URL.Query()["token"]
	if ok {
		// jwt in parameter?
		if len(jwt) == 0 {
			s.DoPanicf(w, http.StatusForbidden, "invalid token %v", jwt)
			return
		}
		tokenstring := jwt[0]
		user, err := s.userFromToken(tokenstring, signature)
		if err != nil {
			s.DoPanicf(w, http.StatusForbidden, "%v", err)
			return
		}
		status.User = user
	} else {
		status.User = NewGuestUser()
	}
	for acl, groups := range status.Doc.ACL {
		for _, group := range groups {
			for _, ugroup := range status.User.Groups {
				if group == ugroup {
					switch acl {
					case "meta":
						status.MetaOK = true
					case "content":
						status.ContentOK = true
					}
				}
			}
			if group == s.guestGroup {
				switch acl {
				case "meta":
					status.MetaPublic = true
				case "content":
					status.ContentPublic = true
				}
			}
		}
	}

	for _, ugroup := range status.User.Groups {
		if s.adminGroup == ugroup {
			status.MetaOK = true
			status.ContentOK = true
		}
	}

	if !status.MetaOK {
		w.WriteHeader(http.StatusForbidden)
		// if there's no error Template, there's no help...
		err = s.forbiddenTemplate.Execute(w, status)
		return
	}

	err = s.detailTemplate.Execute(w, status)
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot parse template: %+v", err)
		return
	}
	//	w.Write([]byte(fmt.Sprintf("%s/%s", access, signature)))
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
		}).HandlerFunc(s.mainHandler)

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
