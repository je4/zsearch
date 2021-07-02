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
	"fmt"
	isoduration "github.com/channelmeter/iso8601duration"
	"github.com/goph/emperror"
	"github.com/je4/zsearch/v2/pkg/mediaserver"
	"github.com/vanng822/go-solr/solr"
	"html/template"
	"io"
	"strings"
	"time"
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
	Width    int64  `json:"width,omitempty"`
	Height   int64  `json:"height,omitempty"`
	Duration int64  `json:"duration,omitempty"`
	Fulltext string `json:"fulltext,omitempty"`
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
	GetSignature() string
	Name() string
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
	GetMedia(ms mediaserver.Mediaserver) map[string]MediaList
	GetPoster(ms mediaserver.Mediaserver) *Media
	GetNotes() []Note
	GetUrl() string
	GetAbstract() string
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
}

type SourceData struct {
	Signature       string               `json:"signature"`
	Source          string               `json:"source"`
	Title           string               `json:"title"`
	Series          string               `json:"series"`
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
	Url             string               `json:"url"`
	Abstract        string               `json:"abstract"`
	References      []Reference          `json:"references"`
	Meta            *Metalist            `json:"meta"`
	Extra           *Metalist            `json:"extra"`
	Vars            *Varlist             `json:"vars"`
	Type            string               `json:"type"`
	Queries         []Query              `json:"queries"`
	ContentStr      string               `json:"-"`
	ContentMime     string               `json:"-"`
	HasMedia        bool                 `json:"hasmedia"`
	Mediatype       []string             `json:"mediatype"`
	DateAdded       time.Time            `json:"dateadded"`
	Timestamp       time.Time            `json:"timestamp"`
}

