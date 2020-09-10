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
	"strings"
)

var tagFieldRegexp = regexp.MustCompile("^(area|field):(.+)$")

func (s *Server) collectionsHandler(w http.ResponseWriter, req *http.Request) {
	var err error
	vars := mux.Vars(req)
	subfiltername, ok := vars["subfilter"]

	if pusher, ok := w.(http.Pusher); ok {
		// Push is supported.
		furl := "/" + s.staticPrefix + "/font/inter/Inter-roman.var.woff2?v=3.15"
		s.log.Infof("pushing font %s", furl)
		if err := pusher.Push(furl, nil); err != nil {
			s.log.Errorf("Failed to push %s: %v", furl, err)
		}
		furl = "/" + s.staticPrefix + "/font/inter/Inter-Bold.woff2?v=3.15"
		s.log.Infof("pushing font %s", furl)
		if err := pusher.Push(furl, nil); err != nil {
			s.log.Errorf("Failed to push %s: %v", furl, err)
		}
	}

	status := CollectionsStatus{
		Type:          "Collections",
		Notifications: []Notification{},
		Self:          fmt.Sprintf("%s/%s", s.addrExt, strings.TrimLeft(req.URL.Path, "/")),
		BaseUrl:       s.addrExt,
		SelfPath:      req.URL.Path,
		LoginUrl:      s.loginUrl,
		Title:         "Collections",
		QueryApi:      "api/search",
		Menu:          s.menu,
		Result:        map[string][]*Document{},
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

	qstr := "*:*"
	s.log.Infof("Query: %s", qstr)

	filters := []string{fmt.Sprintf("catalog:\"%s\"", s.collectionsCatalog)}

	var facets map[string]map[string]bool
	docs, total, _, err := s.mts.Search(qstr,
		filters,
		facets,
		status.User.Groups,
		status.SearchResultVisible,
		int(0),
		int(1000))
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot execute solr query: %v", false, err)
		return
	}

	// sort documents into result sets
	for _, doc := range docs {
		for _, tag := range doc.Content.Tags {
			if r := tagFieldRegexp.FindStringSubmatch(tag); r != nil {
				field := r[2]
				if _, ok := status.Result[field]; !ok {
					status.Result[field] = []*Document{}
				}
				status.Result[field] = append(status.Result[field], doc)
				break
			}
		}
	}

	//status.SearchResult = template.JS(json)
	status.SearchResultRows = len(docs)
	status.SearchResultTotal = int(total)
	status.SearchResultStart = int(0)

	status.MetaDescription = "Collections of Mediathek HGK FHNW"
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
		if tpl, ok := s.templates["collections.amp.gohtml"]; ok {
			if err := tpl.Execute(w, status); err != nil {
				s.DoPanicf(w, http.StatusInternalServerError, "cannot render template: %v", false, err)
				return
			}
		}
	}
	return
}
