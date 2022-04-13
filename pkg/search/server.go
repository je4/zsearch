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
	"crypto/sha512"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/Masterminds/sprig"
	"github.com/bluele/gcache"
	"github.com/drgrib/maps"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/htfy96/reformism"
	"github.com/je4/utils/v2/pkg/JWTInterceptor"
	"github.com/je4/zsearch/v2/pkg/amp"
	"github.com/je4/zsearch/v2/web"
	"github.com/op/go-logging"
	"github.com/pkg/errors"
	"google.golang.org/api/customsearch/v1"
	"html/template"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
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
	OGPNamespace  string
	OGPMeta       string
	JsonLD        string
	Type          string
	Notifications []Notification
	User          *User
	Self          string
	RawQuery      string
	Canonical     string
	Token         string
	BaseUrl       string
	Prefixes      map[string]string
	SelfPath      string
	RelPath       string
	AmpBase       string
	LoginUrl      string
	Title         string
	InstanceName  string
	server        *Server
	Plain         bool
}

type DetailStatus struct {
	BaseStatus
	Doc               *SourceData
	MetaPublic        bool
	ContentPublic     bool
	MetaOK            bool
	ContentOK         bool
	IsAmp             bool
	MetaDescription   string
	Result            *SearchResult
	SearchResultRows  int
	SearchResultTotal int
	FacebookAppId     string
	Plain             bool
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
	EmptySearch         bool
	Stats               FacetCountResult
}

type CollectionsStatus struct {
	BaseStatus

	SearchResult        template.JS
	Result              map[string][]*SourceData
	Areas               []string
	QueryApi            template.URL
	SearchResultStart   int
	SearchResultRows    int
	SearchResultTotal   int
	SearchResultVisible bool
	MetaDescription     string
}

type ClusterStatus struct {
	BaseStatus

	SearchResult        template.JS
	Result              []*SourceData
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
	Name   string `toml:"Name"`
	Label  string `toml:"label"`
	Filter string `toml:"filter"`
}

type NetGroups map[string][]*net.IPNet

