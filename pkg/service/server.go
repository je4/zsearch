package service

import (
	"context"
	"fmt"
	"github.com/goph/emperror"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/op/go-logging"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/mtsolr"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/source"
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
	log            *logging.Logger
	accesslog      io.Writer
}

func NewServer(mts *mtsolr.MTSolr,
	detailTemplate,
	errorTemplate,
	addr,
	mediaserver string,
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
	detailTpl, err := template.ParseFiles(detailTemplate)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot parse detail template %s", detailTemplate)
	}
	errorTpl, err := template.ParseFiles(errorTemplate)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot parse error template %s", errorTemplate)
	}

	return &Server{
		mts:            mts,
		host:           host,
		port:           port,
		detailTemplate: detailTpl,
		errorTemplate:  errorTpl,
		mediaserver:    mediaserver,
		log:            log,
		accesslog:      accesslog,
		staticPrefix:   staticPrefix,
		staticDir:      staticDir,
		privatePrefix:  privatePrefix,
		publicPrefix:   publicPrefix,
	}, nil
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

	media := map[string]source.MediaList{}
	for t, ml := range doc.Source.Media {
		if _, ok := media[t]; !ok {
			media[t] = source.MediaList{}
		}
		for _, m := range ml {
			m.Uri = strings.Replace(m.Uri, "mediaserver:", s.mediaserver, -1)
			media[t] = append(media[t], m)
		}
	}
	doc.Source.Media = media

	s.detailTemplate.Execute(w, doc)
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
