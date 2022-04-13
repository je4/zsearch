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
	"github.com/pkg/errors"
	"github.com/vanng822/go-solr/solr"
	"html/template"
	"io"
	"time"
)

func GUnzip(data string) (string, error) {
	var src, dest bytes.Buffer

	bytedata, err := base64.StdEncoding.DecodeString(data)
	if _, err := src.Write(bytedata); err != nil {
		return "", errors.Wrap(err, "cannot write data into buffer")
	}
	zr, err := gzip.NewReader(&src)
	if err != nil {
		return "", errors.Wrap(err, "cannot create gzip reader")
	}
	if _, err := io.Copy(&dest, zr); err != nil {
		return "", errors.Wrap(err, "uncompress data")
	}
	return dest.String(), nil
}

type Person struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

type Media struct {
	Name        string `json:"name"`
	Mimetype    string `json:"mimetype"`
	Type        string `json:"type"`
	Uri         string `json:"uri"`
	Width       int64  `json:"width,omitempty"`
	Height      int64  `json:"height,omitempty"`
	Orientation int64  `json:"orientation,omitempty"`
	Duration    int64  `json:"duration,omitempty"`
	Fulltext    string `json:"fulltext,omitempty"`
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

func (ml *Metalist) UnmarshalJSON(b []byte) error {
	type kv struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	var arr []kv

	m := Metalist{}
	if err := json.Unmarshal(b, &arr); err != nil {
		return err
	}
	for _, val := range arr {
		m[val.Key] = val.Value
	}
	*ml = m
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

type Varlist map[string][]string

func (vl *Varlist) UnmarshalJSON(b []byte) error {
	type kv struct {
		Key   string   `json:"key"`
		Value []string `json:"value"`
	}
	var arr []kv

	m := Varlist{}
	if err := json.Unmarshal(b, &arr); err != nil {
		return err
	}
	for _, val := range arr {
		m[val.Key] = val.Value
	}
	*vl = m
	return nil
}

func (vl Varlist) MarshalJSON() ([]byte, error) {
	type kv struct {
		Key   string   `json:"key"`
		Value []string `json:"value"`
	}
	var arr []kv
	for key, val := range vl {
		arr = append(arr, kv{Key: key, Value: val})
	}
	return json.Marshal(arr)
}

func (vl Varlist) Append(key string, values []string) {
	if _, ok := vl[key]; !ok {
		vl[key] = []string{}
	}
	vl[key] = append(vl[key], values...)
}

func (vl Varlist) AppendMap(mv map[string][]string) {
	for key, values := range mv {
		vl.Append(key, values)
	}
}

func (vl Varlist) Unique() *Varlist {
	// todo: optimize it
	unique := func(arr []string) []string {
		occured := map[string]bool{}
		result := []string{}
		for e := range arr {
			// check if already the mapped
			// variable is set to true or not
			if occured[arr[e]] != true {
				occured[arr[e]] = true
				// Append to result slice.
				result = append(result, arr[e])
			}
		}

		return result
	}
	result := Varlist{}
	for key, values := range vl {
		result.Append(key, unique(values))
	}
	return &result
}

type Source interface {
	GetSource() string
	GetSignature() string
	GetSignatureOriginal() string
	//Name() string
	GetTitle() string
	GetSeries() string
	GetPlace() string
	GetDate() string
	GetCollectionTitle() string
	GetPersons() []Person
	GetACL() map[string][]string
	GetCatalogs() []string
	GetCategories() []string
	GetTags() []string
	GetMedia() map[string]MediaList
	GetPoster() *Media
	GetNotes() []Note
	GetUrl() string
	GetAbstract() string
	GetRights() string
	GetLicense() string
	GetReferences() []Reference
	GetMeta() *Metalist
	GetExtra() *Metalist
	GetVars() *Varlist
	GetContentType() string
	GetQueries() []Query
	GetSolrDoc() *solr.Document
	GetContentString() string
	GetContentMime() string
	GetDateAdded() time.Time
	GetPublisher() string
}

type JSONData map[string]interface{}

func (jd JSONData) set(key string, value interface{}) {
	jd[key] = value
}

func (jd JSONData) add(key string, value interface{}) {
	if _, ok := jd[key]; !ok {
		jd[key] = []interface{}{}
	}
	if _, ok := jd[key].([]interface{}); ok {
		jd[key] = append(jd[key].([]interface{}), value)
	}
}

type OGData map[string][]string

func (og OGData) add(key, val string) {
	if _, ok := og[key]; !ok {
		og[key] = []string{}
	}
	og[key] = append(og[key], val)
}

func (og OGData) set(key, val string) {
	og[key] = []string{val}
}
