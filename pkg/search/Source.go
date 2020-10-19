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
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"github.com/goph/emperror"
	"github.com/je4/zsync/pkg/zotmedia"
	"github.com/vanng822/go-solr/solr"
	"html/template"
	"io"
)

func GUnzip(data string) (string, error) {
	var src, dest bytes.Buffer

	bytedata, err := base64.StdEncoding.DecodeString(data)
	if _, err := src.Write(bytedata); err != nil {
		return "", emperror.Wrap(err, "cannot write data into buffer")
	}
	zr, err := gzip.NewReader(&src)
	if err != nil {
		return "", emperror.Wrap(err, "cannot create gzip reader")
	}
	if _, err := io.Copy(&dest, zr); err != nil {
		return "", emperror.Wrap(err, "uncompress data")
	}
	return dest.String(), nil
}

type Person struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

type Media struct {
	Name     string `json:"name"`
	Mimetype string `json:"mimetype"`
	Type     string `json:"type"`
	Uri      string `json:"uri"`
	Width    int64  `json:"width"`
	Height   int64  `json:"height"`
	Duration int64  `json:"duration"`
}

type Query struct {
	Label  string
	Search string
}

type MediaList []Media

func (ml MediaList) Len() int           { return len(ml) }
func (ml MediaList) Swap(i, j int)      { ml[i], ml[j] = ml[j], ml[i] }
func (ml MediaList) Less(i, j int) bool { return ml[i].Name < ml[j].Name }

type Note struct {
	Title string
	Note  template.HTML
}

type Reference struct {
	Type      string
	Title     string
	Signature string
}

type Source interface {
	Name() string
	GetTitle() string
	GetPlace() string
	GetDate() string
	GetCollectionTitle() string
	GetPersons() []Person
	GetACL() map[string][]string
	GetCatalogs() []string
	GetCategories() []string
	GetTags() []string
	GetMedia(ms zotmedia.Mediaserver) map[string]MediaList
	GetPoster(ms zotmedia.Mediaserver) *Media
	GetNotes() []Note
	GetAbstract() string
	GetReferences() []Reference
	GetMeta() map[string]string
	GetExtra() map[string]string
	GetContentType() string
	GetQueries() []Query
	GetSolrDoc() *solr.Document
	GetContentString() string
	GetContentMime() string
}

type SourceData struct {
	Source          string               `json:"Source"`
	Title           string               `json:"title"`
	Place           string               `json:"place"`
	Date            string               `json:"date"`
	CollectionTitle string               `json:"collectiontitle"`
	Persons         []Person             `json:"persons"`
	Tags            []string             `json:"tags"`
	Media           map[string]MediaList `json:"media"`
	Poster          *Media               `json:poster`
	Notes           []Note               `json:"notes"`
	Abstract        string               `json:"abstract"`
	HasMedia        bool                 `json:"hasmedia"`
	References      []Reference          `json:"references"`
	Extra           map[string]string    `json:"extra"`
	Meta            map[string]string    `json:"meta"`
	Type            string               `json:"type"`
	Queries         []Query              `json:"queries"`
	ContentStr      string
	ContentMime     string
}
