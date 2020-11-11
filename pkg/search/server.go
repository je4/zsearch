/*
Copyright 2020 Center for Digital Matter HGK FHNW, Basel.
Copyright 2020 info-age GmbH, Basel.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS-IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package search

import (
	"bufio"
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/Masterminds/sprig"
	"github.com/bluele/gcache"
	"github.com/drgrib/maps"
	"github.com/goph/emperror"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/htfy96/reformism"
	"github.com/je4/zsearch/pkg/amp"
	"github.com/op/go-logging"
	"google.golang.org/api/customsearch/v1"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime/debug"
	"sort"
	"strings"
	"time"
)

type Notification struct {
	Id      template.JS
	Message string
}

type BaseStatus struct {
	Type          string
	Notifications []Notification
	User          *User
	Self          string
	Token         string
	BaseUrl       string
	Prefixes      map[string]string
	SelfPath      string
	AmpBase       string
	LoginUrl      string
	Title         string
}

type DetailStatus struct {
	BaseStatus
	Doc             *SourceData
	MetaPublic      bool
	ContentPublic   bool
	MetaOK          bool
	ContentOK       bool
	IsAmp           bool
	MetaDescription string
}

type FacetCountField struct {
	Id        string
	Name      string
	ShortName string
	Selected  bool
}

type SearchStatus struct {
	BaseStatus
	SearchResult        template.JS
	Result              *SearchResult
	QueryApi            template.URL
	SearchResultStart   int
	SearchResultRows    int
	SearchResultTotal   int
	SearchString        string
	Filter              map[string][]string
	SearchResultVisible bool
	Facet               map[string]map[string]FacetCountField
	CoreFacets          []string
	MetaDescription     string
}

type CollectionsStatus struct {
	BaseStatus

	SearchResult        template.JS
	Result              map[string][]*SourceData
	QueryApi            template.URL
	SearchResultStart   int
	SearchResultRows    int
	SearchResultTotal   int
	SearchResultVisible bool
	MetaDescription     string
}

type ImageSearchStatus struct {
	Type              string
	Notifications     []Notification
	User              *User
	Self              string
	Token             string
	BaseUrl           string
	SelfPath          string
	AmpBase           string
	LoginUrl          string
	Title             string
	SearchResult      template.JS
	Result            *SearchResult
	QueryApi          template.URL
	SearchResultStart int
	SearchResultRows  int
	SearchResultTotal int
	SearchString      string
	MetaDescription   string
}

type SubFilter struct {
	Name   string `toml:"name"`
	Label  string `toml:"label"`
	Filter string `toml:"filter"`
}

type NetGroups map[string][]*net.IPNet

type facetField struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	Selected bool   `json:"selected"`
}

type SearchResult struct {
	Items           []SearchResultItem    `json:"items"`
	Total           int64                 `json:"total"`
	Start           int64                 `json:"SearchResultStart"`
	Rows            int64                 `json:"Rows"`
	Query           string                `json:"query"`
	Search          string                `json:"search"`
	Next            string                `json:"next"`
	FacetFieldCount map[string]facetField `json:"facetfieldcount"`
}

type SearchResultItem struct {
	Id            string              `json:"Id"`
	Type          string              `json:"type"`
	Title         string              `json:"title"`
	Text          string              `json:"text"`
	Collection    string              `json:"collection"`
	Authors       []string            `json:"authors"`
	AuthorText    string              `json:"authortext"`
	Link          string              `json:"link"`
	FirstItem     bool                `json:"firstitem"`
	Total         int64               `json:"total,omitempty"`
	Date          string              `json:"date"`
	Icon          string              `json:"icon"`
	Media         map[string]int      `json:"media"`
	MetaPublic    bool                `json:"metapublic"`
	ContentPublic bool                `json:"contentpublic"`
	MetaOK        bool                `json:"metaok"`
	ContentOK     bool                `json:"contentok"`
	Poster        *Media              `json:"poster"`
	Highlight     map[string][]string `json:"highlight"`
}

func (ng NetGroups) Contains(str string) []string {
	var groups []string

	ip := net.ParseIP(str)
	for grp, nets := range ng {
		for _, n := range nets {
			if n.Contains(ip) {
				groups = append(groups, grp)
				break
			}
		}
	}

	return groups
}

type Server struct {
	mts                *Search
	srv                *http.Server
	userCache          *UserCache
	host               string
	port               string
	addrExt            string
	prefixes           map[string]string
	staticDir          string
	staticCacheControl string
	jwtKey             string
	jwtAlg             []string
	linkTokenExp       time.Duration
	loginUrl           string
	loginIssuer        string
	guestGroup         string
	adminGroup         string
	templates          map[string]*template.Template
	templatesFiles     map[string][]string
	templateDev        bool
	mediaserver        string
	mediaserverKey     string
	mediaTokenExp      time.Duration
	log                *logging.Logger
	accesslog          io.Writer
	ampApiKey          *rsa.PrivateKey
	ampCache           *amp.Cache
	searchFields       map[string]string
	facets             SolrFacetList
	locations          NetGroups
	icons              map[string]string
	baseFilter         string
	subFilters         []SubFilter
	funcMap            template.FuncMap
	collectionsCatalog string
	queryCache         gcache.Cache
	google             *customsearch.Service
	googleCSEKey       map[string]string
}

func NewServer(
	mts *Search,
	uc *UserCache,
	google *customsearch.Service,
	templateFiles map[string][]string,
	templateDev bool,
	addr,
	addrExt,
	mediaserver,
	mediaserverkey string,
	mediatokenexp time.Duration,
	log *logging.Logger,
	accesslog io.Writer,
	prefixes map[string]string,
	staticDir,
	staticCacheControl,
	jwtKey string,
	jwtAlg []string,
	linkTokenExp time.Duration,
	loginUrl,
	loginIssuer string,
	guestGroup string,
	adminGroup string,
	AmpCache string,
	ampApiKeyFile string,
	searchFields map[string]string,
	facets SolrFacetList,
	locations NetGroups,
	icons map[string]string,
	baseFilter string,
	subFilter []SubFilter,
	collectionsCatalog string,
	googleCSEKey map[string]string,
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

	srv := &Server{
		mts:                mts,
		userCache:          uc,
		google:             google,
		host:               host,
		port:               port,
		addrExt:            addrExt,
		prefixes:           prefixes,
		mediaserver:        mediaserver,
		mediaserverKey:     mediaserverkey,
		mediaTokenExp:      mediatokenexp,
		log:                log,
		accesslog:          accesslog,
		staticDir:          staticDir,
		staticCacheControl: staticCacheControl,
		templateDev:        templateDev,
		jwtKey:             jwtKey,
		jwtAlg:             jwtAlg,
		linkTokenExp:       linkTokenExp,
		loginUrl:           loginUrl,
		loginIssuer:        loginIssuer,
		guestGroup:         guestGroup,
		adminGroup:         adminGroup,
		ampCache:           ampCache,
		ampApiKey:          ampApiKey,
		searchFields:       searchFields,
		facets:             facets,
		locations:          locations,
		icons:              icons,
		baseFilter:         baseFilter,
		subFilters:         subFilter,
		templates:          make(map[string]*template.Template),
		funcMap:            template.FuncMap{},
		collectionsCatalog: collectionsCatalog,
		queryCache:         gcache.New(100).ARC().Expiration(time.Hour * 3).Build(),
		googleCSEKey:       googleCSEKey,
		templatesFiles:     templateFiles,
	}
	if err := srv.InitTemplates(); err != nil {
		return nil, emperror.Wrapf(err, "cannot initialize server")
	}
	return srv, nil
}

func solrOr(field string, values []string, weight1, weight2 int) string {
	result := ""
	for _, val := range values {
		trimmed := strings.Trim(val, `"`)
		withQuotes := val != trimmed
		if withQuotes {
			val = trimmed
		}
		if weight1 > 0 {
			if result != "" {
				result += " OR "
			}
			result += fmt.Sprintf(`%s:%s^%d`, field, EscapeSolrString(val), weight1)
		}
		if weight2 > 0 && !withQuotes {
			if result != "" {
				result += " OR "
			}
			result += fmt.Sprintf(`%s:%s*^%d`, field, EscapeSolrString(val), weight2)
		}
	}
	return result
}

func initTemplate(tpl []string, name string, funcMap template.FuncMap) (*template.Template, error) {
	newTpl, err := template.New(name).Funcs(funcMap).ParseFiles(tpl...)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot parse error template %v - %v", name, tpl)
	}
	return newTpl, nil
}

func (s *Server) InitTemplates() (err error) {
	mediaMatch := regexp.MustCompile(`^mediaserver:([^/]+)/([^/]+)$`)

	for key, val := range sprig.FuncMap() {
		s.funcMap[key] = val
	}

	for key, val := range reformism.FuncsHTML {
		s.funcMap[key] = val
	}

	s.funcMap["url"] = func(value string) template.URL {
		return template.URL(value)
	}

	s.funcMap["js"] = func(value string) template.JS {
		return template.JS(value)
	}
	s.funcMap["nl2br"] = func(value string) template.HTML {
		safe := template.HTMLEscapeString(value)
		safe = strings.Replace(safe, "\n", "<br>", -1)
		return template.HTML(safe)
	}
	s.funcMap["htmlattr"] = func(value string) template.HTMLAttr {
		return template.HTMLAttr(value)
	}
	s.funcMap["jsstr"] = func(value string) template.JSStr {
		return template.JSStr(value)
	}
	s.funcMap["html"] = func(value string) template.HTML {
		return template.HTML(value)
	}
	s.funcMap["srcset"] = func(value string) template.Srcset {
		return template.Srcset(value)
	}
	s.funcMap["resizeW"] = func(width, height, targetW, targetH int64) int64 {
		aspect := float64(width) / float64(height)
		targetAspect := float64(targetW) / float64(targetH)
		if aspect > targetAspect {
			w := float64(targetW)
			//h := w/aspect
			return int64(w)
		} else {
			h := float64(targetH)
			w := h * aspect
			return int64(w)
		}
	}
	s.funcMap["resizeH"] = func(width, height, targetW, targetH int64) int64 {
		aspect := float64(width) / float64(height)
		targetAspect := float64(targetW) / float64(targetH)
		if aspect > targetAspect {
			w := float64(targetW)
			h := w / aspect
			return int64(h)
		} else {
			h := float64(targetH)
			//w := h*aspect
			return int64(h)
		}
	}
	s.funcMap["mediachild"] = func(uri, child string) string {
		matches := mediaMatch.FindStringSubmatch(uri)
		if matches == nil {
			return uri
		}
		return uri + child
	}
	s.funcMap["searchtokenparam"] = func(user *User) string {
		if !user.LoggedIn {
			return ""
		}
		jwt, err := NewJWT(
			s.mediaserverKey,
			"search",
			"HS256",
			int64(s.linkTokenExp.Seconds()),
			"mediaserver",
			"mediathek",
			user.Id)
		if err != nil {
			return fmt.Sprintf("&error=%s", url.QueryEscape(fmt.Sprintf("ERROR-%v", err)))
		}
		return fmt.Sprintf("&token=%s", jwt)
	}

	s.funcMap["medialink"] = func(uri, action, param string, token bool) string {
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

	for name, templateFiles := range s.templatesFiles {
		tpl, err := initTemplate(templateFiles, name, s.funcMap)
		if err != nil {
			return emperror.Wrapf(err, "cannot initialize template")
		}
		s.templates[name] = tpl
	}
	return nil
}

var regexpMediaUri = regexp.MustCompile(`^mediaserver:(.+)$`)

func (s *Server) mediaserverUri2Url(uri string, params ...string) (string, error) {
	matches := regexpMediaUri.FindStringSubmatch(uri)
	if matches == nil {
		return "", fmt.Errorf("cannot parse uri %s", uri)
	}
	u := fmt.Sprintf("%s/%s", s.mediaserver, matches[1])
	if len(params) > 0 {
		u += "/" + strings.Join(params, "/")
	}
	return u, nil
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

func (s *Server) DoPanic(writer http.ResponseWriter, status int, message string) error {
	type errData struct {
		Status     int
		StatusText string
		Message    string
	}

	//debug.PrintStack()

	s.log.Error(message)
	data := errData{
		Status:     status,
		StatusText: http.StatusText(status),
		Message:    message,
	}
	writer.WriteHeader(status)
	// if there's no error Template, there's no help...
	if tpl, ok := s.templates["error.gohtml"]; ok {
		tpl.Execute(writer, data)
	} else {
		return fmt.Errorf("no error template found")
	}
	return nil
}

func (s *Server) DoPanicJSON(writer http.ResponseWriter, status int, message string) (err error) {
	type errData struct {
		Status     int    `json:"status"`
		StatusText string `json:"statustext"`
		Message    string `json:"message"`
	}

	debug.PrintStack()

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
	claims, err := CheckJWTValid(tokenstring, s.jwtKey, s.jwtAlg)
	if err != nil {
		return nil, emperror.Wrapf(err, "invalid access token")
	}

	// check whether token is from login gsearch
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

const (
	acceptEncoding string = "Accept-Encoding"
	gzipEncoding          = "gzip"
	flateEncoding         = "deflate"
)

func (s *Server) ListenAndServe(cert, key string) error {
	router := mux.NewRouter()

	// https://data.mediathek.hgk.fhnw.ch/search
	searchRegexp := regexp.MustCompile(fmt.Sprintf("/%s(/(.+))?$", s.prefixes["search"]))
	searchMatcher := func(r *http.Request, rm *mux.RouteMatch) bool {
		matches := searchRegexp.FindSubmatch([]byte(r.URL.Path))
		if len(matches) == 0 {
			return false
		}
		rm.Vars = map[string]string{}
		if len(matches) >= 3 {
			if matches[2] != nil {
				filter := string(matches[2])
				rm.Vars = map[string]string{}
				rm.Vars["subfilter"] = filter
			}
		}
		return true
	}
	router.
		MatcherFunc(searchMatcher).
		Handler(handlers.CompressHandler(func() http.Handler { return http.HandlerFunc(s.searchHandler) }())).
		Methods("GET")

	collectionsRegexp := regexp.MustCompile(fmt.Sprintf("/%s(/(.+))?$", s.prefixes["collections"]))
	collectionsMatcher := func(r *http.Request, rm *mux.RouteMatch) bool {
		matches := collectionsRegexp.FindSubmatch([]byte(r.URL.Path))
		if len(matches) == 0 {
			return false
		}
		rm.Vars = map[string]string{}
		if len(matches) >= 3 {
			if matches[2] != nil {
				filter := string(matches[2])
				rm.Vars = map[string]string{}
				rm.Vars["subfilter"] = filter
			}
		}
		return true
	}
	router.
		MatcherFunc(collectionsMatcher).
		Handler(handlers.CompressHandler(func() http.Handler { return http.HandlerFunc(s.collectionsHandler) }())).
		Methods("GET")

	// https://data.mediathek.hgk.fhnw.ch/detail/[signature]
	mainRegexp := regexp.MustCompile(fmt.Sprintf("^/%s/([^/]+)(/(.+))?$", s.prefixes["detail"]))
	router.
		MatcherFunc(func(r *http.Request, rm *mux.RouteMatch) bool {
			matches := mainRegexp.FindSubmatch([]byte(r.URL.Path))
			if len(matches) == 0 {
				return false
			}
			rm.Vars = map[string]string{}
			rm.Vars["signature"] = string(matches[1])
			if matches[3] != nil {
				rm.Vars["sub"] = string(matches[3])
			}
			return true
		}).
		Handler(handlers.CompressHandler(func() http.Handler { return http.HandlerFunc(s.detailHandler) }())).
		Methods("GET")

	// https://data.mediathek.hgk.fhnw.ch/update/[signature]
	updateRegexp := regexp.MustCompile(fmt.Sprintf("^/%s/(.+)$", s.prefixes["update"]))
	router.
		MatcherFunc(func(r *http.Request, rm *mux.RouteMatch) bool {
			matches := updateRegexp.FindSubmatch([]byte(r.URL.Path))
			if len(matches) == 0 {
				return false
			}
			rm.Vars = map[string]string{}
			rm.Vars["signature"] = string(matches[1])
			return true
		}).
		Handler(handlers.CompressHandler(func() http.Handler { return http.HandlerFunc(s.updateHandler) }())).
		Methods("GET")

	// the static fileserver
	router.
		PathPrefix(fmt.Sprintf("/%s", s.prefixes["static"])).
		Handler(http.StripPrefix("/"+s.prefixes["static"], func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Cache-Control", s.staticCacheControl)
				h.ServeHTTP(w, r)
			})
		}(http.FileServer(http.Dir(s.staticDir))))).Methods("GET")

	// google search
	router.HandleFunc(fmt.Sprintf("/%s/{csekey}", s.prefixes["cluster"]), s.clusterHandler).Methods("GET")
	router.HandleFunc(fmt.Sprintf("/%s/{csekey}", s.prefixes["cse"]), s.googleHandler).Methods("GET")

	router.HandleFunc(fmt.Sprintf("/%s/reloadtemplates", s.prefixes["api"]), s.reloadTemplateHandler).Methods("GET")
	router.HandleFunc(fmt.Sprintf("/%s/sitemap", s.prefixes["api"]), s.sitemapHandler).Methods("GET")
	router.HandleFunc(fmt.Sprintf("/%s/sitemap/{start:[0-9]+}", s.prefixes["api"]), s.sitemapHandler).Methods("GET")
	router.HandleFunc("/google54f060b89e33248e.html", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-type", "text/html")
		writer.Write([]byte("google-site-verification: google54f060b89e33248e.html\n"))
	})

	loggedRouter := handlers.LoggingHandler(s.accesslog, router)
	addr := net.JoinHostPort(s.host, s.port)
	s.srv = &http.Server{
		Handler: loggedRouter,
		Addr:    addr,
	}
	if cert == "auto" || key == "auto" {
		s.log.Info("generating new certificate")
		cert, err := DefaultCertificate()
		if err != nil {
			return emperror.Wrap(err, "cannot generate default certificate")
		}
		s.srv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{*cert}}
		s.log.Infof("starting HTTPS zsearch at https://%v/%v", addr, s.prefixes["search"])
		return s.srv.ListenAndServeTLS("", "")
	} else if cert != "" && key != "" {
		s.log.Infof("starting HTTPS zsearch at https://%v", addr)
		return s.srv.ListenAndServeTLS(cert, key)
	} else {
		s.log.Infof("starting HTTP zsearch at http://%v", addr)
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
	groupstr, err := GetClaim(claims, "Groups")
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

//var rexp = regexp.MustCompile(`([a-zA-Z0-9]+:([^ "]+|"[^"]+"))|([^ "]+)|"([^"]+)"`)
var rexp = regexp.MustCompile(`([a-zA-Z0-9]+:([^ "]+|"[^"]+"))`)

func (s *Server) __DELETE__string2Query(search string) string {
	var qstr string

	slice := rexp.FindAllString(search, -1)
	if slice == nil {
		slice = []string{}
	}

	// expand to field an generic search
	rexp2 := regexp.MustCompile(`^(` + strings.Join(maps.GetKeysStringString(s.searchFields), `|`) + `):(.+)$`)
	//Fields := make(map[string][]string)
	gen := []string{}
	qstr2 := ""

	for _, f := range slice {
		fldq := rexp2.FindStringSubmatch(f)
		if fldq != nil {
			fldquery, ok := s.searchFields[fldq[1]]
			if !ok {
				continue
			}
			val := EscapeSolrString(fldq[2])
			fldquery = strings.ReplaceAll(fldquery, "__Q__", val)
			if qstr2 != "" {
				qstr2 += " AND "
			}
			qstr2 += fmt.Sprintf("(%s)", fldquery)
		} else {
			gen = append(gen, f)
		}
	}

	/*
	      (
	   		(title:"fell"^10 OR title:"down"^10 OR title:"the"^10 OR title:"mountains"^10 OR title:"fell down the mountains"^20 )
	   	OR (author:fell^10 OR author:down^10 OR author:the^10 OR author:mountains^10 OR author:fell down the mountains^20 )
	   	OR (publisher:fell^8 OR publisher:down^8 OR publisher:the^8 OR publisher:mountains^8 OR publisher:fell down the mountains^18 )
	   	OR (content:fell*^6 AND content:down*^6 AND content:the*^6 AND content:mountains*^6 OR content:fell down the mountains*^12 )
	   	OR (abstract:fell*^8 AND abstract:down*^8 AND abstract:the*^8 AND abstract:mountains*^8 OR abstract:fell down the mountains*^15 )
	   	OR ( signature:"fell down the mountains"^25 )
	       )
	*/
	if len(gen) > 0 {
		qstr = fmt.Sprintf("%s OR %s OR %s OR %s OR %s OR %s OR %s",
			solrOr("title", gen, 10, 10),
			solrOr("author", gen, 10, 10),
			solrOr("publisher", gen, 8, 10),
			solrOr("content", gen, 0, 6),
			solrOr("abstract", gen, 0, 8),
			solrOr("signature", gen, 20, 10),
			solrOr("cluster", gen, 5, 2),
		)
	}
	if qstr2 != "" {
		if qstr != "" {
			qstr = fmt.Sprintf("(%s) AND (%s)", qstr, qstr2)
		} else {
			qstr = qstr2
		}
	}
	return qstr
}
func (s *Server) string2QList(search string, filterOrg map[string][]string) (map[string][]string, map[string][]string, string) {
	slice := rexp.FindAllString(search, -1)
	if slice == nil {
		slice = []string{}
	}

	// expand to field an generic search
	rexp2 := regexp.MustCompile(`^(` + strings.Join(maps.GetKeysStringString(s.searchFields), `|`) + `):(.+)$`)
	//Fields := make(map[string][]string)
	gen := []string{}
	fldlist := make(map[string][]string)
	fldlistOrg := filterOrg
	if fldlistOrg == nil {
		fldlistOrg = make(map[string][]string)
	}

	for fld, val := range fldlistOrg {
		fld, ok := s.searchFields[fld]
		if !ok {
			continue
		}
		fldlist[fld] = val
	}

	for _, f := range slice {
		fldq := rexp2.FindStringSubmatch(f)
		if fldq != nil {
			fld, ok := s.searchFields[fldq[1]]
			if !ok {
				continue
			}
			val := fldq[2]
			if _, ok := fldlist[fld]; !ok {
				fldlist[fld] = []string{}
				fldlistOrg[fldq[1]] = []string{}
			}
			fldlist[fld] = append(fldlist[fld], strings.Trim(val, `""''`))
			fldlistOrg[fldq[1]] = append(fldlistOrg[fldq[1]], strings.Trim(val, `""''`))
		} else {
			gen = append(gen, f)
		}
	}
	return fldlistOrg, fldlist, strings.TrimSpace(rexp.ReplaceAllString(search, " "))
}

