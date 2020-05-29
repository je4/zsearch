package service

import (
	"fmt"
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

	if err := s.searchTemplate.Execute(w, status); err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot render template: %v", false, err)
		return
	}
	return
}
