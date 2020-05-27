package service

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/juliangruber/go-intersect"
	"net/http"
	"strings"
)

func (s *Server) detailHandler(w http.ResponseWriter, req *http.Request) {
	// remove prefix and use whole rest of url as signature
	vars := mux.Vars(req)
	signature, ok := vars["signature"]
	if !ok {
		s.DoPanicf(w, http.StatusBadRequest, "no signature in url: %s", req.URL.Path)
		return
	}

	status := DetailStatus{
		Doc:           nil,
		User:          nil,
		ContentOK:     false,
		MetaOK:        false,
		Self:          fmt.Sprintf("%s/%s", s.addrExt, strings.TrimLeft(req.URL.Path, "/")),
		SelfPath:      req.URL.Path,
		LoginUrl:      s.loginUrl,
		Notifications: []Notification{},
	}
	var err error
	status.Doc, err = s.mts.LoadEntity(signature)
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
	for key, ref := range status.Doc.Content.References {
		if ref.Title == "" {
			doc, err := s.mts.LoadEntity(ref.Signature)
			if err == nil {
				acl_meta, ok := status.Doc.ACL["meta"]
				if ok && len(intersect.Simple(status.User.Groups, acl_meta)) > 0 {
					status.Doc.Content.References[key].Title = doc.Content.Title
				} else {
					status.Doc.Content.References[key].Title = doc.Id
				}
			}
		}
	}


	if !status.MetaOK {
		w.WriteHeader(http.StatusForbidden)
		// if there's no error Template, there's no help...
		err = s.forbiddenTemplate.Execute(w, status)
		return
	}

	status.Title = status.Doc.Content.CollectionTitle
	status.IsAmp = status.User.LoggedIn && !status.User.LoggedOut && status.MetaOK

	err = s.detailTemplate.Execute(w, status)
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot parse template: %+v", err)
		return
	}
	//	w.Write([]byte(fmt.Sprintf("%s/%s", access, signature)))
}