type facetField struct {
	Id       string `json:"id"`
	Name     string `json:"Name"`
	Selected bool   `json:"Selected"`
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

type KV struct{ Key, Name string }

func (bs BaseStatus) LinkSignatureCache(signature string) string {
	urlstr := fmt.Sprintf("%s/%s/%s", bs.RelPath, bs.server.prefixes["detail"], signature)
	urlstr = strings.TrimLeft(urlstr, "/")
	var err error
	if bs.server.ampCache != nil {
		urlstr, err = bs.server.ampCache.BuildUrl(urlstr, amp.PAGE)
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
	}
	return urlstr
}
func (bs BaseStatus) LinkSearch(query string, facets ...string) template.URL {
	urlstr := fmt.Sprintf("%s/%s?searchtext=%s", bs.RelPath, bs.server.prefixes["search"], url.QueryEscape(query))
	urlstr = strings.TrimLeft(urlstr, "/")
	for _, f := range facets {
		urlstr += fmt.Sprintf("&%s=true", url.QueryEscape(f))
	}
	if bs.User.LoggedIn {
		_, err := NewJWT(
			bs.server.jwtKey,
			"search",
			"HS256",
			int64(bs.server.linkTokenExp.Seconds()),
			"catalogue",
			"mediathek",
			bs.User.Id)
		if err != nil {
			return template.URL(fmt.Sprintf("ERROR: %v", err))
		}
		//urlstr += fmt.Sprintf("&token=%s", jwt)
	} else {
		if bs.server.ampCache != nil {
			var err error
			urlstr, err = bs.server.ampCache.BuildUrl(urlstr, amp.PAGE)
			if err != nil {
				return template.URL(fmt.Sprintf("ERROR: %v", err))
			}
		}
	}
	return template.URL(urlstr)

}
func (bs BaseStatus) LinkSignature(signature string) string {
	urlstr := fmt.Sprintf("%s/%s/%s", bs.RelPath, bs.server.prefixes["detail"], signature)
	urlstr = strings.TrimLeft(urlstr, "/")
	if bs.User.LoggedIn {
		_, err := NewJWT(
			bs.server.jwtKey,
			fmt.Sprintf("detail:%s", signature),
			"HS256",
			int64(bs.server.linkTokenExp.Seconds()),
			"catalogue",
			"mediathek",
			bs.User.Id)
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		//urlstr = fmt.Sprintf("%s?token=%s", urlstr, jwt)
	} else {
		if bs.server.ampCache != nil {
			var err error
			urlstr, err = bs.server.ampCache.BuildUrl(urlstr, amp.PAGE)
			if err != nil {
				return fmt.Sprintf("ERROR: %v", err)
			}
		}
	}
	return urlstr
}
func (bs BaseStatus) LinkCollections() string {
	urlstr := fmt.Sprintf("%s/%s", bs.RelPath, bs.server.prefixes["collections"])
	urlstr = strings.TrimLeft(urlstr, "/")
	if bs.User.LoggedIn {
		_, err := NewJWT(
			bs.User.Server.jwtKey,
			"collections",
			"HS256",
			int64(bs.server.linkTokenExp.Seconds()),
			"catalogue",
			"mediathek",
			bs.User.Id)
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		//urlstr = fmt.Sprintf("%s?token=%s", urlstr, jwt)
	} else {
		if bs.server.ampCache != nil {
			var err error
			urlstr, err = bs.server.ampCache.BuildUrl(urlstr, amp.PAGE)
			if err != nil {
				return fmt.Sprintf("ERROR: %v", err)
			}
		}
	}
	return urlstr
}
func (bs BaseStatus) LinkSubject(area, sub, subject string, params ...string) string {
	prefix, ok := bs.server.prefixes[area]
	if !ok {
		bs.server.log.Errorf("invalid area %s in link", area)
		return fmt.Sprintf("#invalid area %s in link", area)
	}
	var urlstr string
	if sub != "" {
		urlstr = fmt.Sprintf("%s/%s/%s", bs.RelPath, prefix, sub)
	} else {
		urlstr = fmt.Sprintf("%s/%s", bs.RelPath, prefix)
	}
	urlstr = strings.TrimLeft(urlstr, "/")
	if bs.User.LoggedIn {
		_, err := NewJWT(
			bs.server.jwtKey,
			subject,
			"HS256",
			int64(bs.server.linkTokenExp.Seconds()),
			"catalogue",
			"mediathek",
			bs.User.Id)
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		//urlstr = fmt.Sprintf("%s?token=%s", urlstr, jwt)
		if len(params) > 0 {
			urlstr += "?" + strings.Join(params, "&")
		}
	} else {
		if bs.server.ampCache != nil {
			var err error
			urlstr, err = bs.server.ampCache.BuildUrl(urlstr, amp.PAGE)
			if err != nil {
				return fmt.Sprintf("ERROR: %v", err)
			}
		}
		if len(params) > 0 {
			urlstr += "?" + strings.Join(params, "&")
		}
	}
	return urlstr
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
	service             string
	mts                 *Search
	srv                 *http.Server
	userCache           *UserCache
	host                string
	port                string
	addrExt             *url.URL
	prefixes            map[string]string
	staticDir           string
	sitemapDir          string
	staticCacheControl  string
	jwtKey              string
	jwtAlg              []string
	linkTokenExp        time.Duration
	loginUrl            string
	loginIssuer         string
	guestGroup          string
	adminGroup          string
	templates           map[string]*template.Template
	templatesFiles      map[string][]string
	templateDev         bool
	mediaserver         string
	mediaserverKey      string
	mediaTokenExp       time.Duration
	log                 *logging.Logger
	accesslog           io.Writer
	ampApiKey           *rsa.PrivateKey
	ampCache            *amp.Cache
	searchFields        map[string]string
	facets              SolrFacetList
	locations           NetGroups
	icons               map[string]string
	baseCatalog         []string
	subFilters          []SubFilter
	funcMap             template.FuncMap
	collectionsCatalog  string
	clusterCatalog      string
	queryCache          gcache.Cache
	google              *customsearch.Service
	googleCSEKey        map[string]KV
	instanceName        string
	cookieStore         *sessions.CookieStore
	cookieAuthKey       []byte
	cookieEncryptionKey []byte
	sessionTimeout      time.Duration
	templateDir         string
	facebookAppId       string
}

func NewServer(
	service string,
	mts *Search,
	uc *UserCache,
	google *customsearch.Service,
	templateFiles map[string][]string,
	templateDev bool,
	InstanceName,
	addr, addrExt,
	mediaserver, mediaserverkey string,
	mediatokenexp time.Duration,
	log *logging.Logger,
	accesslog io.Writer,
	prefixes map[string]string,
	staticDir,
	sitemapdir,
	staticCacheControl,
	templatedir,
	jwtKey string, jwtAlg []string,
	linkTokenExp,
	sessionTimeout time.Duration,
	loginUrl,
	loginIssuer,
	guestGroup,
	adminGroup,
	AmpCache,
	ampApiKeyFile string,
	searchFields map[string]string,
	facets SolrFacetList,
	locations NetGroups,
	icons map[string]string,
	baseCatalog []string,
	subFilter []SubFilter,
	collectionsCatalog,
	clusterCatalog string,
	googleCSEKey map[string]KV,
	facebookAppId string,
) (*Server, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		//log.Panicf("cannot split address %s: %v", addr, err)
		return nil, errors.Wrapf(err, "cannot split address %s", addr)
	}

	// load private api Key
	privateKeyFile, err := os.Open(ampApiKeyFile)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot open %s", ampApiKeyFile)
	}
	pemfileinfo, _ := privateKeyFile.Stat()
	pembytes := make([]byte, pemfileinfo.Size())
	buffer := bufio.NewReader(privateKeyFile)
	_, err = buffer.Read(pembytes)
	data, _ := pem.Decode([]byte(pembytes))
	if err := privateKeyFile.Close(); err != nil {
		log.Errorf("error closing private key file: %v", err)
	}
	ampApiKey, err := x509.ParsePKCS1PrivateKey(data.Bytes)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot parse private Key %s", string(data.Bytes))
	}

	aCaches, err := amp.GetCaches()
	if err != nil {
		return nil, err
	}
	ampCache, _ := aCaches[AmpCache]

	extUrl, err := url.Parse(addrExt)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot parse external address %s", addrExt)
	}
	authKey := securecookie.GenerateRandomKey(64)
	encryptionKey := securecookie.GenerateRandomKey(32)
	srv := &Server{
		service:            service,
		mts:                mts,
		userCache:          uc,
		google:             google,
		host:               host,
		port:               port,
		addrExt:            extUrl,
		prefixes:           prefixes,
		mediaserver:        mediaserver,
		mediaserverKey:     mediaserverkey,
		mediaTokenExp:      mediatokenexp,
		log:                log,
		accesslog:          accesslog,
		staticDir:          staticDir,
		sitemapDir:         sitemapdir,
		templateDir:        templatedir,
		staticCacheControl: staticCacheControl,
		templateDev:        templateDev,
		jwtKey:             jwtKey,
		jwtAlg:             jwtAlg,
		linkTokenExp:       linkTokenExp,
		sessionTimeout:     sessionTimeout,
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
		baseCatalog:        baseCatalog,
		subFilters:         subFilter,
		templates:          make(map[string]*template.Template),
		funcMap:            template.FuncMap{},
		collectionsCatalog: collectionsCatalog,
		clusterCatalog:     clusterCatalog,
		queryCache:         gcache.New(100).ARC().Expiration(time.Hour * 3).Build(),
		googleCSEKey:       googleCSEKey,
		facebookAppId:      facebookAppId,
		templatesFiles:     templateFiles,
		instanceName:       InstanceName,
		cookieStore: sessions.NewCookieStore(
			authKey,
			nil,
			//encryptionKey,
		),
		cookieAuthKey:       authKey,
		cookieEncryptionKey: encryptionKey,
	}
	srv.cookieStore.Options = &sessions.Options{
		MaxAge:   int(sessionTimeout / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode, // http.SameSiteStrictMode,
		Path:     "/",
	}
	if err := srv.InitTemplates(); err != nil {
		return nil, errors.Wrapf(err, "cannot initialize server")
	}
	return srv, nil
}

