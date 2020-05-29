package service

import (
	"encoding/json"
	"net/http"
)

type searchResult struct {
	Items []searchResultItem `json:"items"`
	Total int64              `json:"total"`
	Start int64              `json:"start"`
	Rows  int64              `json:"rows"`
}

type searchResultItem struct {
	Id    string `json:"id"`
	Title string `json:"title"`
	Text  string `json:text`
}

func (s *Server) apiSearchHandler(w http.ResponseWriter, req *http.Request) {
	jwt, ok := req.URL.Query()["token"]
	var user *User = nil
	var err error

	if ok {
		// jwt in parameter?
		if len(jwt) == 0 {
			s.DoPanicf(w, http.StatusForbidden, "invalid token %v", true, jwt)
			return
		}
		tokenstring := jwt[0]
		if tokenstring != "" {
			user, err = s.userFromToken(tokenstring, "search")
			if err != nil {
				user = NewGuestUser(s)
				user.LoggedOut = true
			}
		}
	}
	if user == nil {
		user = NewGuestUser(s)
	}

	docs, total, err := s.mts.Search("", []string{"zotero"}, map[string][]string{"mediatype": []string{}}, user.Groups, false, 0, 10)
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot execute solr query: %v", true, err)
		return
	}

	result := &searchResult{
		Items: []searchResultItem{},
		Total: total,
		Start: 0,
		Rows:  10,
	}

	for _, doc := range docs {
		item := searchResultItem{
			Id:    doc.Id,
			Title: doc.Content.Title,
			Text:  "",
		}
		result.Items = append(result.Items, item)
		s.log.Infof("[%s] %s", doc.Id, doc.Content.Title)
	}

	jenc := json.NewEncoder(w)
	jenc.Encode(result)

	return
}
