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
	"fmt"
	"html/template"
	"net"
	"net/http"
	"strings"
)

func (s *Server) clusterAllHandler(w http.ResponseWriter, req *http.Request) {
	var err error

	if pusher, ok := w.(http.Pusher); ok {
		// Push is supported.
		furl := "/" + s.prefixes["static"] + "/font/inter/Inter-roman.var.woff2?v=3.15"
		s.log.Infof("pushing font %s", furl)
		if err := pusher.Push(furl, nil); err != nil {
			s.log.Errorf("Failed to push %s: %v", furl, err)
		}
		furl = "/" + s.prefixes["static"] + "/font/inter/Inter-Bold.woff2?v=3.15"
		s.log.Infof("pushing font %s", furl)
		if err := pusher.Push(furl, nil); err != nil {
			s.log.Errorf("Failed to push %s: %v", furl, err)
		}
	}

	status := ClusterStatus{
		BaseStatus: BaseStatus{
			User:          nil,
			Token:         "",
			AmpBase:       "",
			Type:          "Collections",
			Notifications: []Notification{},
			Self:          fmt.Sprintf("%s/%s", s.addrExt, strings.TrimLeft(req.URL.Path, "/")),
			BaseUrl:       s.addrExt.String(),
			SelfPath:      req.URL.Path,
			RelPath:       s.relPath(req.URL.Path),
			LoginUrl:      s.loginUrl,
			Title:         "Wissenscluster",
			Prefixes: map[string]string{
				"detail":      s.prefixes["detail"],
				"search":      s.prefixes["search"],
				"collections": s.prefixes["collections"],
				"cluster":     s.prefixes["cluster"],
				"google":      s.prefixes["cse"],
			},
			InstanceName: s.instanceName,
			server:       s,
		},
		QueryApi: template.URL(fmt.Sprintf("%s/search", s.prefixes["api"])),
		Result:   []*SourceData{},
	}

	jwt, ok := req.URL.Query()["token"]
	if ok {
		// jwt in parameter?
		if len(jwt) == 0 {
			s.DoPanicf(nil, req, w, http.StatusForbidden, "invalid token %v", false, jwt)
			return
		}
		tokenstring := jwt[0]
		if tokenstring != "" {
			status.Token = tokenstring
			user, err := s.userFromToken(tokenstring, "cluster")
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
		_, err := NewJWT(
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
		//status.QueryApi = template.URL(fmt.Sprintf("%s/%s?token=%s", s.addrExt, "api/search", jwt))
		status.QueryApi = template.URL(fmt.Sprintf("%s/%s", s.addrExt, "api/search"))
	}
	ip, _, _ := net.SplitHostPort(req.RemoteAddr)
	for _, grp := range s.locations.Contains(ip) {
		status.User.Groups = append(status.User.Groups, grp)
	}

	//qstr := "*:*"
	//s.log.Infof("Query: %s", qstr)

	filters_fields := make(map[string][]string)
	filters_fields["catalog"] = []string{s.clusterCatalog}

	var facets map[string]TermFacet
	cfg := &SearchConfig{
		FiltersFields:  filters_fields,
		QStr:           "",
		Facets:         facets,
		Groups:         status.User.Groups,
		ContentVisible: status.SearchResultVisible,
		Start:          int(0),
		Rows:           int(1000),
		IsAdmin:        status.User.inGroup(s.adminGroup),
	}
	_, docs, total, _, err := s.mts.Search(cfg)
	if err != nil {
		s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot execute solr query: %v", false, err)
		return
	}

	// sort documents into result sets
	for _, doc := range docs {
		if srch, ok := (*doc.Meta)["Archive"]; ok && strings.TrimSpace(srch) != "" {
			status.Result = append(status.Result, doc)
		}
	}

	//status.SearchResult = template.JS(json)
	status.SearchResultRows = len(docs)
	status.SearchResultTotal = int(total)
	status.SearchResultStart = int(0)

	status.MetaDescription = "Search Cluster of Mediathek HGK FHNW"
	w.Header().Set("Cache-Control", "max-age=14400, s-maxage=12200, stale-while-revalidate=9000, public")
	if tpl, ok := s.templates["clusterall.amp.gohtml"]; ok {
		if err := tpl.Execute(w, status); err != nil {
			s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot render template: %v", false, err)
			return
		}
	}
	return
}
