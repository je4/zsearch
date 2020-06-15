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

package service

import (
	"encoding/json"
	"fmt"
	"github.com/goph/emperror"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/generic"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/source"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type facetField struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	Selected bool   `json:"selected"`
}

type searchResult struct {
	Items           []searchResultItem    `json:"items"`
	Total           int64                 `json:"total"`
	Start           int64                 `json:"start"`
	Rows            int64                 `json:"rows"`
	Query           string                `json:"query"`
	Search          string                `json:"search"`
	Next            string                `json:"next"`
	FacetFieldCount map[string]facetField `json:"facetfieldcount"`
}

type searchResultItem struct {
	Id         string   `json:"Id"`
	Type       string   `json:"type"`
	Title      string   `json:"title"`
	Text       string   `json:"text"`
	Collection string   `json:"collection"`
	Authors    []string `json:"authors"`
	AuthorText string   `json:"authortext"`
	Link       string   `json:"link"`
	FirstItem  bool     `json:"firstitem"`
	Total      int64    `json:"total,omitempty"`
	Date       string   `json:"date"`
}

func doc2json(search string, query string, docs []*source.Document, total int64, facetFieldCount source.FacetCountResult, facets map[string][]string, start int64, user *User, next string) ([]byte, error) {
	result := &searchResult{
		Items:           []searchResultItem{},
		Total:           total,
		Start:           start,
		Rows:            int64(len(docs)),
		Query:           query,
		Search:          search,
		Next:            next,
		FacetFieldCount: make(map[string]facetField),
	}

	for facet, vals := range facetFieldCount {
		for val, count := range vals {
			id := fmt.Sprintf("facet_%s_%s", facet, val)
			result.FacetFieldCount[id] = facetField{
				Id:   id,
				Name: fmt.Sprintf("%s (%d)", val, count),
				Selected: func() bool {
					res, ok := facets[facet]
					if !ok {
						return false
					}
					for _, n := range res {
						if n == val {
							return true
						}
					}
					return false
				}(),
			}
		}
	}

	for key, doc := range docs {
		link := user.LinkSignature(doc.Id)
		if !strings.HasPrefix(strings.ToLower(link), "http") {
			link = "detail/" + link
		}
		item := searchResultItem{
			Id:         doc.Id,
			Type:       doc.Content.Type,
			Title:      doc.Content.Title,
			Text:       "",
			Collection: doc.Content.CollectionTitle,
			Authors:    []string{},
			Link:       link,
			Date:       doc.Content.Date,
		}
		if key == 0 {
			item.FirstItem = true
			item.Total = total
		}
		for _, p := range doc.Content.Persons {
			name := p.Name
			if p.Role != "author" && p.Role != "director" && p.Role != "artist" {
				name += fmt.Sprintf(" (%s)", p.Role)
			}
			item.Authors = append(item.Authors, name)
		}
		if len(item.Authors) > 0 {
			item.AuthorText = item.Authors[0]
		}
		if len(item.Authors) > 1 {
			item.AuthorText += " et al."
		}

		result.Items = append(result.Items, item)
	}
	r, err := json.Marshal(result)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot marshal result")
	}
	return r, nil
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

	if req.Method == "POST" {
		if err := req.ParseMultipartForm(0); err != nil {
			s.DoPanicf(w, http.StatusInternalServerError, "cannot parse multipart formdata: %v", true, err)
			return
		}
	} else {
		if err := req.ParseForm(); err != nil {
			s.DoPanicf(w, http.StatusInternalServerError, "cannot parse formdata: %v", true, err)
			return
		}
	}

	var start int64 = 0
	var rows int64 = 10

	startstr, ok := req.Form["start"]
	if ok && len(startstr) > 0 {
		start, _ = strconv.ParseInt(startstr[0], 10, 64)
	}
	if start < 0 {
		start = 0
	}
	rowsstr, ok := req.Form["rows"]
	if ok && len(rowsstr) > 0 {
		rows, _ = strconv.ParseInt(rowsstr[0], 10, 64)
	}

	search := ""
	searchs := req.Form["search"]
	if len(searchs) == 1 {
		search = searchs[0]
	}

	lastsearch := ""
	lastsearchs := req.Form["lastsearch"]
	if len(lastsearchs) == 1 {
		lastsearch = lastsearchs[0]
	}

	if lastsearch != search {
		start = 0
	}

	qstr := s.string2Query(search)

	s.log.Infof("Query: %s", qstr)

	facets := map[string]map[string]bool{}
	r := regexp.MustCompile(`^facet_([^_]+)_(.+)$`)
	for name, states := range req.Form {
		matches := r.FindStringSubmatch(name)
		if matches != nil {
			field := matches[1]
			val := matches[2]
			fmt.Sprintf("%v, %v", field, val)
			if _, ok := facets[field]; !ok {
				facets[field] = make(map[string]bool)
			}
			for _, state := range states {
				if _, ok := facets[field][val]; !ok {
					facets[field][val] = false
				}
				if fmt.Sprintf("%v", state) == "true" {
					facets[field][val] = true
				}

			}
		}
	}

	docs, total, facetFields, err := s.mts.Search(qstr, []string{"zotero"}, facets, user.Groups, false, int(start), int(rows))
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot execute solr query: %v", true, err)
		return
	}

	// set zero-facets to zero
	for field, vals := range facets {
		if _, ok := facetFields[field]; !ok {
			facetFields[field] = make(map[string]int)
		}
		for val, _ := range vals {
			if _, ok := facetFields[field][val]; !ok {
				facetFields[field][val] = 0
			}
		}
	}
	next := ""
	if total > start+rows {
		next = fmt.Sprintf("%s/%s?search=%s&start=%d&rows=%d", s.addrExt, "api/search", url.QueryEscape(search), start+rows, rows)
		if user.LoggedIn {
			jwt, err := generic.NewJWT(
				user.Server.jwtKey,
				"search",
				"HS256",
				int64(user.Server.linkTokenExp.Seconds()),
				"catalogue",
				"mediathek",
				fmt.Sprintf("%v", user.Id))
			if err != nil {
				s.DoPanicf(w, http.StatusInternalServerError, "create token: %v", false, err)
				return
			}
			next += "&token=" + jwt
		}
	}

	json, err := doc2json(search, qstr, docs, total, facetFields, facets, start, user, next)
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot marshal result: %v", true, err)
		return
	}

	w.Write(json)

	return
}
