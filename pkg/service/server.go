package service

import (
	"bufio"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/Masterminds/sprig"
	"github.com/goph/emperror"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/op/go-logging"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/amp"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/generic"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/source"
	"html/template"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Notification struct {
	Id      template.JS
	Message string
}

type DetailStatus struct {
	Type          string
	Notifications []Notification
	Doc           *source.Document
	User          *User
	MetaPublic    bool
	ContentPublic bool
	MetaOK        bool
	ContentOK     bool
	IsAmp         bool
	Self          string
	SelfPath      string
	AmpBase       string
	LoginUrl      string
	Title         string
}

type SearchStatus struct {
	Type          string
	Notifications []Notification
	User          *User
	Self          string
	SelfPath      string
	AmpBase       string
	LoginUrl      string
	Title         string
	SearchResult  template.JS
}

type Server struct {
	mts               *source.MTSolr
	srv               *http.Server
	userCache         *UserCache
	host              string
	port              string
	addrExt           string
	staticDir         string
	detailPrefix      string
	staticPrefix      string
	updatePrefix      string
	searchPrefix      string
	apiPrefix         string
	jwtKey            string
	jwtAlg            []string
	linkTokenExp      time.Duration
	loginUrl          string
	loginIssuer       string
	guestGroup        string
	adminGroup        string
	detailTemplate    *template.Template
	errorTemplate     *template.Template
	forbiddenTemplate *template.Template
	searchTemplate    *template.Template
	mediaserver       string
	mediaserverKey    string
	mediaTokenExp     time.Duration
	log               *logging.Logger
	accesslog         io.Writer
	ampApiKey         *rsa.PrivateKey
	ampCache          *amp.Cache
}

func NewServer(
	mts *source.MTSolr,
	uc *UserCache,
	detailTemplate,
	errorTemplate,
	forbiddenTemplate []string,
	searchTemplate []string,
	addr,
	addrExt,
	mediaserver,
	mediaserverkey string,
	mediatokenexp time.Duration,
	log *logging.Logger,
	accesslog io.Writer,
	staticPrefix,
	staticDir,
	jwtKey string,
	jwtAlg []string,
	linkTokenExp time.Duration,
	loginUrl,
	loginIssuer string,
	guestGroup string,
	adminGroup string,
	detailPrefix string,
	updatePrefix string,
	searchPrefix string,
	apiPrefix string,
	AmpCache string,
	ampApiKeyFile string,
) (*Server, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		//log.Panicf("cannot split address %s: %v", addr, err)
		return nil, emperror.Wrapf(err, "cannot split address %s", addr)
	}

	// load private api key
	privateKeyFile, err := os.Open(ampApiKeyFile)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot open %s", ampApiKeyFile)
	}
	pemfileinfo, _ := privateKeyFile.Stat()
	pembytes := make([]byte, pemfileinfo.Size())
	buffer := bufio.NewReader(privateKeyFile)
	_, err = buffer.Read(pembytes)
	data, _ := pem.Decode([]byte(pembytes))
	privateKeyFile.Close()
	ampApiKey, err := x509.ParsePKCS1PrivateKey(data.Bytes)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot parse private key %s", string(data.Bytes))
	}

	aCaches, err := amp.GetCaches()
	if err != nil {
		return nil, err
	}
	ampCache, _ := aCaches[AmpCache]
	/*
		if !ok {
			keys := reflect.ValueOf(aCaches).MapKeys()
			if len(keys) == 0 {
				return nil, errors.New("no AMP caches found")
			}
			ampCache = aCaches[keys[rand.Intn(len(keys))].Interface().(string)]
		}
	*/

	srv := &Server{
		mts:            mts,
		userCache:      uc,
		host:           host,
		port:           port,
		addrExt:        addrExt,
		mediaserver:    mediaserver,
		mediaserverKey: mediaserverkey,
		mediaTokenExp:  mediatokenexp,
		log:            log,
		accesslog:      accesslog,
		staticPrefix:   staticPrefix,
		staticDir:      staticDir,
		jwtKey:         jwtKey,
		jwtAlg:         jwtAlg,
		linkTokenExp:   linkTokenExp,
		loginUrl:       loginUrl,
		loginIssuer:    loginIssuer,
		guestGroup:     guestGroup,
		adminGroup:     adminGroup,
		detailPrefix:   detailPrefix,
		updatePrefix:   updatePrefix,
		searchPrefix:   searchPrefix,
		apiPrefix:      apiPrefix,
		ampCache:       ampCache,
		ampApiKey:      ampApiKey,
	}
	if err := srv.InitTemplates(detailTemplate, errorTemplate, forbiddenTemplate, searchTemplate); err != nil {
		return nil, emperror.Wrapf(err, "cannot initialize server")
	}
	return srv, nil
}

