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
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"html/template"
	"net"
	"net/http"
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
			"/" + s.staticPrefix + "/font/inter/Inter-ExtraLight.woff2?v=3.15",
			"/" + s.staticPrefix + "/font/inter/Inter-Regular.woff2?v=3.15",
			"/" + s.staticPrefix + "/font/inter/Inter-Light.woff2?v=3.15",
			"/" + s.staticPrefix + "/font/inter/Inter-Bold.woff2?v=3.15",
			"/" + s.staticPrefix + "/font/inter/Inter-roman.var.woff2?v=3.15",
		}

		for _, furl := range pushfonts {
			s.log.Infof("pushing font %s", furl)
			if err := pusher.Push(furl, nil); err != nil {
				s.log.Errorf("Failed to push %s: %v", furl, err)
			}
		}
	}

	status := SearchStatus{
		Type:          "search",
		Notifications: []Notification{},
		Self:          fmt.Sprintf("%s/%s", s.addrExt, strings.TrimLeft(req.URL.Path, "/")),
		BaseUrl:       s.addrExt,
		SelfPath:      req.URL.Path,
		LoginUrl:      s.loginUrl,
		Title:         "Search",
		QueryApi:      "api/search",
		Facet:         make(map[string]map[string]FacetCountField),
		Menu:          s.menu,
		CoreFacets:    []string{},
		Filter:        make(map[string][]string),
	}

	jwt, ok := req.URL.Query()["token"]
	if ok {
		// jwt in parameter?
		if len(jwt) == 0 {
			s.DoPanicf(w, http.StatusForbidden, "invalid token %v", false, jwt)
			return
		}
		tokenstring := jwt[0]
		if tokenstring != "" {
			status.Token = tokenstring
			user, err := s.userFromToken(tokenstring, "search")
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
	if status.User == nil {
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
			s.DoPanicf(w, http.StatusInternalServerError, "create token: %v", false, err)
			return
		}
		status.QueryApi = template.URL(fmt.Sprintf("%s/%s?token=%s", s.addrExt, "api/search", jwt))
	}
	ip, _, _ := net.SplitHostPort(req.RemoteAddr)
	for _, grp := range s.locations.Contains(ip) {
		status.User.Groups = append(status.User.Groups, grp)
	}

	facets := map[string]termFacet{}
	for _, val := range s.facets {
		if _, ok := facets[val.Field]; !ok {
			facets[val.Field] = termFacet{
				selected: map[string]bool{},
				prefix:   "",
				limit:    0,
			}
		}
		for v, sel := range val.Restrict {
			facets[val.Field].selected[v] = sel
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
						facets[fld] = termFacet{
							selected: map[string]bool{},
							prefix:   "",
							limit:    0,
						}
					}
					v := m[1]
					if m[2] == "true" {
						facets[fld].selected[v] = true
					} else {
						//facets[fld].selected[v] = false
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

	filterOrg, filterField, qstr := s.string2QList(search, filterOrg)
	subfiltername, ok := vars["subfilter"]
	if ok {
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
				s.DoPanicf(w, http.StatusNotFound, "error loading signature %s: %v", false, subfiltername, err)
				return
			}
			if doc == nil {
				s.DoPanicf(w, http.StatusInternalServerError, "data of signature %s is nil", false, subfiltername)
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
						facets[facetField] = termFacet{
							selected: map[string]bool{},
							prefix:   "",
							limit:    0,
						}
					}
					f := facets[facetField]
					f.prefix = facetPrefix
					f.limit = facetLimit
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

	cfg := &SearchConfig{
		fields:         make(map[string][]string),
		qstr:           qstr,
		filters_fields: filterField,
		facets:         facets,
		groups:         status.User.Groups,
		contentVisible: status.SearchResultVisible,
		start:          int(start),
		rows:           int(rows),
		isAdmin:        status.User.inGroup(s.adminGroup),
	}

	highlights, docs, total, facetFieldCount, err := s.mts.Search(cfg)
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot execute solr query: %v", false, err)
		return
	}
	status.Result, err = s.doc2result("", "", docs, total, facetFieldCount, facets, 0, status.User, "", highlights)
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot marshal result: %v", false, err)
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
			selected, ok := facets[facet].selected[val]
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
			selected, ok := facets[facet].selected[val]
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

	status.MetaDescription = "Integrated Catalogue of Mediathek HGK FHNW"
	switch subfiltername {
	case "data":
		enc := json.NewEncoder(w)
		w.Header().Set("Content-type", "text/json")
		if err := enc.Encode(status); err != nil {
			s.DoPanicf(w, http.StatusInternalServerError, "cannot marshal solr doc", true, jwt)
			return
		}
	default:
		w.Header().Set("Cache-Control", "max-age=14400, s-maxage=12200, stale-while-revalidate=9000, public")
		if tpl, ok := s.templates["search.amp.gohtml"]; ok {
			if err := tpl.Execute(w, status); err != nil {
				s.DoPanicf(w, http.StatusInternalServerError, "cannot render template: %v", false, err)
				return
			}
		}
	}
	return
}
