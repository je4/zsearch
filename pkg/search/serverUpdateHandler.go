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
	"github.com/gorilla/mux"
	"net/http"
	"strings"
)

func (s *Server) updateHandler(w http.ResponseWriter, req *http.Request) {
	// remove Prefix and use whole rest of url as signature
	vars := mux.Vars(req)
	signature, ok := vars["signature"]
	if !ok {
		s.DoPanicf(nil, req, w, http.StatusBadRequest, "no signature in url: %s", false, req.URL.Path)
		return
	}
	if s.ampCache == nil {
		s.DoPanicf(nil, req, w, http.StatusInternalServerError, "no amp configured", false)
		return
	}
	doc, err := s.mts.LoadEntity(signature)
	if err != nil {
		s.DoPanicf(nil, req, w, http.StatusNotFound, "error loading signature %s: %v", false, signature, err)
		return
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
			// jwt valid?
			claims, err := CheckJWTValid(tokenstring, s.jwtKey, s.jwtAlg)
			if err != nil {
				s.DoPanicf(nil, req, w, http.StatusForbidden, "invalid access token: %v", false, err)
				return
			}

			// sub given?
			sub, err := GetClaim(claims, "sub")
			if err != nil {
				s.DoPanicf(nil, req, w, http.StatusForbidden, "no sub in token: %v", false, err)
				return
			}
			// sub correct?
			if strings.ToLower(sub) != strings.ToLower("update:"+signature) {
				s.DoPanicf(nil, req, w, http.StatusForbidden, "invalid sub %s (should be update:%s) in token: %v", false, sub, signature, err)
				return
			}
		}
	}
	metaPublic := false
	for acl, groups := range doc.ACL {
		if acl == "meta" {
			for _, group := range groups {
				if group == s.guestGroup {
					metaPublic = true
					break
				}
			}
		}
	}
	if !metaPublic {
		s.DoPanicf(nil, req, w, http.StatusNotFound, "%s is not an amp page (metadata not public): %v", false, signature, err)
		return
	}

	theUrl := fmt.Sprintf("%s/%s/%s", s.addrExt.String(), s.prefixes["detail"], signature)
	updateUrl, err := s.ampCache.BuildUpdateUrl(theUrl, s.ampApiKey)
	if err != nil {
		s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot build update url: %v", false, err)
		return
	}
	s.log.Infof("update url: %v", updateUrl)
	w.Write([]byte(updateUrl))

	refresRSA, err := s.ampCache.BuildRefreshRSA(s.addrExt.Host)
	if err != nil {
		s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot build rsa refresh url: %v", false, err)
		return
	}
	w.Write([]byte("\n\n"))
	w.Write([]byte(refresRSA))

}