func (s *Server) InitTemplates(detailTemplate, errorTemplate, forbiddenTemplate, searchTemplate []string) (err error) {
	mediaMatch := regexp.MustCompile(`^mediaserver:([^/]+)/([^/]+)$`)

	funcMap := sprig.FuncMap()
	funcMap["url"] = func(value string) template.URL {
		return template.URL(value)
	}
	funcMap["js"] = func(value string) template.JS {
		return template.JS(value)
	}
	funcMap["htmlattr"] = func(value string) template.HTMLAttr {
		return template.HTMLAttr(value)
	}
	funcMap["jsstr"] = func(value string) template.JSStr {
		return template.JSStr(value)
	}
	funcMap["html"] = func(value string) template.HTML {
		return template.HTML(value)
	}
	funcMap["srcset"] = func(value string) template.Srcset {
		return template.Srcset(value)
	}
	funcMap["mediachild"] = func(uri, child string) string {
		matches := mediaMatch.FindStringSubmatch(uri)
		if matches == nil {
			return uri
		}
		return uri + child
	}
	funcMap["medialink"] = func(uri, action, param string, token bool) string {
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
			jwt, err := generic.NewJWT(
				s.mediaserverKey,
				strings.TrimRight(fmt.Sprintf("mediaserver:%s/%s/%s/%s", collection, signature, action, strings.Join(params, "/")), "/"),
				"HS256",
				int64(s.mediaTokenExp.Seconds()),
				"mediaserver",
				"mediathek",
				"")
			if err != nil {
				return fmt.Sprintf("ERROR: %v", err)
			}
			url = fmt.Sprintf("%s?token=%s", url, jwt)
		} else {
			if s.ampCache != nil {
				url, err = s.ampCache.BuildUrl(url, amp.IMAGE)
				if err != nil {
					return fmt.Sprintf("ERROR: %v", err)
				}
			}
		}
		return url
	}

	s.detailTemplate, err = template.New("details.amp.gohtml").Funcs(funcMap).ParseFiles(detailTemplate...)
	//ParseFiles(detailTemplate)
	//s.detailTemplate, err = template.ParseFiles(detailTemplate)
	if err != nil {
		return emperror.Wrapf(err, "cannot parse detail template %v", detailTemplate)
	}
	s.errorTemplate, err = template.New("error.gohtml").Funcs(funcMap).ParseFiles(errorTemplate...)
	if err != nil {
		return emperror.Wrapf(err, "cannot parse error template %v", errorTemplate)
	}
	s.forbiddenTemplate, err = template.New("forbidden.amp.gohtml").Funcs(funcMap).ParseFiles(forbiddenTemplate...)
	if err != nil {
		return emperror.Wrapf(err, "cannot parse forbidden template %v", forbiddenTemplate)
	}
	s.searchTemplate, err = template.New("search.amp.gohtml").Funcs(funcMap).ParseFiles(searchTemplate...)
	if err != nil {
		return emperror.Wrapf(err, "cannot parse forbidden template %v", forbiddenTemplate)
	}
	return nil
}