func (s *Server) doc2result(
	search string,
	query string,
	docs []*SourceData,
	total int64,
	facetFieldCount FacetCountResult,
	facets map[string]termFacet,
	start int64,
	user *User,
	next string,
	highlight []map[string][]string) (*SearchResult, error) {
	result := &SearchResult{
		Items:           []SearchResultItem{},
		Total:           total,
		Start:           start,
		Rows:            int64(len(docs)),
		Query:           query,
		Search:          search,
		Next:            next,
		FacetFieldCount: make(map[string]facetField),
	}

	for facet, vals := range facetFieldCount {
		for val, count := range vals {
			id := fmt.Sprintf("facet_%s_%s", facet, val)
			result.FacetFieldCount[id] = facetField{
				Id:   id,
				Name: fmt.Sprintf("%s (%d)", val, count),
				Selected: func() bool {
					res, ok := facets[facet]
					if !ok {
						return false
					}
					if res.selected[val] {
						return true
					}
					return false
				}(),
			}
		}
	}

	for key, doc := range docs {
		if doc == nil {
			return nil, fmt.Errorf("empty document %v", key)
		}
		link := user.LinkSignature(doc.Signature)
		if !strings.HasPrefix(strings.ToLower(link), "http") {
			link = "detail/" + link
		}
		icon, ok := s.icons[strings.ToLower(doc.Type)]
		if !ok {
			icon = "#ion-open-outline"
		}
		item := SearchResultItem{
			Id:         doc.Signature,
			Type:       doc.Type,
			Title:      doc.Title,
			Text:       "",
			Collection: doc.CollectionTitle,
			Authors:    []string{},
			Link:       link,
			Date:       doc.Date,
			Icon:       icon,
			Media:      map[string]int{},
			Poster:     doc.Poster,
		}
		if key == 0 {
			item.FirstItem = true
			item.Total = total
		}
		for _, p := range doc.Persons {
			name := p.Name
			if p.Role != "author" && p.Role != "director" && p.Role != "artist" {
				name += fmt.Sprintf(" (%s)", p.Role)
			}
			item.Authors = append(item.Authors, name)
		}
		if len(item.Authors) > 0 {
			item.AuthorText = item.Authors[0]
		}
		if len(item.Authors) > 1 {
			item.AuthorText += " et al."
		}
		for mtype, medialist := range doc.Media {
			if mtype == "default" {
				continue
			}
			count := len(medialist)
			if count == 0 {
				break
			}
			item.Media[mtype] = count
		}
		for acl, groups := range doc.ACL {
			for _, group := range groups {
				for _, ugroup := range user.Groups {
					if group == ugroup {
						switch acl {
						case "meta":
							item.MetaOK = true
						case "content":
							item.ContentOK = true
						}
					}
				}
				if group == s.guestGroup {
					switch acl {
					case "meta":
						item.MetaPublic = true
					case "content":
						item.ContentPublic = true
					}
				}
			}
		}
		if user.inGroup(s.adminGroup) {
			item.MetaOK = true
			item.ContentOK = true
		}

		if hl := highlight[key]; ok {
			item.Highlight = hl
		}

		result.Items = append(result.Items, item)
	}
	return result, nil
}

func (s *Server) doc2json(search string,
	query string,
	docs []*SourceData,
	total int64,
	facetFieldCount FacetCountResult,
	facets map[string]termFacet,
	start int64,
	user *User,
	next string) ([]byte, error) {
	result, err := s.doc2result(search, query, docs, total, facetFieldCount, facets, start, user, next, nil)
	if err != nil {
		return nil, emperror.Wrap(err, "cannot format result")
	}
	r, err := json.Marshal(result)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot marshal result")
	}
	return r, nil
}
