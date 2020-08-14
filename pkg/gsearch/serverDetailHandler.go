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
	"github.com/juliangruber/go-intersect"
	"net"
	"net/http"
	"strings"
)

func (s *Server) detailHandler(w http.ResponseWriter, req *http.Request) {
	// remove prefix and use whole rest of url as signature
	vars := mux.Vars(req)
	sub := vars["sub"]
	signature, ok := vars["signature"]
	if !ok {
		s.DoPanicf(w, http.StatusBadRequest, "no signature in url: %s", false, req.URL.Path)
		return
	}

	status := DetailStatus{
		Type:          "detail",
		User:          nil,
		ContentOK:     false,
		MetaOK:        false,
		Self:          fmt.Sprintf("%s/%s", s.addrExt, strings.TrimLeft(req.URL.Path, "/")),
		BaseUrl:       s.addrExt,
		SelfPath:      req.URL.Path,
		LoginUrl:      s.loginUrl,
		Notifications: []Notification{},
		Menu:          s.menu,
	}
	doc, err := s.mts.LoadEntity(signature)
	if err != nil {
		s.DoPanicf(w, http.StatusNotFound, "error loading signature %s: %v", false, signature, err)
		return
	}
	if doc == nil {
		s.DoPanicf(w, http.StatusInternalServerError, "data of signature %s is nil", false, signature)
		return
	}
	status.Doc = doc

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
			user, err := s.userFromToken(tokenstring, "detail:"+signature)
			if err != nil {
				status.Notifications = append(status.Notifications, Notification{
					Id:      "notificationInvalidAccessToken",
					Message: fmt.Sprintf("%s - User logged out", err.Error()),
				})
				status.User = NewGuestUser(s)
				status.User.LoggedOut = true
				//				s.DoPanicf(w, http.StatusForbidden, "%v", err)
				//				return
			} else {
				status.User = user
			}
		}
	}
	if status.User == nil {
		status.User = NewGuestUser(s)
	}
	ip, _, _ := net.SplitHostPort(req.RemoteAddr)
	for _, grp := range s.locations.Contains(ip) {
		status.User.Groups = append(status.User.Groups, grp)
	}

	for acl, groups := range status.Doc.ACL {
		for _, group := range groups {
			for _, ugroup := range status.User.Groups {
				if group == ugroup {
					switch acl {
					case "meta":
						status.MetaOK = true
					case "content":
						status.ContentOK = true
					}
				}
			}
			if group == s.guestGroup {
				switch acl {
				case "meta":
					status.MetaPublic = true
				case "content":
					status.ContentPublic = true
				}
			}
		}
	}

	for _, ugroup := range status.User.Groups {
		if s.adminGroup == ugroup {
			status.MetaOK = true
			status.ContentOK = true
		}
	}

	// load all references
	// title only if rights ok
	removeRefs := []int{}
	for key, ref := range status.Doc.Content.References {
		if ref.Title == "" {
			doc, err := s.mts.LoadEntity(ref.Signature)
			if err != nil {
				removeRefs = append(removeRefs, key)
			} else if doc != nil {
				acl_meta, ok := status.Doc.ACL["meta"]
				if !ok {
					acl_meta = []string{}
				}
				sect := intersect.Simple(status.User.Groups, acl_meta)
				if ok && len([]interface{}{sect}) > 0 {
					status.Doc.Content.References[key].Title = doc.Content.Title
				} else {
					status.Doc.Content.References[key].Title = doc.Id
					// remove reference if no rights
					// removeRefs = append(removeRefs, key)
				}
			}
		}
	}
	// remove references, which failed on load
	for _, key := range removeRefs {
		status.Doc.Content.References = append(status.Doc.Content.References[:key], status.Doc.Content.References[key+1:]...)
	}

	if !status.MetaOK {
		w.WriteHeader(http.StatusForbidden)
		// if there's no error Template, there's no help...
		err = s.forbiddenTemplate.Execute(w, status)
		return
	}

	status.Title = status.Doc.Content.CollectionTitle
	status.IsAmp = status.User.LoggedIn && !status.User.LoggedOut && status.MetaOK

	switch sub {
	case "data":
		enc := json.NewEncoder(w)
		w.Header().Set("Content-type", "text/json")
		if err := enc.Encode(status); err != nil {
			s.DoPanicf(w, http.StatusInternalServerError, "cannot marshal solr doc", true, jwt)
			return
		}
	case "meta":
		w.Header().Set("Content-type", doc.Content.ContentMime)
		w.Write([]byte(doc.Content.ContentStr))
	default:
		err = s.detailTemplate.Execute(w, status)
		if err != nil {
			s.DoPanicf(w, http.StatusInternalServerError, "cannot parse template: %+v", false, err)
			return
		}
	}

	//	w.Write([]byte(fmt.Sprintf("%s/%s", access, signature)))
}
