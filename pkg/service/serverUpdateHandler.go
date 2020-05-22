package service

import (
	"fmt"
	"github.com/gorilla/mux"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/generic"
	"net/http"
	"net/url"
	"strings"
)

func (s *Server) updateHandler(w http.ResponseWriter, req *http.Request) {
	// remove prefix and use whole rest of url as signature
	vars := mux.Vars(req)
	signature, ok := vars["signature"]
	if !ok {
		s.DoPanicf(w, http.StatusBadRequest, "no signature in url: %s", req.URL.Path)
		return
	}
	if s.ampCache == nil {
		s.DoPanicf(w, http.StatusInternalServerError, "no amp configured")
		return
	}
	doc, err := s.mts.LoadEntity(signature)
	if err != nil {
		s.DoPanicf(w, http.StatusNotFound, "error loading signature %s: %v", signature, err)
		return
	}

	jwt, ok := req.URL.Query()["token"]
	if ok {
		// jwt in parameter?
		if len(jwt) == 0 {
			s.DoPanicf(w, http.StatusForbidden, "invalid token %v", jwt)
			return
		}
		tokenstring := jwt[0]
		if tokenstring != "" {
			// jwt valid?
			claims, err := generic.CheckJWTValid(tokenstring, s.jwtKey, s.jwtAlg)
			if err != nil {
				s.DoPanicf(w, http.StatusForbidden, "invalid access token: %v", err)
				return
			}

			// sub given?
			sub, err := generic.GetClaim(claims, "sub")
			if err != nil {
				s.DoPanicf(w, http.StatusForbidden, "no sub in token: %v", err)
				return
			}
			// sub correct?
			if strings.ToLower(sub) != strings.ToLower("update:"+signature) {
				s.DoPanicf(w, http.StatusForbidden, "invalid sub %s (should be update:%s) in token: %v", sub, signature, err)
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
		s.DoPanicf(w, http.StatusNotFound, "%s is not an amp page (metadata not public): %v", signature, err)
		return
	}

	extUrl, err := url.Parse(s.addrExt)
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot parse addrExt %s: %v", s.addrExt, err)
		return
	}
	theUrl := fmt.Sprintf("%s/%s/%s", extUrl, s.detailPrefix, signature)
	updateUrl, err := s.ampCache.BuildUpdateUrl(theUrl, s.ampApiKey)
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot build update url: %v", err)
		return
	}
	s.log.Infof("update url: %v", updateUrl)
	w.Write([]byte(updateUrl))

	refresRSA, err := s.ampCache.BuildRefreshRSA(extUrl.Host)
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot build rsa refresh url: %v", err)
		return
	}
	w.Write([]byte("\n\n"))
	w.Write([]byte(refresRSA))

}
