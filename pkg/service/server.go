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
	"strings"
)

type Server struct {
	mts            *mtsolr.MTSolr
	srv            *http.Server
	host           string
	port           string
	staticDir      string
	publicPrefix   string
	privatePrefix  string
	staticPrefix   string
	detailTemplate *template.Template
	errorTemplate  *template.Template
	mediaserver    string
	mediaserverkey string
	log            *logging.Logger
	accesslog      io.Writer
}

func NewServer(mts *mtsolr.MTSolr,
	detailTemplate []string,
	errorTemplate,
	addr,
	mediaserver,
	mediaserverkey string,
	log *logging.Logger,
	accesslog io.Writer,
	staticPrefix,
	staticDir,
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
		privatePrefix:  privatePrefix,
		publicPrefix:   publicPrefix,
	}
	if err := srv.Init(detailTemplate, errorTemplate); err != nil {
		return nil, emperror.Wrapf(err, "cannot initialize server")
	}
	return srv, nil
}

func (s *Server) Init(detailTemplate []string, errorTemplate string) (err error) {
	mediaMatch := regexp.MustCompile(`^mediaserver:([^/]+)/([^/]+)$`)
	s.detailTemplate, err = template.New("details.amp.gohtml").
	Funcs(template.FuncMap{
		"medialink": func(uri, action, param string) string {
			matches := mediaMatch.FindStringSubmatch(uri)
			// if not matching, just return the uri
			if matches == nil {
				return uri
			}
			collection := matches[1]
			signature := matches[2]
			jwt, err := NewJWT(
				s.mediaserverkey,
				fmt.Sprintf("mediaserver:%s/%s/%s/%s", collection, signature, action, param),
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
		return emperror.Wrapf(err, "cannot parse detail template %s", detailTemplate)
	}
	s.errorTemplate, err = template.ParseFiles(errorTemplate)
	if err != nil {
		return emperror.Wrapf(err, "cannot parse error template %s", errorTemplate)
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
	doc, err := s.mts.LoadEntity(signature)
	if err != nil {
		s.DoPanicf(w, http.StatusNotFound, "error loading signature %s: %v", signature, err)
		return
	}

	err = s.detailTemplate.Execute(w, doc)
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot parse template: %+v", err)
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
