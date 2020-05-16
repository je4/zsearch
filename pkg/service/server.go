package service

import (
	"context"
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
	"time"
)

type Status struct {
	Doc       *mtsolr.Document
	User      *User
	MetaOK    bool
	ContentOK bool
	Self      string
	LoggedIn  bool
	LoginUrl   string
}

type Server struct {
	mts               *mtsolr.MTSolr
	srv               *http.Server
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
	detailTemplate    *template.Template
	errorTemplate     *template.Template
	forbiddenTemplate *template.Template
	mediaserver       string
	mediaserverkey    string
	log               *logging.Logger
	accesslog         io.Writer
	userCache         *UserCache
}

func NewServer(mts *mtsolr.MTSolr,
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
		privatePrefix:  privatePrefix,
		publicPrefix:   publicPrefix,
		userCache:      NewUserCache(),
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
			"medialink": func(uri, action, param string) string {
				matches := mediaMatch.FindStringSubmatch(uri)
				params := strings.Split(param, "/")
				sort.Strings(params)
				// if not matching, just return the uri
				if matches == nil {
					return uri
				}
				collection := matches[1]
				signature := matches[2]
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
				url := fmt.Sprintf("%s/%s/%s/%s/%s?token=%s", s.mediaserver, collection, signature, action, param, jwt)
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
		Self:      fmt.Sprintf("%s://%s/%s", proto, req.Host, req.URL.String()),
		LoggedIn:  false,
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

		// jwt valid?
		claims, err := CheckJWTValid(tokenstring, s.jwtKey, s.jwtAlg)
		if err != nil {
			s.DoPanicf(w, http.StatusForbidden, "invalid token %v", err)
			return
		}

		// check whether token is from login service
		issuer, ok := claims["iss"]
		if !ok {
			s.DoPanicf(w, http.StatusForbidden, "no iss in key %v", tokenstring)
			return
		}
		issuerstr, ok := issuer.(string)
		if !ok {
			s.DoPanicf(w, http.StatusForbidden, "iss not a string in key %v", tokenstring)
			return
		}

		if issuerstr == s.loginIssuer {
			u, err := GetClaimUser(claims)
			if err != nil {
				s.DoPanicf(w, http.StatusForbidden, "cannot extract userdata from key %v", tokenstring)
				return
			}
			status.User = u
			status.LoggedIn = true
		} else {
			// sub given?
			sub, err := GetClaim(claims, "sub")
			if err != nil {
				s.DoPanicf(w, http.StatusForbidden, "no sub in key %v: %v", tokenstring, err)
				return
			}
			// sub correct?
			if strings.ToLower(sub) != strings.ToLower(signature) {
				s.DoPanicf(w, http.StatusForbidden, "invalid sub %s (should be %s) in key %v", sub, signature, tokenstring)
				return
			}
			// user given?
			user, err := GetClaim(claims, "user")
			if err != nil {
				s.DoPanicf(w, http.StatusForbidden, "no user in key %v: %v", tokenstring, err)
				return
			}
			// user is string?

			status.User, err = s.userCache.GetUser(user)
			if err != nil {
				s.DoPanicf(w, http.StatusForbidden, "user not a string in key %v", tokenstring)
				return
			}
			status.LoggedIn = true
		}
	} else {
		status.User = &User{
			Id:        "0",
			Groups:    []string{"global/guest"},
			Email:     "",
			FirstName: "",
			LastName:  "Guest",
			HomeOrg:   "",
			Exp:       time.Now().Add(time.Hour * 24),
		}
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