func InitSourceData(source Source, ms mediaserver.Mediaserver) *SourceData {
	sd := &SourceData{
		Signature:       source.GetSignature(),
		Source:          source.Name(),
		Title:           source.GetTitle(),
		Series:          source.GetSeries(),
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
		Url:             source.GetUrl(),
		Abstract:        source.GetAbstract(),
		References:      source.GetReferences(),
		Meta:            source.GetMeta(),
		Extra:           source.GetExtra(),
		Vars:            source.GetVars(),
		Type:            source.GetContentType(),
		Queries:         source.GetQueries(),
		ContentStr:      source.GetContentString(),
		ContentMime:     source.GetContentMime(),
		DateAdded:       source.GetDateAdded(),
		Mediatype:       []string{},
		Timestamp:       time.Now(),
	}
	sd.HasMedia = len(sd.Media) > 0
	for mt, _ := range sd.Media {
		sd.Mediatype = append(sd.Mediatype, mt)
	}
	return sd
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
func (sd *SourceData) GetJsonLD(self string, mediaserver func(uri string, params ...string) (string, error)) (result interface{}) {
	videos, videook := sd.Media["video"]
	audios, audiook := sd.Media["audio"]
	vData := make(JSONData)

	var description string
	if videook || audiook {
	}

	if videook {
		if len(videos) > 0 {
			video := videos[0]
			vData.set("@type", "VideoObject")
			vData.set("@context", "https://schema.org")

			if sd.Poster != nil {
				if imgUrl, err := mediaserver(sd.Poster.Uri, "resize", fmt.Sprintf("size%vx%v", sd.Poster.Width, sd.Poster.Height), "formatJPEG"); err == nil {
					vData.add("thumbnailUrl", imgUrl)
					thumb := make(JSONData)
					thumb.set("@type", "ImageObject")
					thumb.set("url", imgUrl)
					thumb.set("width", fmt.Sprintf("%v", sd.Poster.Width))
					thumb.set("height", fmt.Sprintf("%v", sd.Poster.Height))
					vData.add("thumbnail", thumb)
				}
			}
			if imgUrl, err := mediaserver(sd.Poster.Uri, "resize", fmt.Sprintf("size%vx%v", 640, 480), "crop", "formatJPEG"); err == nil {
				vData.add("thumbnailUrl", imgUrl)
				thumb := make(JSONData)
				thumb.set("@type", "ImageObject")
				thumb.set("url", imgUrl)
				thumb.set("width", "640")
				thumb.set("height", "480")
				vData.add("thumbnail", thumb)
			}
			if imgUrl, err := mediaserver(sd.Poster.Uri, "resize", fmt.Sprintf("size%vx%v", 480, 480), "crop", "formatJPEG"); err == nil {
				vData.add("thumbnailUrl", imgUrl)
				thumb := make(JSONData)
				thumb.set("@type", "ImageObject")
				thumb.set("url", imgUrl)
				thumb.set("width", "480")
				thumb.set("height", "480")
				vData.add("thumbnail", thumb)
			}
			// duration / width / height
			var isoDuration = isoduration.Duration{}
			isoDuration.Hours = int(video.Duration / 3600)
			isoDuration.Minutes = (int(video.Duration) % 3600) / 60
			isoDuration.Seconds = int(video.Duration) % 60
			vData.set("duration", fmt.Sprintf("%v", isoDuration.String()))
			vData.set("width", fmt.Sprintf("%v", video.Width))
			vData.set("height", fmt.Sprintf("%v", video.Height))
			if coll, sig, err := mediaserverUri2ColSig(video.Uri); err == nil {
				vData.set("embedurl", fmt.Sprintf("%s/embed/%s/%s", self, coll, sig))
			}
		}
	} else {
		if audiook {
			if len(audios) > 0 {
				audio := audios[0]
				vData.set("@type", "AudioObject")
				vData.set("@context", "https://schema.org")

				// duration / width / height
				var isoDuration = isoduration.Duration{}
				isoDuration.Hours = int(audio.Duration / 3600)
				isoDuration.Minutes = (int(audio.Duration) % 3600) / 60
				isoDuration.Seconds = int(audio.Duration) % 60
				vData.set("duration", fmt.Sprintf("%v", isoDuration.String()))
				/*
					if coll, sig, err := mediaserverUri2ColSig(audio.Uri); err == nil {
						vData.set("embedurl", fmt.Sprintf("%s/embed/%s/%s", self, coll, sig))
					}
				*/
			}
		}
	}
	if videook || audiook {
		vData.set("url", self)
		vData.set("contenturl", self)
		vData.set("name", sd.Title)
		vData.set("uploadDate", sd.DateAdded.Format("2006-01-02T15:04:05Z"))

		description = sd.Abstract
		// director / actor / ...
		for _, p := range sd.Persons {
			vData.add("author", p.Name)
			description += fmt.Sprintf("  %s: %s", strings.Title(p.Role), p.Name)
		}
		pd := ""
		if sd.Place != "" {
			pd = sd.Place
		}
		if sd.Date != "" {
			if pd != "" {
				pd += ", "
			}
			pd += sd.Date
		}
		if pd != "" {
			if description != "" {
				description += "\n\n"
			}
			description += pd
		}
		vData.set("description", strings.Trim(description, " \n"))

		if sd.Poster != nil {
			if imgUrl, err := mediaserver(sd.Poster.Uri, "resize", fmt.Sprintf("size%vx%v", sd.Poster.Width, sd.Poster.Height), "formatJPEG"); err == nil {
				vData.add("thumbnailUrl", imgUrl)
				thumb := make(JSONData)
				thumb.set("@type", "ImageObject")
				thumb.set("url", imgUrl)
				thumb.set("width", fmt.Sprintf("%v", sd.Poster.Width))
				thumb.set("height", fmt.Sprintf("%v", sd.Poster.Height))
				vData.add("thumbnail", thumb)
			}
			if imgUrl, err := mediaserver(sd.Poster.Uri, "resize", fmt.Sprintf("size%vx%v", 640, 480), "crop", "formatJPEG"); err == nil {
				vData.add("thumbnailUrl", imgUrl)
				thumb := make(JSONData)
				thumb.set("@type", "ImageObject")
				thumb.set("url", imgUrl)
				thumb.set("width", "640")
				thumb.set("height", "480")
				vData.add("thumbnail", thumb)
			}
			if imgUrl, err := mediaserver(sd.Poster.Uri, "resize", fmt.Sprintf("size%vx%v", 480, 480), "crop", "formatJPEG"); err == nil {
				vData.add("thumbnailUrl", imgUrl)
				thumb := make(JSONData)
				thumb.set("@type", "ImageObject")
				thumb.set("url", imgUrl)
				thumb.set("width", "480")
				thumb.set("height", "480")
				vData.add("thumbnail", thumb)
			}
		}
		return vData
	}
	return nil
}

func (sd *SourceData) GetOpenGraph(app_id, self string, mediaserver func(uri string, params ...string) (string, error)) (namespace string, ogstr string) {
	var ogdata = make(OGData)

	namespace = "https://ogp.me/ns#"

	ogdata.set("fb:app_id", app_id)
	ogdata.set("og:title", sd.Title)
	ogdata.set("og:type", "website")
	ogdata.set("og:url", self)
	switch sd.Type {
	}
	if videos, ok := sd.Media["video"]; ok {
		if len(videos) > 0 {
			//namespace = "https://ogp.me/ns/video#"
			video := videos[0]
			// type
			ogdata.set("og:type", "video.other")

			// director / actor / ...
			for _, p := range sd.Persons {
				switch p.Role {
				case "director":
					ogdata.add("og:video:director", p.Name)
				case "artist":
					ogdata.add("og:video:director", p.Name)
				default:
					ogdata.add(fmt.Sprintf("og:video:actor:%s", p.Role), p.Name)
				}
			}
			// duration / width / height
			ogdata.set("og:video:duration", fmt.Sprintf("%v", video.Duration))
			ogdata.set("og:video:width", fmt.Sprintf("%v", video.Width))
			ogdata.set("og:video:height", fmt.Sprintf("%v", video.Height))

			// release
			ogdata.set("og:video:release_data", sd.Date)

			// url
			ogdata.set("og:video:url", self)
			ogdata.set("og:video:secure_url", self)
		}
	}
	ogdata.set("og:description", sd.Abstract)
	if sd.Poster != nil {
		if imgUrl, err := mediaserver(sd.Poster.Uri, "resize", fmt.Sprintf("size%vx%v", sd.Poster.Width, sd.Poster.Height), "formatJPEG"); err == nil {
			ogdata.set("og:image", imgUrl)
			ogdata.set("og:image:url", imgUrl)
			ogdata.set("og:image:secure_url", imgUrl)
			ogdata.set("og:image:width", fmt.Sprintf("%v", sd.Poster.Width))
			ogdata.set("og:image:height", fmt.Sprintf("%v", sd.Poster.Height))
			ogdata.set("og:image:type", "image/jpeg")
		}
	}
	for key, vals := range ogdata {
		ogstr += fmt.Sprintf(`   <meta property="%s" value="%s">`, key, strings.Join(vals, "; ")) + "\n"
	}
	return namespace, ogstr
}
