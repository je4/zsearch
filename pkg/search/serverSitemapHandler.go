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
	"encoding/xml"
	"fmt"
	"github.com/bluele/gcache"
	"github.com/gorilla/mux"
	"github.com/je4/sitemap"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
)

var sitemapSize = 5000

func (s *Server) sitemapHandler(w http.ResponseWriter, req *http.Request) {
	var sitemapTemplatesSignature = fmt.Sprintf("%s:sitemap", s.prefixes["api"])

	vars := mux.Vars(req)

	jwt, ok := req.URL.Query()["token"]
	if !ok {
		s.DoPanicf(w, http.StatusForbidden, "no token to access template reload", true)
		return
	}
	// jwt in parameter?
	if len(jwt) == 0 {
		s.DoPanicf(w, http.StatusForbidden, "invalid token %v", false, jwt)
		return
	}
	tokenstring := jwt[0]
	if tokenstring == "" {
		s.DoPanicf(w, http.StatusForbidden, "empty token to access template reload", true)
		return
	}
	claims, err := CheckJWTValid(tokenstring, s.jwtKey, s.jwtAlg)
	if err != nil {
		s.DoPanicf(w, http.StatusForbidden, "invalid access token - %v: %v", true, tokenstring, err)
		return
	}
	sub, err := GetClaim(claims, "sub")
	if err != nil {
		s.DoPanicf(w, http.StatusForbidden, "no sub in token - %v", true, tokenstring)
		return
	}
	// sub correct?
	if strings.ToLower(sub) != strings.ToLower(sitemapTemplatesSignature) {
		s.DoPanicf(w, http.StatusForbidden, "invalid subject %v token, should be %v - %v", true, sub, sitemapTemplatesSignature, tokenstring)
		return
	}

	var xmlbytes []byte

	startstr, ok := vars["start"]
	if !ok {
		var facets map[string]termFacet
		var filterField map[string][]string
		var groups = []string{"global/guest"}
		cfg := &SearchConfig{
			Fields:         make(map[string][]string),
			QStr:           "",
			FiltersFields:  filterField,
			Facets:         facets,
			Groups:         groups,
			ContentVisible: false,
			Start:          int(0),
			Rows:           int(0),
			IsAdmin:        false,
		}

		_, _, total, _, err := s.mts.Search(cfg)
		if err != nil {
			s.DoPanicf(w, http.StatusInternalServerError, "cannot execute solr query: %v", false, err)
			return
		}

		num := int(math.Ceil(float64(total) / float64(sitemapSize)))
		sitemapindex := sitemap.NewSitemapIndex()
		for i := 0; i < num; i++ {
			us := fmt.Sprintf("%v/%v/sitemap/%v", s.addrExt, s.prefixes["api"], i*sitemapSize)
			sitemapindex.URLs = append(sitemapindex.URLs, &sitemap.URL{Loc: us})
		}

		xmlbytes, err = xml.Marshal(sitemapindex)
		if err != nil {
			s.DoPanicf(w, http.StatusInternalServerError, "cannot marshal xml: %v", false, err)
			return
		}
		w.Header().Set("Content-Type", "text/xml")
		w.Write(xmlbytes)
	} else {
		start, err := strconv.ParseInt(startstr, 10, 64)
		if err != nil {
			s.DoPanicf(w, http.StatusInternalServerError, "cannot parse start parameter %s: %v", false, startstr, err)
			return
		}
		if int(start)%sitemapSize != 0 {
			s.DoPanicf(w, http.StatusNotFound, "invalid start value %v", false, start)
			return
		}
		var facets map[string]termFacet
		var filterField map[string][]string
		var groups = []string{"global/guest"}
		cfg := &SearchConfig{
			Fields:         make(map[string][]string),
			QStr:           "",
			FiltersFields:  filterField,
			Facets:         facets,
			Groups:         groups,
			ContentVisible: false,
			Start:          int(start),
			Rows:           int(sitemapSize),
			IsAdmin:        false,
		}

		hk, err := Hash(cfg)
		if err != nil {
			s.DoPanicf(w, http.StatusInternalServerError, "cannot hash config: %v", false, err)
			return
		}

		result, err := s.queryCache.Get(hk)
		if err != nil && err != gcache.KeyNotFoundError {
			s.DoPanicf(w, http.StatusInternalServerError, "cannot access cache: %v", false, err)
			return
		}
		if err != gcache.KeyNotFoundError {
			s.log.Info("serving from cache")
			dt, err := Decompress(result.([]byte))
			if err != nil {
				s.DoPanicf(w, http.StatusInternalServerError, "cannot decompress cache: %v", false, err)
				return
			}
			w.Header().Set("Cache-Control", "max-age=14400, s-maxage=12200, stale-while-revalidate=9000, public")
			w.Header().Set("Content-Type", "text/xml")
			io.WriteString(w, string(dt))
			return
		}

		_, docs, _, _, err := s.mts.Search(cfg)
		if err != nil {
			s.DoPanicf(w, http.StatusInternalServerError, "cannot execute solr query: %v", false, err)
			return
		}
		sm := sitemap.New()
		for _, item := range docs {
			us := fmt.Sprintf("%s/%s/%s", s.addrExt, s.prefixes["detail"], item.Signature)
			u := &sitemap.URL{
				Loc: us,
			}
			if videos, hasVideo := item.Media["video"]; hasVideo {
				if len(videos) > 0 {
					var us string
					if item.Poster != nil {
						us, err = s.mediaserverUri2Url(item.Poster.Uri, "resize", "size800x600", "formatPNG")
						if err != nil {
							s.DoPanicf(w, http.StatusInternalServerError, "cannot parse poster media uri %s: %v", false, videos[0].Uri, err)
							return
						}
					}

					abstract := ""
					for _, p := range item.Persons {
						abstract += fmt.Sprintf("%s (%s)\n", p.Name, p.Role)
					}
					abstract += "\n" + item.Abstract
					u.Video = &sitemap.Video{
						ThumbnailLoc: us,
						Title:        item.Title,
						Description:  abstract,
						ContentLoc:   "",
						PlayerLoc:    fmt.Sprintf("%s/%s/%s", s.addrExt, s.prefixes["detail"], item.Signature),
						AllowEmbed:   "no",
						Duration:     videos[0].Duration,
						GalleryLoc: &sitemap.GalleryLocation{
							Title: "Mediathek HGK, Basel",
							Value: "https://mediathek.hgk.fhnw.ch",
						},
					}
				}
			}
			sm.URLs = append(sm.URLs, u)
		}

		w.Header().Set("Content-Type", "text/xml")
		sm.WriteTo(w)
	}
}
