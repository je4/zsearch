package search

import (
	"bytes"
	"fmt"
	"github.com/gorilla/mux"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type GoogleResultStatus struct {
	BaseStatus
	SearchName   string
	CX           string
	Searches     []string
	SearchString string
}

func (s *Server) googleHandler(w http.ResponseWriter, req *http.Request) {

	vars := mux.Vars(req)

	csekey, ok := vars["csekey"]
	if !ok {
		s.DoPanicf(nil, req, w, http.StatusNotFound, "no csekey in url", false)
		return
	}

	cx, ok := s.googleCSEKey[csekey]
	if !ok {
		s.DoPanicf(nil, req, w, http.StatusNotFound, "invalid Key %v", false, csekey)
		return
	}

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

	searches := []string{}
	for s, _ := range s.googleCSEKey {
		searches = append(searches, s)
	}

	status := &GoogleResultStatus{
		BaseStatus: BaseStatus{
			Prefixes: map[string]string{
				"detail":      s.prefixes["detail"],
				"search":      s.prefixes["search"],
				"collections": s.prefixes["collections"],
				"cluster":     s.prefixes["cluster"],
				"google":      s.prefixes["cse"],
			},
			Type:          "search",
			Notifications: []Notification{},
			Self:          fmt.Sprintf("%s/%s", s.addrExt, strings.TrimLeft(req.URL.Path, "/")),
			RawQuery:      req.URL.RawQuery,
			BaseUrl:       s.addrExt.String(),
			SelfPath:      req.URL.Path,
			RelPath:       s.relPath(req.URL.Path),
			LoginUrl:      s.loginUrl,
			Title:         "Wissenscluster",
			InstanceName:  s.instanceName,
			server:        s,
		},
		SearchName: csekey,
		CX:         cx.Key,
		Searches:   searches,
	}

	_, logout := req.URL.Query()["logout"]

	var tokenstring string
	session, _ := s.cookieStore.Get(req, "logged-in")
	var jwt []string = []string{""}
	sessJWT, ok := session.Values["user"]
	if ok {
		jwt[0], ok = sessJWT.(string)
	}
	if !ok {
		jwt, ok = req.URL.Query()["token"]
	}
	if ok {
		// jwt in parameter?
		if len(jwt) == 0 {
			s.DoPanicf(nil, req, w, http.StatusForbidden, "invalid token %v", false, jwt)
			return
		}
		tokenstring = jwt[0]
		if tokenstring != "" {
			status.Token = tokenstring
			user, err := s.userFromToken(tokenstring, "" /* "search" */)
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
		session.Options.MaxAge = int(s.sessionTimeout / time.Second)
		session.Values["user"] = tokenstring
	} else {
		session.Options.MaxAge = -1
	}
	if err := session.Save(req, w); err != nil {
		s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot store cookie logged-in: %v", false, err)
		return
	}
	ip, _, _ := net.SplitHostPort(req.RemoteAddr)
	for _, grp := range s.locations.Contains(ip) {
		status.User.Groups = append(status.User.Groups, grp)
	}

	w.Header().Set("Cache-Control", "max-age=14400, s-maxage=12200, stale-while-revalidate=9000, public")
	if tpl, ok := s.templates["google.amp.gohtml"]; ok {
		var cacheBuffer bytes.Buffer
		writer := io.MultiWriter(&cacheBuffer, w)
		if err := tpl.Execute(writer, status); err != nil {
			s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot render template: %v", false, err)
			return
		}
	}
}