func (s *Server) DoPanicf(writer http.ResponseWriter, status int, message string, json bool, a ...interface{}) (err error) {
	msg := fmt.Sprintf(message, a...)
	if json {
		s.DoPanicJSON(writer, status, msg)
	} else {
		s.DoPanic(writer, status, msg)
	}
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

func (s *Server) DoPanicJSON(writer http.ResponseWriter, status int, message string) (err error) {
	type errData struct {
		Status     int `json:"status"`
		StatusText string `json:"statustext"`
		Message    string `json:"message"`
	}
	s.log.Error(message)
	data := errData{
		Status:     status,
		StatusText: http.StatusText(status),
		Message:    message,
	}
	writer.WriteHeader(status)
	jenc := json.NewEncoder(writer)
	jenc.Encode(data)
	return
}


func (s *Server) userFromToken(tokenstring, signature string) (*User, error) {

	// jwt valid?
	claims, err := generic.CheckJWTValid(tokenstring, s.jwtKey, s.jwtAlg)
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
		sub, err := generic.GetClaim(claims, "sub")
		if err != nil {
			return nil, emperror.Wrapf(err, "no sub in token")
		}
		// sub correct?
		if strings.ToLower(sub) != strings.ToLower(signature) {
			return nil, emperror.Wrapf(err, "invalid sub %s (should be %s) in token", sub, signature)
		}
		// user given?
		userstr, err := generic.GetClaim(claims, "user")
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

	// https://data.mediathek.hgk.fhnw.ch/search
	router.HandleFunc(fmt.Sprintf("/%s", s.searchPrefix), s.searchHandler).Methods("GET")

	// https://data.mediathek.hgk.fhnw.ch/api/search
	router.HandleFunc(fmt.Sprintf("/%s/search", s.apiPrefix), s.apiSearchHandler).Methods("GET", "POST")

	// https://data.mediathek.hgk.fhnw.ch/detail/[signature]
	mainRegexp := regexp.MustCompile(fmt.Sprintf("^/%s/(.+)$", s.detailPrefix))
	router.
		MatcherFunc(func(r *http.Request, rm *mux.RouteMatch) bool {
			matches := mainRegexp.FindSubmatch([]byte(r.URL.Path))
			if len(matches) == 0 {
				return false
			}
			rm.Vars = map[string]string{}
			rm.Vars["signature"] = string(matches[1])
			return true
		}).HandlerFunc(s.detailHandler).Methods("GET")

	// https://data.mediathek.hgk.fhnw.ch/update/[signature]
	updateRegexp := regexp.MustCompile(fmt.Sprintf("^/%s/(.+)$", s.updatePrefix))
	router.
		MatcherFunc(func(r *http.Request, rm *mux.RouteMatch) bool {
			matches := updateRegexp.FindSubmatch([]byte(r.URL.Path))
			if len(matches) == 0 {
				return false
			}
			rm.Vars = map[string]string{}
			rm.Vars["signature"] = string(matches[1])
			return true
		}).HandlerFunc(s.updateHandler).Methods("GET")

	// the static fileserver
	router.
		PathPrefix(fmt.Sprintf("/%s", s.staticPrefix)).
		Handler(http.StripPrefix("/"+s.staticPrefix, http.FileServer(http.Dir(s.staticDir)))).Methods("GET")

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
	id, err := generic.GetClaim(claims, "userId")
	if err != nil {
		return nil, emperror.Wrapf(err, "no userid in key")
	}
	groupstr, err := generic.GetClaim(claims, "groups")
	if err != nil {
		groupstr = "global/guest"
	}
	groups := strings.Split(groupstr, ";")
	firstName, _ := generic.GetClaim(claims, "firstName")
	lastName, _ := generic.GetClaim(claims, "lastName")
	homeOrg, _ := generic.GetClaim(claims, "homeOrg")
	email, _ := generic.GetClaim(claims, "email")
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
