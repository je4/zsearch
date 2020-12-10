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
	"github.com/gorilla/mux"
	"net/http"
)

func (s *Server) userHandler(w http.ResponseWriter, req *http.Request) {
	// remove Prefix and use whole rest of url as signature
	vars := mux.Vars(req)
	_, ok := vars["access"]
	if !ok {
		s.DoPanicf(nil, req, w, http.StatusBadRequest, "no accesstype in url: %s", false, req.URL.Path)
		return
	}
	signature, ok := vars["signature"]
	if !ok {
		s.DoPanicf(nil, req, w, http.StatusBadRequest, "no signature in url: %s", false, req.URL.Path)
		return
	}

	var user *User
	var err error
	jwt, ok := req.URL.Query()["token"]
	if ok {
		// jwt in parameter?
		if len(jwt) == 0 {
			s.DoPanicf(nil, req, w, http.StatusForbidden, "invalid token %v", false, jwt)
			return
		}
		tokenstring := jwt[0]
		if tokenstring != "" {
			user, err = s.userFromToken(tokenstring, signature)
			if err != nil {
				s.DoPanicf(nil, req, w, http.StatusForbidden, "%v", false, err)
				return
			}
		}
	}
	if user == nil {
		user = NewGuestUser(s)
	}

	js, err := json.Marshal(user)
	if err != nil {
		s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot marshal user: %v", false, user)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}
