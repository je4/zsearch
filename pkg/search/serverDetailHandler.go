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
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/juliangruber/go-intersect"
	"net"
	"net/http"
	"strings"
)

func (s *Server) detailHandler(w http.ResponseWriter, req *http.Request) {
	// remove Prefix and use whole rest of url as signature
	vars := mux.Vars(req)
	sub := vars["sub"]
	signature, ok := vars["signature"]
	if !ok {
		s.DoPanicf(w, http.StatusBadRequest, "no signature in url: %s", false, req.URL.Path)
		return
	}

	status := DetailStatus{
		BaseStatus: BaseStatus{
			Type:          "detail",
			User:          nil,
			Self:          fmt.Sprintf("%s/%s", s.addrExt, strings.TrimLeft(req.URL.Path, "/")),
			BaseUrl:       s.addrExt,
			SelfPath:      req.URL.Path,
			LoginUrl:      s.loginUrl,
			Notifications: []Notification{},
			Token:         "",
			Prefixes: map[string]string{
				"detail":      s.prefixes["detail"],
				"search":      s.prefixes["search"],
				"collections": s.prefixes["collections"],
				"cluster":     s.prefixes["cluster"],
				"google":      s.prefixes["cse"],
			},
			AmpBase: "",
			Title:   "",
		},
		ContentOK: false,
		MetaOK:    false,
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
	status.BaseStatus.OGPNamespace, status.BaseStatus.OGPMeta = doc.GetOpenGraph(req.URL.Path, s.mediaserverUri2Url)
	ldo := doc.GetJsonLD(req.URL.Path, s.mediaserverUri2Url)
	if jsonstr, err := json.Marshal([]interface{}{ldo}); err == nil {
		status.BaseStatus.JsonLD = fmt.Sprintf(`<script type="application/ld+json">%s</script>`, string(jsonstr)) + "\n"
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
	for key, ref := range status.Doc.References {
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
					status.Doc.References[key].Title = doc.Title
				} else {
					status.Doc.References[key].Title = doc.Signature
					// remove reference if no rights
					// removeRefs = append(removeRefs, Key)
				}
			}
		}
	}
	// remove references, which failed on load
	for _, key := range removeRefs {
		status.Doc.References = append(status.Doc.References[:key], status.Doc.References[key+1:]...)
	}

	if !status.MetaOK {
		w.WriteHeader(http.StatusForbidden)
		// if there's no error Template, there's no help...
		if tpl, ok := s.templates["forbidden.amp.gohtml"]; ok {
			tpl.Execute(w, status)
		}
		return
	}

	status.Title = status.Doc.CollectionTitle
	status.IsAmp = !status.User.LoggedIn && !status.User.LoggedOut && status.MetaOK

	metadescription := ""
	metadescription = fmt.Sprintf("Title: %s", doc.Title)
	if len(doc.Persons) > 0 {
		metadescription += fmt.Sprintf("\nAuthor: ")
		for k, p := range doc.Persons {
			if k > 0 {
				metadescription += "; "
			}
			metadescription += p.Name
		}
	}
	if doc.Abstract != "" {
		metadescription += "\nAbstract: " + doc.Abstract
	}
	status.MetaDescription = strings.ReplaceAll(metadescription, "\"", "'")

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

	switch sub {
	case "data":
		enc := json.NewEncoder(w)
		w.Header().Set("Content-type", "text/json")
		if err := enc.Encode(status); err != nil {
			s.DoPanicf(w, http.StatusInternalServerError, "cannot marshal solr doc", true, jwt)
			return
		}
	case "meta":
		w.Header().Set("Content-type", doc.ContentMime)
		w.Write([]byte(doc.ContentStr))
	default:
		if tpl, ok := s.templates["details.amp.gohtml"]; ok {
			err = tpl.Execute(w, status)
			if err != nil {
				s.DoPanicf(w, http.StatusInternalServerError, "cannot parse template: %+v", false, err)
				return
			}
		}
	}

	//	w.Write([]byte(fmt.Sprintf("%s/%s", access, signature)))
}
