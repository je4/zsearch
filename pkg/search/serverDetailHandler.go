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
	"github.com/goph/emperror"
	"github.com/gorilla/mux"
	"github.com/juliangruber/go-intersect"
	"net"
	"net/http"
	"strings"
)

type ErrorHTTPStatus struct {
	status int
	err    error
}

func (err *ErrorHTTPStatus) Error() string {
	return fmt.Sprintf("%s - %s", http.StatusText(err.status), err.err.Error())
}

func (s *Server) getDetailStatus(signature, path, tokenstring, remoteHost string) (*DetailStatus, error) {
	status := DetailStatus{
		BaseStatus: BaseStatus{
			Type:          "detail",
			User:          nil,
			Self:          fmt.Sprintf("%s/%s", s.addrExt, strings.TrimLeft(path, "/")),
			Canonical:     fmt.Sprintf("%s/%s/%s", s.addrExt, s.prefixes["detail"], signature),
			BaseUrl:       s.addrExt,
			SelfPath:      path,
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
			//				s.DoPanicf(nil, req, w, http.StatusForbidden, "%v", err)
			//				return
		} else {
			status.User = user
		}
	}

	if status.User == nil {
		status.User = NewGuestUser(s)
	}

	for _, grp := range s.locations.Contains(remoteHost) {
		status.User.Groups = append(status.User.Groups, grp)
	}

	doc, err := s.mts.LoadEntity(signature)
	if err != nil {
		return nil, &ErrorHTTPStatus{
			status: http.StatusNotFound,
			err:    emperror.Wrapf(err, "we could not find signature #%s", signature),
		}
	}
	if doc == nil {
		return nil, &ErrorHTTPStatus{
			status: http.StatusInternalServerError,
			err:    emperror.Wrapf(err, "data of signature #%s is nil", signature),
		}
	}
	status.Doc = doc
	//	status.BaseStatus.OGPNamespace, status.BaseStatus.OGPMeta = doc.GetOpenGraph("1102189490244305", s.addrExt+path, s.mediaserverUri2Url)
	ldo := doc.GetJsonLD(fmt.Sprintf("%s/%s/%s", s.addrExt, s.prefixes["detail"], doc.Signature), s.mediaserverUri2Url)
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

	status.Title = status.Doc.CollectionTitle
	status.IsAmp = !status.User.LoggedIn && !status.User.LoggedOut && status.MetaOK

	metadescription := ""
	//	metadescription = fmt.Sprintf("Title: %s", doc.Title)
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
	if len(status.MetaDescription) >= 160 {
		status.MetaDescription = status.MetaDescription[0:155] + "..."
	}

	return &status, nil
}

func (s *Server) detailEmbedHandler(w http.ResponseWriter, req *http.Request) {
	// remove Prefix and use whole rest of url as signature
	vars := mux.Vars(req)
	signature, ok := vars["signature"]
	if !ok {
		s.DoPanicf(nil, req, w, http.StatusBadRequest, "no signature in url: %s", false, req.URL.Path)
		return
	}
	var tokenstring string
	jwt, ok := req.URL.Query()["token"]
	if ok {
		if len(jwt) > 0 {
			tokenstring = jwt[0]
		}
	}

	remoteHost, _, _ := net.SplitHostPort(req.Host)
	status, err := s.getDetailStatus(signature, req.URL.Path, tokenstring, remoteHost)
	if err != nil {
		if ehs, ok := err.(*ErrorHTTPStatus); ok {
			s.DoPanicf(nil, req, w, ehs.status, ehs.err.Error(), false)
		} else {
			s.DoPanicf(nil, req, w, http.StatusInternalServerError, err.Error(), false)
		}
		return
	}

	if !status.MetaOK || !status.ContentOK {
		w.WriteHeader(http.StatusForbidden)
		// if there's no error Template, there's no help...
		if tpl, ok := s.templates["forbidden.amp.gohtml"]; ok {
			tpl.Execute(w, status)
		}
		return
	}

	newStatus := struct {
		Media    Media
		Link     string
		LinkText string
		BaseUrl  string
	}{
		Link:     status.Canonical,
		LinkText: fmt.Sprintf(`"%s" aus "%s"`, status.Doc.Title, status.Title),
		BaseUrl:  status.BaseUrl,
	}

	uri := fmt.Sprintf("mediaserver:%s/%s", vars["embedCollection"], vars["embedSignature"])
	var template string
	for t, medias := range status.Doc.Media {
		for _, media := range medias {
			if media.Uri == uri {
				newStatus.Media = media
				switch t {
				case "video":
					template = "embedVideo.gohtml"
					break
				}
			}
			if template != "" {
				break
			}
		}
	}
	if template == "" {
		s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot embed media #%v", false, uri)
		return
	}
	if tpl, ok := s.templates[template]; ok {
		err = tpl.Execute(w, newStatus)
		if err != nil {
			s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot parse template: %+v", false, err)
			return
		}
	}
}

func (s *Server) detailHandler(w http.ResponseWriter, req *http.Request) {
	// remove Prefix and use whole rest of url as signature
	vars := mux.Vars(req)
	//	sub := vars["sub"]
	data := vars["data"] == "data"
	//embed := vars["embed"] == "embed"
	signature, ok := vars["signature"]
	if !ok {
		s.DoPanicf(nil, req, w, http.StatusBadRequest, "no signature in url: %s", false, req.URL.Path)
		return
	}
	var tokenstring string
	jwt, ok := req.URL.Query()["token"]
	if ok {
		if len(jwt) > 0 {
			tokenstring = jwt[0]
		}
	}

	remoteHost, _, _ := net.SplitHostPort(req.Host)
	status, err := s.getDetailStatus(signature, req.URL.Path, tokenstring, remoteHost)
	if err != nil {
		if ehs, ok := err.(*ErrorHTTPStatus); ok {
			s.DoPanicf(nil, req, w, ehs.status, ehs.err.Error(), false)
		} else {
			s.DoPanicf(nil, req, w, http.StatusInternalServerError, err.Error(), false)
		}
		return
	}

	if !status.MetaOK {
		w.WriteHeader(http.StatusForbidden)
		// if there's no error Template, there's no help...
		if tpl, ok := s.templates["forbidden.amp.gohtml"]; ok {
			tpl.Execute(w, status)
		}
		return
	}

	if data {
		w.Header().Set("Content-type", "text/json")
		enc := json.NewEncoder(w)
		if err := enc.Encode(status); err != nil {
			s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot marshal solr doc", true, jwt)
			return
		}
		return
	}

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

	if tpl, ok := s.templates["details.amp.gohtml"]; ok {
		err = tpl.Execute(w, status)
		if err != nil {
			s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot parse template: %+v", false, err)
			return
		}
	}

	//	w.Write([]byte(fmt.Sprintf("%s/%s", access, signature)))

}