func initTemplate(filesystem fs.FS, tpl []string, name string, funcMap template.FuncMap) (*template.Template, error) {
	newTpl, err := template.New(name).Funcs(funcMap).ParseFS(filesystem, tpl...)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot parse error template %v - %v", name, tpl)
	}
	return newTpl, nil
}

func (s *Server) InitTemplates() (err error) {
	mediaMatch := regexp.MustCompile(`^mediaserver:([^/]+)/([^/]+)$`)

	for key, val := range sprig.FuncMap() {
		s.funcMap[key] = val
	}
	s.funcMap["trimLeft"] = func(cutset, value string) string {
		return strings.TrimLeft(value, cutset)
	}
	s.funcMap["trimRight"] = func(cutset, value string) string {
		return strings.TrimRight(value, cutset)
	}
	s.funcMap["trimLength"] = func(value string, length int, suffix string) string {
		return TrimLength(value, length, suffix)
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
		_, err := NewJWT(
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
		return "" // fmt.Sprintf("&token=%s", jwt)
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
		urlstr := fmt.Sprintf("%s/%s/%s/%s/%s", s.mediaserver, collection, signature, action, param)
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
			urlstr = fmt.Sprintf("%s?token=%s", urlstr, jwt)
		} else {
			if s.ampCache != nil {
				urlstr, err = s.ampCache.BuildUrl(urlstr, amp.IMAGE)
				if err != nil {
					return fmt.Sprintf("ERROR: %v", err)
				}
			}
		}
		return urlstr
	}

	var filesystem fs.FS
	if s.templateDir == "" {
		filesystem, err = fs.Sub(web.TemplateFS, "template")
		if err != nil {
			return errors.Wrap(err, "cannot get subtree of embedded static")
		}
	} else {
		filesystem = os.DirFS(s.templateDir)
	}

	for name, templateFiles := range s.templatesFiles {
		tpl, err := initTemplate(filesystem, templateFiles, name, s.funcMap)
		if err != nil {
			return errors.Wrapf(err, "cannot initialize template")
		}
		s.templates[name] = tpl
	}
	return nil
}

