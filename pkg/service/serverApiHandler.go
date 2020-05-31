package service

import (
	"encoding/json"
	"fmt"
	"github.com/goph/emperror"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/generic"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/source"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"text/scanner"
)

type searchResult struct {
	Items []searchResultItem `json:"items"`
	Total int64              `json:"total"`
	Start int64              `json:"start"`
	Rows  int64              `json:"rows"`
	Query string             `json:"query"`
	Next  string             `json:"next"`
}

type searchResultItem struct {
	Id         string   `json:"id"`
	Title      string   `json:"title"`
	Text       string   `json:"text"`
	Collection string   `json:"collection"`
	Authors    []string `json:"authors"`
	Link       string   `json:"link"`
	FirstItem  bool     `json:"firstitem"`
	Total      int64    `json:"total,omitempty"`
}

func doc2json(query string, docs []*source.Document, total int64, start int64, user *User, next string) ([]byte, error) {
	result := &searchResult{
		Items: []searchResultItem{},
		Total: total,
		Start: start,
		Rows:  int64(len(docs)),
		Query: query,
		Next:  next,
	}

	for key, doc := range docs {
		link := user.LinkSignature(doc.Id)
		if !strings.HasPrefix(strings.ToLower(link), "http") {
			link = "detail/" + link
		}
		item := searchResultItem{
			Id:         doc.Id,
			Title:      doc.Content.Title,
			Text:       "",
			Collection: doc.Content.CollectionTitle,
			Authors:    []string{},
			Link:       link,
		}
		if key == 0 {
			item.FirstItem = true
			item.Total = total
		}
		for _, p := range doc.Content.Persons {
			name := p.Name
			if p.Role != "author" {
				name += fmt.Sprintf(" (%s)", p.Role)
			}
			item.Authors = append(item.Authors, name)
		}
		result.Items = append(result.Items, item)
	}
	r, err := json.Marshal(result)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot marshal result")
	}
	return r, nil
}

func solrOr(field string, values []string, weight1, weight2, weight3, weight4 int) string {
	result := ""
	if weight1 > 0 {
		for _, val := range values {
			if result != "" {
				result += " OR "
			}
			result += fmt.Sprintf(`%s:%s^%d`, field, source.EscapeSolrString(val), weight1)
		}
	}
	if weight2 > 0 {
		for _, val := range values {
			if result != "" {
				result += " OR "
			}
			result += fmt.Sprintf(`%s:%s*^%d`, field, source.EscapeSolrString(val), weight2)
		}
	}
	if weight3 > 0 {
		if result != "" {
			result += " OR "
		}
		result += fmt.Sprintf(`%s:%s^%d`, field, source.EscapeSolrString(strings.Join(values, " ")), weight3)
	}
	if weight4 > 0 {
		if result != "" {
			result += " OR "
		}
		result += fmt.Sprintf(`%s:%s*^%d`, field, source.EscapeSolrString(strings.Join(values, " ")), weight4)
	}
	return result
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
	rowsstr, ok := req.Form["rows"]
	if ok && len(rowsstr) > 0 {
		rows, _ = strconv.ParseInt(rowsstr[0], 10, 64)
	}

	search := ""
	searchs := req.Form["search"]
	if len(searchs) == 1 {
		search = searchs[0]
	}

	var sc scanner.Scanner

	sc.Init(strings.NewReader(search))
	slice := []string{}
	for tok := sc.Scan(); tok != scanner.EOF; tok = sc.Scan() {
		slice = append(slice, sc.TokenText())
	}

	/*
	      (
	   		(title:"fell"^10 OR title:"down"^10 OR title:"the"^10 OR title:"mountains"^10 OR title:"fell down the mountains"^20 )
	   	OR (author:fell^10 OR author:down^10 OR author:the^10 OR author:mountains^10 OR author:fell down the mountains^20 )
	   	OR (publisher:fell^8 OR publisher:down^8 OR publisher:the^8 OR publisher:mountains^8 OR publisher:fell down the mountains^18 )
	   	OR (content:fell*^6 AND content:down*^6 AND content:the*^6 AND content:mountains*^6 OR content:fell down the mountains*^12 )
	   	OR (abstract:fell*^8 AND abstract:down*^8 AND abstract:the*^8 AND abstract:mountains*^8 OR abstract:fell down the mountains*^15 )
	   	OR ( signature:"fell down the mountains"^25 )
	       )
	*/
	var qstr string = "*:*"
	if len(slice) > 0 {
		qstr = fmt.Sprintf("%s OR %s OR %s OR %s OR %s OR %s",
			solrOr("title", slice, 10, 0, 20, 15),
			solrOr("author", slice, 10, 0, 20, 15),
			solrOr("publisher", slice, 8, 0, 18, 13),
			solrOr("content", slice, 0, 6, 12, 10),
			solrOr("abstract", slice, 0, 8, 15, 8),
			solrOr("signature", slice, 0, 0, 25, 18),
		)
	}

	s.log.Infof("Query: %s", qstr)

	docs, total, err := s.mts.Search(qstr, []string{"zotero"}, map[string][]string{"mediatype": []string{}}, user.Groups, false, int(start), int(rows))
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot execute solr query: %v", true, err)
		return
	}
	if total == 0 {
		s.DoPanicf(w, http.StatusNotFound, "no results found", true)
		return
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

	json, err := doc2json(qstr, docs, total, start, user, next)
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot marshal result: %v", true, err)
		return
	}

	w.Write(json)

	return
}
