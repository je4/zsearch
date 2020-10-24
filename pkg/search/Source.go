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
	"encoding/json"
	"github.com/goph/emperror"
	"github.com/je4/zsearch/pkg/mediaserver"
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
	Label  string `json:"label"`
	Search string `json:"search"`
}

type MediaList []Media

func (ml MediaList) Len() int           { return len(ml) }
func (ml MediaList) Swap(i, j int)      { ml[i], ml[j] = ml[j], ml[i] }
func (ml MediaList) Less(i, j int) bool { return ml[i].Name < ml[j].Name }

type Note struct {
	Title string        `json:"title"`
	Note  template.HTML `json:"note"`
}

type Reference struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Signature string `json:"signature"`
}

type Metalist map[string]string

func (ml Metalist) UnmarshalJSON(b []byte) error {
	type kv struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	var arr []kv

	ml = map[string]string{}
	if err := json.Unmarshal(b, &arr); err != nil {
		return err
	}
	for _, val := range arr {
		(ml)[val.Key] = val.Value
	}

	return nil
}

func (ml Metalist) MarshalJSON() ([]byte, error) {
	type kv struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	var arr []kv
	for key, val := range ml {
		arr = append(arr, kv{Key: key, Value: val})
	}
	return json.Marshal(arr)
}

type Source interface {
	GetSignature() string
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
	GetMedia(ms mediaserver.Mediaserver) map[string]MediaList
	GetPoster(ms mediaserver.Mediaserver) *Media
	GetNotes() []Note
	GetAbstract() string
	GetReferences() []Reference
	GetMeta() Metalist
	GetExtra() Metalist
	GetContentType() string
	GetQueries() []Query
	GetSolrDoc() *solr.Document
	GetContentString() string
	GetContentMime() string
}

type SourceData struct {
	Signature       string               `json:"signature"`
	Source          string               `json:"source"`
	Title           string               `json:"title"`
	Place           string               `json:"place"`
	Date            string               `json:"date"`
	CollectionTitle string               `json:"collectiontitle"`
	Persons         []Person             `json:"persons"`
	ACL             map[string][]string  `json:"acl"`
	Catalog         []string             `json:"catalog"`
	Category        []string             `json:"category"`
	Tags            []string             `json:"tags"`
	Media           map[string]MediaList `json:"media"`
	Poster          *Media               `json:"poster"`
	Notes           []Note               `json:"notes"`
	Abstract        string               `json:"abstract"`
	References      []Reference          `json:"references"`
	Meta            Metalist             `json:"meta"`
	Extra           Metalist             `json:"extra"`
	Type            string               `json:"type"`
	Queries         []Query              `json:"queries"`
	ContentStr      string               `json:"-"`
	ContentMime     string               `json:"-"`
	HasMedia        bool                 `json:"hasmedia"`
	Mediatype       []string             `json:"mediatype"`
}

func InitSourceData(source Source, ms mediaserver.Mediaserver) *SourceData {
	sd := &SourceData{
		Signature:       source.GetSignature(),
		Source:          source.Name(),
		Title:           source.GetTitle(),
		Place:           source.GetPlace(),
		Date:            source.GetDate(),
		CollectionTitle: source.GetCollectionTitle(),
		Persons:         source.GetPersons(),
		ACL:             source.GetACL(),
		Catalog:         source.GetCatalogs(),
		Category:        source.GetCategories(),
		Tags:            source.GetTags(),
		Media:           source.GetMedia(ms),
		Poster:          source.GetPoster(ms),
		Notes:           source.GetNotes(),
		Abstract:        source.GetAbstract(),
		References:      source.GetReferences(),
		Meta:            source.GetMeta(),
		Extra:           source.GetExtra(),
		Type:            source.GetContentType(),
		Queries:         source.GetQueries(),
		ContentStr:      source.GetContentString(),
		ContentMime:     source.GetContentMime(),
		Mediatype:       []string{},
	}
	sd.HasMedia = len(sd.Media) > 0
	for mt, _ := range sd.Media {
		sd.Mediatype = append(sd.Mediatype, mt)
	}
	return sd
}
