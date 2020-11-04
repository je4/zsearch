package search

import (
	"bytes"
	"fmt"
	"github.com/bluele/gcache"
	"github.com/gorilla/mux"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
)

type GoogleResultItem struct {
	Title                           string
	Snippet                         string
	Thumbnail                       string
	ThumbnailWidth, ThumbnailHeight int64
	Domain                          string
	Link                            string
	Mimetype                        string
	FileFormat                      string
}

type ClusterResultStatus struct {
	BaseStatus
	SearchResultTotal int64
	SearchResultStart int64
	SearchResultRows  int64
	Items             []GoogleResultItem
	TotalResults      string
	SearchString      string
	Searches          []string
	CSEBase           string
	SearchToken       string
	SearchName        string
}

func (s *Server) clusterHandler(w http.ResponseWriter, req *http.Request) {

	vars := mux.Vars(req)

	csekey, ok := vars["csekey"]
	if !ok {
		s.DoPanicf(w, http.StatusNotFound, "no csekey in url", false)
		return
	}

	cx, ok := s.googleCSEKey[csekey]
	if !ok {
		s.DoPanicf(w, http.StatusNotFound, "invalid key %v", false, csekey)
		return
	}

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

	var start int64 = 0
	//var rows int64 = 10
	var search, lastsearch string

	for key, vals := range req.URL.Query() {
		if len(vals) == 0 {
			continue
		}
		val := vals[0]
		val = strings.TrimSpace(val)
		switch key {
		case "start":
			start, _ = strconv.ParseInt(val, 10, 64)
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

	hash, err := Hash(struct {
		Search string
		Start  int64
		Name   string
	}{
		Search: search,
		Start:  start,
		Name:   csekey,
	})
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot create hash", false)
		return
	}

	result, err := s.queryCache.Get(hash)
	if err != nil && err != gcache.KeyNotFoundError {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot access cache: %v", false, err)
		return
	}
	if err != gcache.KeyNotFoundError {
		s.log.Info("serving from cache")
		dt, err := Decompress(result.([]byte))
		if err != nil {
			s.DoPanicf(w, http.StatusInternalServerError, "cannot decompress cache: %v", false, err)
			return
		}
		w.Header().Set("Cache-Control", "max-age=14400, s-maxage=12200, stale-while-revalidate=9000, public")
		io.WriteString(w, string(dt))
		return
	}

	searches := []string{}
	for s, _ := range s.googleCSEKey {
		searches = append(searches, s)
	}

	var status *ClusterResultStatus
	if search != "" {
		resp, err := s.google.Cse.List().Q(search).Start(start).Cx(cx).Do()
		if err != nil {
			s.DoPanicf(w, http.StatusInternalServerError, "cannot search: %v", false, err)
			return
		}

		numResult, _ := strconv.ParseInt(resp.SearchInformation.TotalResults, 10, 64)
		status = &ClusterResultStatus{
			BaseStatus: BaseStatus{
				Prefixes: map[string]string{
					"detail":      s.detailPrefix,
					"search":      s.searchPrefix,
					"collections": s.collectionsPrefix,
					"cluster":     s.clusterSearchPrefix,
					"google":      s.googleSearchPrefix,
				},
				Type:          "search",
				Notifications: []Notification{},
				Self:          fmt.Sprintf("%s/%s", s.addrExt, strings.TrimLeft(req.URL.Path, "/")),
				BaseUrl:       s.addrExt,
				SelfPath:      req.URL.Path,
				LoginUrl:      s.loginUrl,
				Title:         "Wissenscluster",
			},
			SearchString:      search,
			SearchResultTotal: numResult,
			TotalResults:      resp.SearchInformation.FormattedTotalResults,
			SearchResultStart: start,
			Items:             []GoogleResultItem{},
			Searches:          searches,
			CSEBase:           fmt.Sprintf("%s/%s", s.addrExt, s.clusterSearchPrefix),
			SearchName:        csekey,
		}
		status.SearchResultRows = int64(len(resp.Items))

		for _, result := range resp.Items {
			status.Items = append(status.Items, GoogleResultItem{
				Title:           result.HtmlTitle,
				Snippet:         result.HtmlSnippet,
				Thumbnail:       "",
				ThumbnailWidth:  0,
				ThumbnailHeight: 0,
				Domain:          result.DisplayLink,
				Link:            result.Link,
				Mimetype:        result.Mime,
				FileFormat:      result.FileFormat,
			})
		}

	} else {
		status = &ClusterResultStatus{
			BaseStatus: BaseStatus{
				Prefixes: map[string]string{
					"detail":      s.detailPrefix,
					"search":      s.searchPrefix,
					"collections": s.collectionsPrefix,
					"google":      s.clusterSearchPrefix,
				},
				Type:          "search",
				Notifications: []Notification{},
				Self:          fmt.Sprintf("%s/%s", s.addrExt, strings.TrimLeft(req.URL.Path, "/")),
				BaseUrl:       s.addrExt,
				SelfPath:      req.URL.Path,
				LoginUrl:      s.loginUrl,
				Title:         "Wissenscluster",
			},
			SearchString:      search,
			SearchResultTotal: 0,
			TotalResults:      "0",
			SearchResultStart: start,
			Items:             []GoogleResultItem{},
			Searches:          searches,
			CSEBase:           fmt.Sprintf("%s/%s", s.addrExt, s.clusterSearchPrefix),
			SearchName:        csekey,
		}
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
		status.SearchToken = jwt
	}
	ip, _, _ := net.SplitHostPort(req.RemoteAddr)
	for _, grp := range s.locations.Contains(ip) {
		status.User.Groups = append(status.User.Groups, grp)
	}

	w.Header().Set("Cache-Control", "max-age=14400, s-maxage=12200, stale-while-revalidate=9000, public")
	if tpl, ok := s.templates["cluster.amp.gohtml"]; ok {
		var cacheBuffer bytes.Buffer
		writer := io.MultiWriter(&cacheBuffer, w)
		if err := tpl.Execute(writer, status); err != nil {
			s.DoPanicf(w, http.StatusInternalServerError, "cannot render template: %v", false, err)
			return
		}
		if err := s.queryCache.Set(hash, Compress(cacheBuffer.Bytes())); err != nil {
			s.DoPanicf(w, http.StatusInternalServerError, "cannot cache result: %v", false, err)
			return
		}
	}
}
