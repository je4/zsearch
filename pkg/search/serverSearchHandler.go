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
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/bluele/gcache"
	"github.com/gorilla/mux"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var facetDefRegexp = regexp.MustCompile("^([^:]+):(.+:)?([0-9-]+)$")
var facetValRegexp = regexp.MustCompile("^(.+)\\.(true|false)$")
var facetRegexp = regexp.MustCompile("^facet_([^_]+)_(.+)$")
var filterRegexp = regexp.MustCompile("^filter_[0-9]+_(.+)$")

func (s *Server) searchHandler(w http.ResponseWriter, req *http.Request) {
	var facetCounter int64 = 1000
	var err error
	vars := mux.Vars(req)

	if pusher, ok := w.(http.Pusher); ok {
		pushfonts := []string{
			"/" + s.prefixes["static"] + "/font/inter/Inter-ExtraLight.woff2?v=3.15",
			"/" + s.prefixes["static"] + "/font/inter/Inter-Regular.woff2?v=3.15",
			"/" + s.prefixes["static"] + "/font/inter/Inter-Light.woff2?v=3.15",
			"/" + s.prefixes["static"] + "/font/inter/Inter-Bold.woff2?v=3.15",
			"/" + s.prefixes["static"] + "/font/inter/Inter-roman.var.woff2?v=3.15",
		}

		for _, furl := range pushfonts {
			s.log.Infof("pushing font %s", furl)
			if err := pusher.Push(furl, nil); err != nil {
				s.log.Errorf("Failed to push %s: %v", furl, err)
			}
		}
	}

	status := SearchStatus{
		BaseStatus: BaseStatus{
			Notifications: []Notification{},
			Self:          fmt.Sprintf("%s/%s", s.addrExt, strings.TrimLeft(req.URL.Path, "/")),
			BaseUrl:       s.addrExt.String(),
			Prefixes: map[string]string{
				"detail":      s.prefixes["detail"],
				"search":      s.prefixes["search"],
				"collections": s.prefixes["collections"],
				"cluster":     s.prefixes["cluster"],
				"google":      s.prefixes["cse"],
			},
			InstanceName: s.instanceName,
			SelfPath:     req.URL.Path,
			LoginUrl:     s.loginUrl,
			Title:        "Search",
			User:         nil,
			Token:        "",
			AmpBase:      "",
			Type:         "search",
			server:       s,
			RelPath:      s.relPath(req.URL.Path),
			Plain:        false,
		},
		QueryApi:        "api/search",
		Facet:           make(map[string]map[string]FacetCountField),
		CoreFacets:      []string{},
		Filter:          make(map[string][]string),
		Stats:           FacetCountResult{},
		EmptySearch:     false,
		MetaDescription: "Integrated Catalogue of Mediathek HGK FHNW",
	}

	params := []string{}
	for key, vals := range req.URL.Query() {
		if key == "token" || key == "logout" {
			continue
		}
		for _, val := range vals {
			params = append(params, fmt.Sprintf("%s=%s", key, url.QueryEscape(val)))
		}
	}
	status.RawQuery = strings.Join(params, "&")

	_, logout := req.URL.Query()["logout"]
	sess, _ := s.cookieStore.Get(req, "logged-in")
	var jwt []string = []string{""}
	var ok bool
	jwt, ok = req.URL.Query()["token"]
	if !ok {
		jwt = []string{""}
		var sessJWT interface{}
		sessJWT, ok = sess.Values["user"]
		if ok {
			jwt[0], ok = sessJWT.(string)
		}
	}
	if ok {
		// jwt in parameter?
		if len(jwt) == 0 {
			s.DoPanicf(nil, req, w, http.StatusForbidden, "invalid token %v", false, jwt)
			return
		}
		tokenstring := jwt[0]
		if tokenstring != "" {
			status.Token = tokenstring
			user, err := s.userFromToken(tokenstring, "")
			if err != nil {
				status.Notifications = append(status.Notifications, Notification{
					Id:      "notificationInvalidAccessToken",
					Message: fmt.Sprintf("%s - User logged out", err.Error()),
				})
				status.User = NewGuestUser(s)
				status.User.LoggedOut = true
			} else {
				status.User = user
			}
		}
	}
	if status.User == nil || logout {
		status.User = NewGuestUser(s)
	}
	if status.User.LoggedIn {
		jwt, err := NewJWT(
			status.User.Server.jwtKey,
			"search",
			"HS256",
			int64(status.User.Server.linkTokenExp.Seconds()),
			"catalogue",
			"mediathek",
			status.User.Id)
		if err != nil {
			s.DoPanicf(nil, req, w, http.StatusInternalServerError, "create token: %v", false, err)
			return
		}
		status.QueryApi = template.URL(fmt.Sprintf("%s/%s", status.RelPath, "api/search"))
		//status.QueryApi = template.URL(fmt.Sprintf("%s/%s?token=%s", status.RelPath, "api/search", jwt))

		/*
			jwt2, err := NewJWT(
				status.User.Server.jwtKey,
				"",
				"HS256",
				int64(status.User.Server.linkTokenExp.Seconds()),
				"catalogue",
				"mediathek",
				status.User.Id)
			if err != nil {
				s.DoPanicf(nil, req, w, http.StatusInternalServerError, "create token: %v", false, err)
				return
			}
		*/
		sess.Values["user"] = jwt
		sess.Options.MaxAge = 0 // int(s.sessionTimeout/time.Second)
	} else {
		sess.Values["user"] = ""
		sess.Options.MaxAge = -1
	}
	sess.Options.Path = "/"
	if err := sess.Save(req, w); err != nil {
		s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot store cookie logged-in: %v", false, err)
		return
	}

	ip, _, _ := net.SplitHostPort(req.RemoteAddr)
	for _, grp := range s.locations.Contains(ip) {
		status.User.Groups = append(status.User.Groups, grp)
	}

	facets := map[string]TermFacet{}
	for _, val := range s.facets {
		if _, ok := facets[val.Field]; !ok {
			facets[val.Field] = TermFacet{
				Selected: map[string]bool{},
				Prefix:   "",
				Limit:    0,
			}
		}
		for v, sel := range val.Restrict {
			facets[val.Field].Selected[v] = sel
		}
		status.CoreFacets = append(status.CoreFacets, val.Field)
	}

	var start int64 = 0
	var rows int64 = 10
	var search, lastsearch string
	var filterOrg = make(map[string][]string)

	for key, vals := range req.URL.Query() {
		if len(vals) == 0 {
			continue
		}
		val := vals[0]
		val = strings.TrimSpace(val)
		switch key {
		case "start":
			start, _ = strconv.ParseInt(val, 10, 64)
		case "rows":
			rows, _ = strconv.ParseInt(val, 10, 64)
		case "lastsearch":
			lastsearch = val
		case "searchtext":
			search = val
		case "visible":
			status.SearchResultVisible = val == "true"
		default:
			if found := facetRegexp.FindStringSubmatch(key); found != nil {
				fld := found[1]
				if m := facetValRegexp.FindStringSubmatch(val); m != nil {
					if _, ok := facets[fld]; !ok {
						facets[fld] = TermFacet{
							Selected: map[string]bool{},
							Prefix:   "",
							Limit:    0,
						}
					}
					v := m[1]
					if m[2] == "true" {
						facets[fld].Selected[v] = true
					} else {
						//Facets[fld].Selected[v] = false
					}
				}
			} else {
				if found := filterRegexp.FindStringSubmatch(key); found != nil {
					fld := found[1]
					if _, ok := filterOrg[fld]; !ok {
						filterOrg[fld] = []string{}
					}
					if val != "" {
						filterOrg[fld] = append(filterOrg[fld], val)
					}
				}
			}
		}
	}

	if start < 0 {
		start = 0
	}
	if search != lastsearch {
		start = 0
	}

	var showJSON bool
	filterOrg, filterField, qstr := s.string2QList(search, filterOrg)
	subfiltername := vars["subfilter"]
	if subfiltername == "data" {
		subfiltername = ""
		showJSON = true
	}

	if subfiltername != "" {
		var f *SubFilter = nil
		// check for configured subfilter
		for _, sf := range s.subFilters {
			if sf.Label == subfiltername {
				f = &sf
				break
			}
		}
		if f == nil {
			// load as collection
			doc, err := s.mts.LoadEntity(subfiltername)
			if err != nil {
				s.DoPanicf(nil, req, w, http.StatusNotFound, "error loading signature %s: %v", false, subfiltername, err)
				return
			}
			if doc == nil {
				s.DoPanicf(nil, req, w, http.StatusInternalServerError, "data of signature %s is nil", false, subfiltername)
				return
			}
			if filter, ok := (*doc.Meta)["Archive"]; ok {
				// todo: deal with search string
				_, ff, _ /* fg */ := s.string2QList(filter, nil)
				//filter_general = append(filter_general, fg...)
				for fld, vals := range ff {
					if _, ok := filterField[fld]; !ok {
						filterField[fld] = []string{}
					}
					filterField[fld] = append(filterField[fld], vals...)
				}
				status.Title = doc.Title
			}
			if facetstring, ok := (*doc.Meta)["Extra"]; ok {
				if fl := facetDefRegexp.FindStringSubmatch(facetstring); fl != nil {
					s.log.Infof("%v", fl)
					facetField := fl[1]
					facetPrefix := fl[2]
					facetLimit, err := strconv.ParseInt(fl[3], 10, 64)
					if err != nil {
						facetLimit = 0
					}
					if _, ok := facets[facetField]; !ok {
						facets[facetField] = TermFacet{
							Selected: map[string]bool{},
							Prefix:   "",
							Limit:    0,
						}
					}
					f := facets[facetField]
					f.Prefix = facetPrefix
					f.Limit = facetLimit
					facets[facetField] = f
				}
			}

		} else {
			// todo: deal with search string
			_, ff, _ /* fg */ := s.string2QList(f.Filter, nil)
			//filter_general = append(filter_general, fg...)
			for fld, vals := range ff {
				if _, ok := filterField[fld]; !ok {
					filterField[fld] = []string{}
				}
				filterField[fld] = append(filterField[fld], vals...)
			}
			status.Title = f.Name
		}
	}

	if len(filterField) == 0 && qstr == "" {
		total, facets, err := s.mts.StatsByACL(s.baseCatalog)
		if err != nil {
			s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot get statistics: %v", false, err)
			return
		}
		s.log.Infof("total records: %v", total)
		s.log.Infof("statistics by ACL: %v", facets)
		status.EmptySearch = true
		status.Stats = facets
		status.SearchResultTotal = int(total)

		result, err := s.queryCache.Get("empty")
		if err != nil && err != gcache.KeyNotFoundError {
			s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot access cache: %v", false, err)
			return
		}
		if err != gcache.KeyNotFoundError {
			s.log.Info("serving from cache")
			dt, err := Decompress(result.([]byte))
			if err != nil {
				s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot decompress cache: %v", false, err)
				return
			}
			w.Header().Set("Cache-Control", "max-age=14400, s-maxage=12200, stale-while-revalidate=9000, public")
			io.WriteString(w, string(dt))
			return
		}
		if showJSON {
			enc := json.NewEncoder(w)
			w.Header().Set("Content-type", "text/json")
			if err := enc.Encode(status); err != nil {
				s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot marshal solr doc", true, jwt)
				return
			}
		} else {
			s.log.Infof("search.amp.gohtml - empty")
			w.Header().Set("Cache-Control", "max-age=14400, s-maxage=12200, stale-while-revalidate=9000, public")
			if tpl, ok := s.templates["search.amp.gohtml"]; ok {
				var cacheBuffer bytes.Buffer
				writer := io.MultiWriter(&cacheBuffer, w)
				if err := tpl.Execute(writer, status); err != nil {
					s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot render template: %v", false, err)
					return
				}
				if err := s.queryCache.Set("empty", Compress(cacheBuffer.Bytes())); err != nil {
					s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot cache result: %v", false, err)
					return
				}
			}
		}
	}

	if len(s.baseCatalog) > 0 {
		if _, ok := filterField["catalog"]; !ok {
			filterField["catalog"] = []string{}
		}
		filterField["catalog"] = append(filterField["catalog"], s.baseCatalog...)
	}
	cfg := &SearchConfig{
		Fields:         make(map[string][]string),
		QStr:           qstr,
		FiltersFields:  filterField,
		Facets:         facets,
		Groups:         status.User.Groups,
		ContentVisible: status.SearchResultVisible,
		Start:          int(start),
		Rows:           int(rows),
		IsAdmin:        status.User.inGroup(s.adminGroup),
	}

	hk, err := Hash(cfg)
	if err != nil {
		s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot hash config: %v", false, err)
		return
	}

	result, err := s.queryCache.Get(hk)
	if err != nil && err != gcache.KeyNotFoundError {
		s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot access cache: %v", false, err)
		return
	}
	if err != gcache.KeyNotFoundError {
		s.log.Info("serving from cache")
		dt, err := Decompress(result.([]byte))
		if err != nil {
			s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot decompress cache: %v", false, err)
			return
		}
		w.Header().Set("Cache-Control", "max-age=14400, s-maxage=12200, stale-while-revalidate=9000, public")
		io.WriteString(w, string(dt))
		return
	}

	highlights, docs, total, facetFieldCount, err := s.mts.Search(cfg)
	if err != nil {
		s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot execute solr query: %v", false, err)
		return
	}
	status.Result, err = s.doc2result("", "", docs, total, facetFieldCount, facets, 0, &status.BaseStatus, "", highlights)
	if err != nil {
		s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot marshal result: %v", false, err)
		return
	}

	// todo: MD5 for facet names

	//status.SearchResult = template.JS(json)
	status.SearchResultRows = len(docs)
	status.SearchResultTotal = int(total)
	status.SearchResultStart = int(start)
	//status.SearchString = search
	status.SearchString = qstr
	status.Filter = filterOrg
	for _, f := range s.facets {
		vals := f.Restrict
		facet := f.Field
		status.Facet[facet] = map[string]FacetCountField{}
		for val, _ := range vals {
			if len(val) == 0 {
				continue
			}
			if val[0] == '\u0001' {
				continue
			}
			id := fmt.Sprintf("facet_%v_%v", facet, facetCounter)
			facetCounter++
			count := 0
			if _, ok := facetFieldCount[facet]; ok {
				for v, c := range facetFieldCount[facet] {
					if val == v {
						count = c
					}
				}
			}
			selected, ok := facets[facet].Selected[val]
			if !ok {
				selected = false
			}
			status.Facet[facet][id] = FacetCountField{
				Id:        id,
				Name:      fmt.Sprintf("%s (%d)", val, count),
				ShortName: val,
				Selected:  selected,
			}
			//status.FacetCount[id] = status.Facet[facet][id]
		}
	}
	for facet, _ := range facets {
		if InList(status.CoreFacets, facet) {
			continue
		}
		status.Facet[facet] = map[string]FacetCountField{}
		for val, c := range facetFieldCount[facet] {
			if len(val) == 0 {
				continue
			}
			if val[0] == '\u0001' {
				continue
			}

			//			re := regexp.MustCompile("([^a-zA-Z0-9])+")
			//id := fmt.Sprintf("facet_%s_%s", facet, fmt.Sprintf("%x", md5.Sum([]byte(val))))
			//id := "facet_" + fmt.Sprintf("%x", crc32.Checksum([]byte(facet+val), crc32q))
			id := fmt.Sprintf("facet_%v_%v", facet, facetCounter)
			facetCounter++
			selected, ok := facets[facet].Selected[val]
			if !ok {
				selected = false
			}
			status.Facet[facet][id] = FacetCountField{
				Id:        id,
				Name:      fmt.Sprintf("%s (%d)", val, c),
				ShortName: val,
				Selected:  selected,
			}
		}
	}

	if showJSON {
		enc := json.NewEncoder(w)
		w.Header().Set("Content-type", "text/json")
		if err := enc.Encode(status); err != nil {
			s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot marshal solr doc", true, jwt)
			return
		}
	} else {
		w.Header().Set("Cache-Control", "max-age=14400, s-maxage=12200, stale-while-revalidate=9000, public")
		s.log.Infof("search.amp.gohtml")
		if tpl, ok := s.templates["search.amp.gohtml"]; ok {
			var cacheBuffer bytes.Buffer
			writer := io.MultiWriter(&cacheBuffer, w)
			if err := tpl.Execute(writer, status); err != nil {
				s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot render template: %v", false, err)
				return
			}
			if err := s.queryCache.Set(hk, Compress(cacheBuffer.Bytes())); err != nil {
				s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot cache result: %v", false, err)
				return
			}
		}
	}
	return
}
