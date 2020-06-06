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
		FacetCount:    []FacetCountField{},
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

	docs, total, facetFieldCount, err := s.mts.Search("", []string{"zotero"}, map[string][]string{"mediatype": []string{}}, status.User.Groups, false, 0, 10)
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot execute solr query: %v", false, err)
		return
	}
	if total == 0 {
		s.DoPanicf(w, http.StatusNoContent, "no results found", false)
		return
	}
	json, err := doc2json("", "", docs, total, facetFieldCount, 0, status.User, "")
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot marshal result: %v", false, err)
		return
	}
	status.SearchResult = template.JS(json)
	status.SearchResultRows = len(docs)
	status.SearchResultTotal = int(total)
	status.SearchResultStart = 0
	for facet, vals := range facetFieldCount {
		for val, count := range vals {
			status.FacetCount = append(status.FacetCount, FacetCountField{
				Id:       fmt.Sprintf("%s_%s", facet, val),
				Name:     fmt.Sprintf("%s (%d)", val, count),
				Selected: false,
			})
		}
	}


	if err := s.searchTemplate.Execute(w, status); err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot render template: %v", false, err)
		return
	}
	return
}
