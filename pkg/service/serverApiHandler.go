package service

import (
	"encoding/json"
	"fmt"
	"github.com/drgrib/maps"
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
			id := fmt.Sprintf("%s_%s", facet, val)
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
			if p.Role != "author" && p.Role != "director" {
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

func solrOr(field string, values []string, weight1, weight2 int) string {
	result := ""
	for _, val := range values {
		trimmed := strings.Trim(val, `"`)
		withQuotes := val != trimmed
		if withQuotes {
			val = trimmed
		}
		if weight1 > 0 {
			if result != "" {
				result += " OR "
			}
			result += fmt.Sprintf(`%s:%s^%d`, field, source.EscapeSolrString(val), weight1)
		}
		if weight2 > 0 && !withQuotes {
			if result != "" {
				result += " OR "
			}
			result += fmt.Sprintf(`%s:%s*^%d`, field, source.EscapeSolrString(val), weight2)
		}
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

	var qstr string

	rexp := regexp.MustCompile(`([a-zA-Z0-9]+:([^ "]+|"[^"]+"))|([^ "]+)|"([^"]+)"`)
	slice := rexp.FindAllString(search, -1)
	if slice == nil {
		slice = []string{}
	}

	// expand to field an generic search
	rexp2 := regexp.MustCompile(`^(` + strings.Join(maps.GetKeysStringString(s.searchFields), `|`) + `):(.+)$`)
	fields := make(map[string][]string)
	gen := []string{}

	for _, f := range slice {
		fldq := rexp2.FindStringSubmatch(f)
		if fldq != nil {
			fldname, ok := s.searchFields[fldq[1]]
			if !ok {
				continue
			}
			if _, ok := fields[fldname]; !ok {
				fields[fldname] = []string{}
			}
			fields[fldname] = append(fields[fldname], fldq[2])
		} else {
			gen = append(gen, f)
		}
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
	if len(gen) > 0 {
		qstr = fmt.Sprintf("%s OR %s OR %s OR %s OR %s OR %s",
			solrOr("title", gen, 10, 10),
			solrOr("author", gen, 10, 10),
			solrOr("publisher", gen, 8, 10),
			solrOr("content", gen, 0, 6),
			solrOr("abstract", gen, 0, 8),
			solrOr("signature", gen, 20, 10),
		)
	}
	qstr2 := ""
	if len(fields) > 0 {
		for field, val := range fields {
			if qstr2 != "" {
				qstr2 += " OR "
			}
			qstr2 += solrOr(field, val, 30, 15)
		}
	}
	if qstr2 != "" {
		if qstr != "" {
			qstr = fmt.Sprintf("(%s) AND (%s)", qstr, qstr2)
		} else {
			qstr = qstr2
		}
	}

	s.log.Infof("Query: %s", qstr)

	facets := map[string][]string{"mediatype": []string{}}
	for name, vals := range req.Form {
		for key, _ := range facets {
			if strings.HasPrefix(name, key+"_") && len(vals) > 0 {
				val := vals[0]
				if fmt.Sprintf("%v", val) == "true" {
					facets[key] = append(facets[key], strings.TrimPrefix(name, key+"_"))
				}
			}
		}
	}

	docs, total, facetFields, err := s.mts.Search(qstr, []string{"zotero"}, facets, user.Groups, false, int(start), int(rows))
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

	json, err := doc2json(search, qstr, docs, total, facetFields, facets, start, user, next)
	if err != nil {
		s.DoPanicf(w, http.StatusInternalServerError, "cannot marshal result: %v", true, err)
		return
	}

	w.Write(json)

	return
}
