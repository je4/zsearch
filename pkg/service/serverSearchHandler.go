package service

import (
	"fmt"
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

	docs, total, err := s.mts.Search("", []string{"zotero"}, map[string][]string{"mediatype": []string{}}, status.User.Groups, false, 0, 10)
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot execute solr query: %v", false, err)
		return
	}
	if total == 0 {
		s.DoPanicf(w, http.StatusNoContent, "no results found", false)
		return
	}
	json, err := doc2json(docs, total, 0)
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot marshal result: %v", false, err)
		return
	}
	status.SearchResult = template.JS(json)

	if err := s.searchTemplate.Execute(w, status); err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot render template: %v", false, err)
		return
	}
	return
}
