package search

import (
	"fmt"
	"net/http"
	"strings"
)

func (s *Server) reloadTemplateHandler(w http.ResponseWriter, req *http.Request) {
	var err error
	var reloadTemplatesSignature = fmt.Sprintf("%s:reloadtemplates", s.prefixes["api"])

	jwt, ok := req.URL.Query()["token"]
	if !ok {
		s.DoPanicf(nil, req, w, http.StatusForbidden, "no token to access template reload", true)
		return
	}
	// jwt in parameter?
	if len(jwt) == 0 {
		s.DoPanicf(nil, req, w, http.StatusForbidden, "invalid token %v", false, jwt)
		return
	}
	tokenstring := jwt[0]
	if tokenstring == "" {
		s.DoPanicf(nil, req, w, http.StatusForbidden, "empty token to access template reload", true)
		return
	}
	claims, err := CheckJWTValid(tokenstring, s.jwtKey, s.jwtAlg)
	if err != nil {
		s.DoPanicf(nil, req, w, http.StatusForbidden, "invalid access token - %v: %v", true, tokenstring, err)
		return
	}
	sub, err := GetClaim(claims, "sub")
	if err != nil {
		s.DoPanicf(nil, req, w, http.StatusForbidden, "no sub in token - %v", true, tokenstring)
		return
	}
	// sub correct?
	if strings.ToLower(sub) != strings.ToLower(reloadTemplatesSignature) {
		s.DoPanicf(nil, req, w, http.StatusForbidden, "invalid subject %v token, should be %v - %v", true, sub, reloadTemplatesSignature, tokenstring)
		return
	}

	for name, templateFiles := range s.templatesFiles {
		tpl, err := initTemplate(templateFiles, name, s.funcMap)
		if err != nil {
			s.DoPanicf(nil, req, w, http.StatusInternalServerError, "cannot initialize template %v: %v", true, name, err)
			return
		}
		s.templates[name] = tpl
	}

	s.DoPanicf(nil, req, w, http.StatusOK, "%v templates initialized", true, len(s.templatesFiles))
	return
}