var regexpMediaUri = regexp.MustCompile(`^mediaserver:([^/]+)/([^/]+)$`)

func (s *Server) mediaserverUri2Url(uri string, params ...string) (string, error) {
	matches := regexpMediaUri.FindStringSubmatch(uri)
	if matches == nil {
		return "", fmt.Errorf("cannot parse uri %s", uri)
	}
	u := fmt.Sprintf("%s/%s/%s", s.mediaserver, matches[1], matches[2])
	if len(params) > 0 {
		u += "/" + strings.Join(params, "/")
	}
	return u, nil
}

func mediaserverUri2ColSig(uri string) (string, string, error) {
	matches := regexpMediaUri.FindStringSubmatch(uri)
	if matches == nil {
		return "", "", fmt.Errorf("cannot parse uri %s", uri)
	}
	return matches[1], matches[2], nil
}

func (s *Server) DoPanicf(user *User, req *http.Request, writer http.ResponseWriter, status int, message string, json bool, a ...interface{}) (err error) {
	msg := fmt.Sprintf(message, a...)
	if json {
		if err := s.DoPanicJSON(writer, status, msg); err != nil {
			s.log.Errorf("error sending panic json: %v", err)
		}
	} else {
		if user == nil {
			user = NewGuestUser(s)
		}
		data := struct {
			BaseStatus
			Error   string
			Message string
		}{
			BaseStatus: BaseStatus{
				Type:          "error",
				User:          user,
				Self:          fmt.Sprintf("%s/%s", s.addrExt, strings.TrimLeft(req.URL.Path, "/")),
				BaseUrl:       s.addrExt.String(),
				RawQuery:      req.URL.RawQuery,
				SelfPath:      req.URL.Path,
				LoginUrl:      s.loginUrl,
				Notifications: []Notification{},
				Token:         "",
				Prefixes: map[string]string{
					"detail":      s.prefixes["detail"],
					"search":      s.prefixes["search"],
					"collections": s.prefixes["collections"],
					"cluster":     s.prefixes["cluster"],
					"google":      s.prefixes["cse"],
				},
				AmpBase:      "",
				Title:        "",
				InstanceName: s.instanceName,
			},
			Error:   fmt.Sprintf("%v - %s", status, http.StatusText(status)),
			Message: msg,
		}
		writer.WriteHeader(http.StatusNotFound)
		if tpl, ok := s.templates["error.amp.gohtml"]; ok {
			if err := tpl.Execute(writer, data); err != nil {
				s.log.Errorf("executing error.amp.gohtml template: %v", err)
			}
		}
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
		if err := tpl.Execute(writer, data); err != nil {
			s.log.Errorf("executing error.gohtml template: %v", err)
		}
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
	if err := jenc.Encode(data); err != nil {
		s.log.Errorf("error encoding json [%v]: %v", data, err)
	}
	return
}

func (s *Server) userFromToken(tokenstring, signature string) (*User, error) {

	// jwt valid?
	claims, err := CheckJWTValid(tokenstring, s.jwtKey, s.jwtAlg)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid access token")
	}

	// check whether token is from login gsearch
	issuer, ok := claims["iss"]
	if !ok {
		return nil, errors.Wrapf(err, "no iss in token %v", tokenstring)
	}
	issuerstr, ok := issuer.(string)
	if !ok {
		return nil, errors.Wrapf(err, "iss not a string in token %v", tokenstring)
	}

	// token from login
	var user *User
	if issuerstr == s.loginIssuer {
		user, err = s.GetClaimUser(claims)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot extract userdata from token %v", tokenstring)
		}
		user.LoggedIn = true
		user.LoggedOut = false

		if err := s.userCache.SetUser(user, user.Id); err != nil {
			s.log.Errorf("error adding user to cache: %v", err)
		}
	} else {
		// sub given?
		sub, err := GetClaim(claims, "sub")
		if err != nil {
			return nil, errors.Wrapf(err, "no sub in token")
		}
		// sub correct?
		if signature != "" && strings.ToLower(sub) != strings.ToLower(signature) {
			return nil, errors.Wrapf(err, "invalid sub %s (should be %s) in token", sub, signature)
		}
		// user given?

		userstr, err := GetClaim(claims, "user")
		if err != nil {
			return nil, errors.Wrapf(err, "no user in token")
		}
		// user is string?

		user, err = s.userCache.GetUser(userstr)
		// user not found --> log out and become a guest
		if err != nil {
			return nil, errors.Wrapf(err, "user timed out")
		} else {
			user.LoggedOut = false
			user.LoggedIn = true
		}
	}
	return user, nil
}

