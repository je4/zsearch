package service

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"net/http"
)

func (s *Server) userHandler(w http.ResponseWriter, req *http.Request) {
	// remove prefix and use whole rest of url as signature
	vars := mux.Vars(req)
	_, ok := vars["access"]
	if !ok {
		s.DoPanicf(w, http.StatusBadRequest, "no accesstype in url: %s", false, req.URL.Path)
		return
	}
	signature, ok := vars["signature"]
	if !ok {
		s.DoPanicf(w, http.StatusBadRequest, "no signature in url: %s", false, req.URL.Path)
		return
	}

	var user *User
	var err error
	jwt, ok := req.URL.Query()["token"]
	if ok {
		// jwt in parameter?
		if len(jwt) == 0 {
			s.DoPanicf(w, http.StatusForbidden, "invalid token %v", false, jwt)
			return
		}
		tokenstring := jwt[0]
		if tokenstring != "" {
			user, err = s.userFromToken(tokenstring, signature)
			if err != nil {
				s.DoPanicf(w, http.StatusForbidden, "%v", false, err)
				return
			}
		}
	}
	if user == nil {
		user = NewGuestUser(s)
	}

	js, err := json.Marshal(user)
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot marshal user: %v", false, user)
		return
	}
	w.Header().Set("ContentStr-Type", "application/json")
	w.Write(js)
}


