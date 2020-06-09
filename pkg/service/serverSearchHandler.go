package service

import (
	"fmt"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/generic"
	"html/template"
	"net/http"
	"strings"
)

func (s *Server) searchHandler(w http.ResponseWriter, req *http.Request) {

	status := SearchStatus{
		Type:          "search",
		Notifications: []Notification{},
		Self:          fmt.Sprintf("%s/%s", s.addrExt, strings.TrimLeft(req.URL.Path, "/")),
		SelfPath:      req.URL.Path,
		LoginUrl:      s.loginUrl,
		Title:         "search",
		QueryApi:      "api/search",
		FacetCount:    make(map[string]FacetCountField),
		Facets:        s.facets,
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
		jwt, err := generic.NewJWT(
			status.User.Server.jwtKey,
			"search",
			"HS256",
			int64(status.User.Server.linkTokenExp.Seconds()),
			"catalogue",
			"mediathek",
			fmt.Sprintf("%v", status.User.Id))
		if err != nil {
			s.DoPanicf(w, http.StatusInternalServerError, "create token: %v", false, err)
			return
		}
		status.QueryApi = template.URL(fmt.Sprintf("%s/%s?token=%s", s.addrExt, "api/search", jwt))
	}

	search := ""
	searchs := req.URL.Query()["search"]
	if len(searchs) == 1 {
		search = searchs[0]
	}

	facets := map[string][]string{"mediatype": []string{}}
	for name, vals := range req.URL.Query() {
		for key, _ := range facets {
			if strings.HasPrefix(key+"_", name) && len(vals) > 0 {
				val := vals[0]
				fmt.Sprintf("%v", val)
				facets[key] = append(facets[key], strings.TrimPrefix(key+"_", name))
			}
		}
	}

	docs, total, facetFieldCount, err := s.mts.Search(search, []string{"zotero"}, facets, status.User.Groups, false, 0, 10)
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot execute solr query: %v", false, err)
		return
	}
	if total == 0 {
		s.DoPanicf(w, http.StatusNoContent, "no results found", false)
		return
	}
	json, err := doc2json("", "", docs, total, facetFieldCount, facets, 0, status.User, "")
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot marshal result: %v", false, err)
		return
	}
	status.SearchResult = template.JS(json)
	status.SearchResultRows = len(docs)
	status.SearchResultTotal = int(total)
	status.SearchResultStart = 0
	for facet, vals := range s.facets {
		for _, val := range vals {
			id := fmt.Sprintf("%s_%s", facet, val)
			count := 0
			if _, ok := facetFieldCount[facet]; ok {
				for v, c := range facetFieldCount[facet] {
					if val == v {
						count = c
					}
				}
			}
			status.FacetCount[id] = FacetCountField{
				Id:       id,
				Name:     fmt.Sprintf("%s (%d)", val, count),
				ShortName: val,
				Selected: false,
			}
		}
	}

	if err := s.searchTemplate.Execute(w, status); err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot render template: %v", false, err)
		return
	}
	return
}
