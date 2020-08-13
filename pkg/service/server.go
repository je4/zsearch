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
	"github.com/drgrib/maps"
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
	Token         string
	MetaPublic    bool
	ContentPublic bool
	MetaOK        bool
	ContentOK     bool
	IsAmp         bool
	Self          string
	BaseUrl       string
	SelfPath      string
	AmpBase       string
	LoginUrl      string
	Title         string
	Menu          []Menu
}

type FacetCountField struct {
	Id        string
	Name      string
	ShortName string
	Selected  bool
}

type SearchStatus struct {
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
	FacetCount        map[string]FacetCountField
	Menu              []Menu
}

type SubFilter struct {
	Name   string `toml:"name"`
	Label  string `toml:"label"`
	Filter string `toml:"filter"`
}

type NetGroups map[string][]*net.IPNet

type Menu struct {
	Label string
	Url   string
	Sub   map[string]string
}

type facetField struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	Selected bool   `json:"selected"`
}

type SearchResult struct {
	Items           []SearchResultItem    `json:"items"`
	Total           int64                 `json:"total"`
	Start           int64                 `json:"start"`
	Rows            int64                 `json:"rows"`
	Query           string                `json:"query"`
	Search          string                `json:"search"`
	Next            string                `json:"next"`
	FacetFieldCount map[string]facetField `json:"facetfieldcount"`
}

type SearchResultItem struct {
	Id            string         `json:"Id"`
	Type          string         `json:"type"`
	Title         string         `json:"title"`
	Text          string         `json:"text"`
	Collection    string         `json:"collection"`
	Authors       []string       `json:"authors"`
	AuthorText    string         `json:"authortext"`
	Link          string         `json:"link"`
	FirstItem     bool           `json:"firstitem"`
	Total         int64          `json:"total,omitempty"`
	Date          string         `json:"date"`
	Icon          string         `json:"icon"`
	Media         map[string]int `json:"media"`
	MetaPublic    bool           `json:"metapublic"`
	ContentPublic bool           `json:"contentpublic"`
	MetaOK        bool           `json:"metaok"`
	ContentOK     bool           `json:"contentok"`
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
	searchFields      map[string]string
	facets            source.SolrFacetList
	locations         NetGroups
	menu              []Menu
	icons             map[string]string
	baseFilter        string
	subFilters        []SubFilter
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
	searchFields map[string]string,
	facets source.SolrFacetList,
	locations NetGroups,
	menu []Menu,
	icons map[string]string,
	baseFilter string,
	subFilter []SubFilter,
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
		searchFields:   searchFields,
		facets:         facets,
		locations:      locations,
		menu:           menu,
		icons:          icons,
		baseFilter:     baseFilter,
		subFilters:     subFilter,
	}
	if err := srv.InitTemplates(detailTemplate, errorTemplate, forbiddenTemplate, searchTemplate); err != nil {
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
			result += fmt.Sprintf(`%s:%s^%d`, field, source.EscapeSolrString(val), weight1)
		}
		if weight2 > 0 && !withQuotes {
			if result != "" {
				result += " OR "
			}
			result += fmt.Sprintf(`%s:%s*^%d`, field, source.EscapeSolrString(val), weight2)
		}
	}
	return result
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
		Status     int    `json:"status"`
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
	searchRegexp := regexp.MustCompile(fmt.Sprintf("/%s(/(.+))?$", s.searchPrefix))
	router.
		MatcherFunc(func(r *http.Request, rm *mux.RouteMatch) bool {
			matches := searchRegexp.FindSubmatch([]byte(r.URL.Path))
			if len(matches) == 0 {
				return false
			}
			rm.Vars = map[string]string{}
			if len(matches) >= 3 {
				if matches[2] != nil {
					filter := string(matches[2])
					// check for valid filtername
					filterok := false
					for _, sf := range s.subFilters {
						if sf.Label == filter {
							filterok = true
							break
						}
					}
					// otherwise 404
					if !filterok {
						if filter != "data" {
							return false
						}
					}
					rm.Vars = map[string]string{}
					rm.Vars["subfilter"] = filter
				}
			}
			return true
		}).HandlerFunc(s.searchHandler).Methods("GET")
	//router.HandleFunc(fmt.Sprintf("/%s", s.searchPrefix), s.searchHandler).Methods("GET")

	// https://data.mediathek.hgk.fhnw.ch/detail/[signature]
	mainRegexp := regexp.MustCompile(fmt.Sprintf("^/%s/([^/]+)(/(.+))?$", s.detailPrefix))
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

func (s *Server) string2Query(search string) string {
	var qstr string

	rexp := regexp.MustCompile(`([a-zA-Z0-9]+:([^ "]+|"[^"]+"))|([^ "]+)|"([^"]+)"`)
	slice := rexp.FindAllString(search, -1)
	if slice == nil {
		slice = []string{}
	}

	// expand to field an generic search
	rexp2 := regexp.MustCompile(`^(` + strings.Join(maps.GetKeysStringString(s.searchFields), `|`) + `):(.+)$`)
	fields := make(map[string][]string)
	gen := []string{}

	for _, f := range slice {
		fldq := rexp2.FindStringSubmatch(f)
		if fldq != nil {
			fldname, ok := s.searchFields[fldq[1]]
			if !ok {
				continue
			}
			if _, ok := fields[fldname]; !ok {
				fields[fldname] = []string{}
			}
			fields[fldname] = append(fields[fldname], fldq[2])
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
	qstr2 := ""
	if len(fields) > 0 {
		for field, val := range fields {
			if qstr2 != "" {
				qstr2 += " OR "
			}
			qstr2 += solrOr(field, val, 30, 15)
		}
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

func (s *Server) doc2result(search string,
	query string,
	docs []*source.Document,
	total int64,
	facetFieldCount source.FacetCountResult,
	facets map[string]map[string]bool,
	start int64,
	user *User,
	next string) (*SearchResult, error) {
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
					if res[val] {
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
		link := user.LinkSignature(doc.Id)
		if !strings.HasPrefix(strings.ToLower(link), "http") {
			link = "detail/" + link
		}
		if doc.Content == nil {
			return nil, fmt.Errorf("no content in document %v", doc)
		}
		icon, ok := s.icons[strings.ToLower(doc.Content.Type)]
		if !ok {
			icon = "#ion-open-outline"
		}
		item := SearchResultItem{
			Id:         doc.Id,
			Type:       doc.Content.Type,
			Title:      doc.Content.Title,
			Text:       "",
			Collection: doc.Content.CollectionTitle,
			Authors:    []string{},
			Link:       link,
			Date:       doc.Content.Date,
			Icon:       icon,
			Media:      map[string]int{},
		}
		if key == 0 {
			item.FirstItem = true
			item.Total = total
		}
		for _, p := range doc.Content.Persons {
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
		for mtype, medialist := range doc.Content.Media {
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

		result.Items = append(result.Items, item)
	}
	return result, nil
}

func (s *Server) doc2json(search string,
	query string,
	docs []*source.Document,
	total int64,
	facetFieldCount source.FacetCountResult,
	facets map[string]map[string]bool,
	start int64,
	user *User,
	next string) ([]byte, error) {
	result, err := s.doc2result(search, query, docs, total, facetFieldCount, facets, start, user, next)
	if err != nil {
		return nil, emperror.Wrap(err, "cannot format result")
	}
	r, err := json.Marshal(result)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot marshal result")
	}
	return r, nil
}
