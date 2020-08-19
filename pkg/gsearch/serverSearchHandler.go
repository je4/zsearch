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
package gsearch

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

func (s *Server) searchHandler(w http.ResponseWriter, req *http.Request) {
	var err error
	vars := mux.Vars(req)

	status := SearchStatus{
		Type:          "search",
		Notifications: []Notification{},
		Self:          fmt.Sprintf("%s/%s", s.addrExt, strings.TrimLeft(req.URL.Path, "/")),
		BaseUrl:       s.addrExt,
		SelfPath:      req.URL.Path,
		LoginUrl:      s.loginUrl,
		Title:         "Search",
		QueryApi:      "api/search",
		FacetCount:    make(map[string]FacetCountField),
		Menu:          s.menu,
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

	facets := map[string]map[string]bool{}
	for _, val := range s.facets {
		if _, ok := facets[val.Field]; !ok {
			facets[val.Field] = map[string]bool{}
		}
		for v, sel := range val.Restrict {
			facets[val.Field][v] = sel
		}
	}

	var start int64 = 0
	var rows int64 = 10
	var search, lastsearch string

	facetRegex := regexp.MustCompile("^facet_([^_]+)_([^_]+)$")

	for key, vals := range req.URL.Query() {
		if len(vals) == 0 {
			continue
		}
		val := vals[0]
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
			found := facetRegex.FindAllStringSubmatch(key, -1)
			if len(found) > 0 {
				if len(found[0]) == 3 {
					fld := found[0][1]
					v := found[0][2]
					if _, ok := facets[fld]; !ok {
						facets[fld] = map[string]bool{}
					}
					if val == "true" {
						facets[fld][v] = true
					} else {
						facets[fld][v] = false
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

	qstr := s.string2Query(search)
	s.log.Infof("Query: %s", qstr)

	filters := []string{s.baseFilter}
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
			for _, q := range doc.Content.Queries {
				if strings.ToLower(q.Label) == "group" {
					filters = append(filters, s.string2Query(q.Search))
					status.Title = doc.Content.Title
				}
			}
		} else {
			filters = append(filters, f.Filter)
			status.Title = f.Name
		}

	}

	docs, total, facetFieldCount, err := s.mts.Search(qstr,
		filters,
		facets,
		status.User.Groups,
		status.SearchResultVisible,
		int(start),
		int(rows))
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot execute solr query: %v", false, err)
		return
	}
	status.Result, err = s.doc2result("", "", docs, total, facetFieldCount, facets, 0, status.User, "")
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot marshal result: %v", false, err)
		return
	}

	//status.SearchResult = template.JS(json)
	status.SearchResultRows = len(docs)
	status.SearchResultTotal = int(total)
	status.SearchResultStart = int(start)
	status.SearchString = search
	for _, f := range s.facets {
		vals := f.Restrict
		facet := f.Field
		for val, _ := range vals {
			id := fmt.Sprintf("facet_%s_%s", facet, val)
			count := 0
			if _, ok := facetFieldCount[facet]; ok {
				for v, c := range facetFieldCount[facet] {
					if val == v {
						count = c
					}
				}
			}
			selected, ok := facets[facet][val]
			if !ok {
				selected = false
			}
			status.FacetCount[id] = FacetCountField{
				Id:        id,
				Name:      fmt.Sprintf("%s (%d)", val, count),
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
		if err := s.searchTemplate.Execute(w, status); err != nil {
			s.DoPanicf(w, http.StatusInternalServerError, "cannot render template: %v", false, err)
			return
		}
	}
	return
}
