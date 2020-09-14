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
	"strconv"
	"strings"
)

func (s *Server) imageSearchHandler(w http.ResponseWriter, req *http.Request) {
	var err error
	vars := mux.Vars(req)

	status := ImageSearchStatus{
		Type:          "imagesearch",
		Notifications: []Notification{},
		Self:          fmt.Sprintf("%s/%s", s.addrExt, strings.TrimLeft(req.URL.Path, "/")),
		BaseUrl:       s.addrExt,
		SelfPath:      req.URL.Path,
		LoginUrl:      s.loginUrl,
		Title:         "Image Search",
		QueryApi:      "api/search",
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

	var start int64 = 0
	var rows int64 = 10
	var search, lastsearch string

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
		default:
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

	filters := []string{s.baseFilter, "mediatype:image"}
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
	var facets map[string]termFacet
	docs, total, facetFieldCount, err := s.mts.Search(qstr,
		filters,
		facets,
		status.User.Groups,
		true,
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
		if tpl, ok := s.templates["imagesearch"]; ok {
			if err := tpl.Execute(w, status); err != nil {
				s.DoPanicf(w, http.StatusInternalServerError, "cannot render template: %v", false, err)
				return
			}
		}
	}
	return
}