/*
const (
	acceptEncoding string = "Accept-Encoding"
	gzipEncoding          = "gzip"
	flateEncoding         = "deflate"
)
*/

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
	router.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, fmt.Sprintf("%s/%s", s.addrExt, s.prefixes["search"]), http.StatusMovedPermanently)
	}))
	router.
		MatcherFunc(searchMatcher).
		Handler(handlers.CompressHandler(func() http.Handler { return http.HandlerFunc(s.searchHandler) }())).
		Methods("GET")

	collectionsRegexp := regexp.MustCompile(fmt.Sprintf("/%s(/(?P<subfilter>.+))?$", s.prefixes["collections"]))
	router.
		MatcherFunc(buildMatcher(collectionsRegexp)).
		Handler(handlers.CompressHandler(func() http.Handler { return http.HandlerFunc(s.collectionsHandler) }())).
		Methods("GET")

	// https://data.mediathek.hgk.fhnw.ch/detail/[signature]/embed/[embedCollection|/[embedSignature]
	embedRegexp := regexp.MustCompile(fmt.Sprintf("/%s/(?P<signature>[^/]+)/embed/(?P<embedCollection>[^/]+)/(?P<embedSignature>[^/]+)(/(?P<rest>.*))?$", s.prefixes["detail"]))
	router.
		MatcherFunc(buildMatcher(embedRegexp)).
		Handler(handlers.CompressHandler(func() http.Handler { return http.HandlerFunc(s.detailEmbedHandler) }())).
		Methods("GET")
	// https://data.mediathek.hgk.fhnw.ch/detail/[signature]
	detailRegexp := regexp.MustCompile(fmt.Sprintf("/%s/(?P<signature>[^/]+)(/(?P<collection>[^/]+-[^/]+))?(/(?P<data>data))?(/(?P<plain>plain))?(/(?P<rest>.*))?$", s.prefixes["detail"]))
	router.
		MatcherFunc(buildMatcher(detailRegexp)).
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
	// trouble with mimetypes on windows
	if runtime.GOOS == "windows" {

		if err := mime.AddExtensionType(".js", "application/javascript; charset=utf-8"); err != nil {
			s.log.Errorf("cannot add mime extension type: %v", err)
		}

		if err := mime.AddExtensionType(".css", "text/css; charset=utf-8"); err != nil {
			s.log.Errorf("cannot add mime extension type: %v", err)
		}
	}

	var httpStaticServer http.Handler
	if s.staticDir == "" {
		fsys, err := fs.Sub(web.StaticFS, "static")
		if err != nil {
			return errors.Wrap(err, "cannot get subtree of embedded static")
		}
		httpStaticServer = http.FileServer(http.FS(fsys))
	} else {
		httpStaticServer = http.FileServer(http.Dir(s.staticDir))
	}
	router.
		PathPrefix(fmt.Sprintf("/%s", s.prefixes["static"])).
		Handler(http.StripPrefix("/"+s.prefixes["static"], func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Cache-Control", s.staticCacheControl)
				h.ServeHTTP(w, r)
			})
		}( /*http.FileServer(http.Dir(s.staticDir))*/ httpStaticServer))).Methods("GET")

	var httpSitemapServer http.Handler
	httpSitemapServer = http.FileServer(http.Dir(s.sitemapDir))
	router.
		PathPrefix(fmt.Sprintf("/%s", s.prefixes["sitemap"])).
		Handler(http.StripPrefix("/"+s.prefixes["sitemap"], func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				//w.Header().Set("Cache-Control", s.staticCacheControl)
				h.ServeHTTP(w, r)
			})
		}( /*http.FileServer(http.Dir(s.staticDir))*/ httpSitemapServer))).Methods("GET")

	// google search
	router.HandleFunc(fmt.Sprintf("/%s", s.prefixes["cluster"]), s.clusterAllHandler).Methods("GET")
	router.HandleFunc(fmt.Sprintf("/%s/{csekey}", s.prefixes["cluster"]), s.clusterHandler).Methods("GET")
	router.HandleFunc(fmt.Sprintf("/%s/{csekey}", s.prefixes["cse"]), s.googleHandler).Methods("GET")

	router.HandleFunc(fmt.Sprintf("/%s/reloadtemplates", s.prefixes["api"]), s.reloadTemplateHandler).Methods("GET")
	//	router.HandleFunc(fmt.Sprintf("/%s/sitemap", s.prefixes["api"]), s.sitemapHandler).Methods("GET")
	//	router.HandleFunc(fmt.Sprintf("/%s/sitemap/{start:[0-9]+}", s.prefixes["api"]), s.sitemapHandler).Methods("GET")
	router.HandleFunc("/google54f060b89e33248e.html", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-type", "text/html")

		if _, err := writer.Write([]byte("google-site-verification: google54f060b89e33248e.html\n")); err != nil {
			s.log.Errorf("cannot write response data: %v", err)
		}
	})
	router.Handle(
		fmt.Sprintf("/%s/signatures", s.prefixes["api"]), JWTInterceptor.JWTInterceptor(
			s.service,
			"SignatureCreate",
			JWTInterceptor.Secure,
			func() http.Handler { return http.HandlerFunc(s.apiHandlerSignatureCreate) }(),
			s.jwtKey,
			s.jwtAlg,
			sha512.New(),
			s.log,
		),
	).
		Methods("POST")
	router.Handle(
		fmt.Sprintf("/%s/clearcache", s.prefixes["api"]), JWTInterceptor.JWTInterceptor(
			s.service,
			"ClearCache",
			JWTInterceptor.Secure,
			func() http.Handler { return http.HandlerFunc(s.apiHandlerClearCache) }(),
			s.jwtKey,
			s.jwtAlg,
			sha512.New(),
			s.log,
		),
	).
		Methods("POST")
	router.Handle(
		fmt.Sprintf("/%s/signatures/{prefix}", s.prefixes["api"]), JWTInterceptor.JWTInterceptor(
			s.service,
			"SignaturesDelete",
			JWTInterceptor.Secure,
			func() http.Handler { return http.HandlerFunc(s.apiHandlerSignaturesDelete) }(),
			s.jwtKey,
			s.jwtAlg,
			sha512.New(),
			s.log,
		)).
		Methods("DELETE")
	router.Handle(
		fmt.Sprintf("/%s/buildsitemap", s.prefixes["api"]), JWTInterceptor.JWTInterceptor(
			s.service,
			"BuildSitemap",
			JWTInterceptor.Secure,
			func() http.Handler { return http.HandlerFunc(s.apiHandlerBuildSitemap) }(),
			s.jwtKey,
			s.jwtAlg,
			sha512.New(),
			s.log,
		)).
		Methods("POST")
	router.Handle(
		fmt.Sprintf("/%s/signatures/{prefix}/lastupdate", s.prefixes["api"]), JWTInterceptor.JWTInterceptor(
			s.service,
			"LastUpdate",
			JWTInterceptor.Secure,
			func() http.Handler { return http.HandlerFunc(s.apiHandlerLastUpdate) }(),
			s.jwtKey,
			s.jwtAlg,
			sha512.New(),
			s.log,
		)).
		Methods("GET")
	router.HandleFunc(fmt.Sprintf("/%s/ping", s.prefixes["api"]), s.apiHandlerPing).Methods("GET")

	loggedRouter := handlers.CombinedLoggingHandler(s.accesslog, handlers.ProxyHeaders(router))
	addr := net.JoinHostPort(s.host, s.port)
	s.srv = &http.Server{
		Handler: loggedRouter,
		Addr:    addr,
	}
	if cert == "auto" || key == "auto" {
		s.log.Info("generating new certificate")
		cert, err := DefaultCertificate()
		if err != nil {
			return errors.Wrap(err, "cannot generate default certificate")
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
		return nil, errors.Wrapf(err, "no userid in Key")
	}
	groupstr, err := GetClaim(claims, "groups")
	if err != nil {
		groupstr = "global/guest;global/user"
	}
	groups := strings.Split(groupstr, ";")
	firstName, _ := GetClaim(claims, "firstName")
	lastName, _ := GetClaim(claims, "lastName")
	homeOrg, _ := GetClaim(claims, "homeOrg")
	email, _ := GetClaim(claims, "email")
	expval, ok := claims["exp"]
	if !ok {
		return nil, errors.Wrapf(err, "no exp in Key")
	}
	exp, ok := expval.(float64)
	if !ok {
		return nil, errors.Wrapf(err, "exp not an ")
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
	facets map[string]TermFacet,
	start int64,
	bs *BaseStatus,
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
					if res.Selected[val] {
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
		link := bs.LinkSignature(doc.Signature)
		//		if !strings.HasPrefix(strings.ToLower(link), "http") {
		//			link = "detail/" + link
		//		}
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
				for _, ugroup := range bs.User.Groups {
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
		if bs.User.inGroup(s.adminGroup) {
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
	facets map[string]TermFacet,
	start int64,
	bs *BaseStatus,
	next string) ([]byte, error) {
	result, err := s.doc2result(search, query, docs, total, facetFieldCount, facets, start, bs, next, nil)
	if err != nil {
		return nil, errors.Wrap(err, "cannot format result")
	}
	r, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot marshal result")
	}
	return r, nil
}

func (s *Server) relPath(path string) string {
	parts := len(strings.Split(strings.TrimLeft(path, "/"), "/")) - 1
	relpath := ""
	for i := 0; i < parts; i++ {
		relpath += "../"
	}
	relpath = strings.TrimRight(relpath, "/")
	if relpath == "" {
		relpath = "."
	}
	//	s.log.Debugf("relativ path to %s - %s", path, relpath)
	return relpath
}
