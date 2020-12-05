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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/goph/emperror"
	"github.com/je4/zsearch/pkg/mediaserver"
	"github.com/vanng822/go-solr/solr"
	"html/template"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

type SourceZoteroDeprecated struct {
	mts        *MTSolr
	ZData      ZoteroData           `json:"ZData"`
	CollMeta   map[string]string    `json:"collmeta"`
	doc        *solr.Document       `json:"-"`
	contentStr string               `json:"-"`
	medias     map[string]MediaList `jsnon:"-"`
}

func (zot *SourceZoteroDeprecated) GetACL() map[string][]string {
	panic("implement me")
}

func (zot *SourceZoteroDeprecated) GetCatalogs() []string {
	panic("implement me")
}

func (zot *SourceZoteroDeprecated) GetCategories() []string {
	panic("implement me")
}

var zoteroDeprecatedIgnoreMetaFields = []string{
	"AbstractNote",
	"Collections",
	"Creators",
	"Date",
	"Media",
	"Place",
	"Relations",
	"Tags",
	"Title",
	"Extra",
	"Note",
}

// Name:value
var zoteroDeprecatedTagVariable = regexp.MustCompile(`^(acl_meta|acl_content):(.+)$`)

func NewSourceZoteroDeprecated(entry *cacheEntry, mts *MTSolr) (*SourceZoteroDeprecated, error) {
	zot := &SourceZoteroDeprecated{
		ZData:      ZoteroData{},
		CollMeta:   map[string]string{},
		mts:        mts,
		contentStr: entry.ContentStr,
	}
	return zot, zot.Init(entry)
}

func (zot *SourceZoteroDeprecated) Init(entry *cacheEntry) error {
	err := json.Unmarshal([]byte(entry.ContentStr), &zot.ZData)
	if err != nil {
		return emperror.Wrapf(err, "cannot unmarshal json\n%s", zot.ZData)
	}
	r := regexp.MustCompile("([^<>]+):([^<>]+)")
	matches := r.FindAllStringSubmatch(zot.ZData.Group.Data.Description, -1)
	for _, match := range matches {
		zot.CollMeta[strings.TrimSpace(strings.ToLower(match[1]))] = strings.TrimSpace(match[2])
	}
	zot.doc = entry.Doc
	return nil
}

func (zot *SourceZoteroDeprecated) Name() string { return "zotero" }

func (zot *SourceZoteroDeprecated) GetSignature() string {
	return fmt.Sprintf("%s-%v.%v", zot.Name(), zot.ZData.Group.Id, zot.ZData.Key)
}

func (zot *SourceZoteroDeprecated) GetContentString() string {
	return zot.contentStr
}

func (zot *SourceZoteroDeprecated) GetContentMime() string {
	return "text/json"
}

func (zot *SourceZoteroDeprecated) GetSolrDoc() *solr.Document {
	return zot.doc
}

func (zot *SourceZoteroDeprecated) GetCollectionTitle() string {
	t, ok := zot.CollMeta["title"]
	if !ok {
		return strings.ReplaceAll(zot.ZData.Group.Data.Name, `_`, ` `)
	}
	return t
}

func (zot *SourceZoteroDeprecated) GetTitle() string {
	return zot.ZData.Data.Title
}

func (zot *SourceZoteroDeprecated) GetSeries() string {
	return zot.ZData.Data.Series
}

func (zot *SourceZoteroDeprecated) GetUrl() string {
	return zot.ZData.Data.Url
}

func (zot *SourceZoteroDeprecated) GetPlace() string {
	return zot.ZData.Data.Place
}

func (zot *SourceZoteroDeprecated) GetDate() string {
	return zot.ZData.Data.Date
}

func (zot *SourceZoteroDeprecated) GetMeta() *Metalist {
	var result = Metalist{}
	s := reflect.ValueOf(&zot.ZData.Data).Elem()
	typeOfT := s.Type()
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		fname := typeOfT.Field(i).Name
		if fname == "ItemDataBase" {
			continue
		}
		valstr := strings.TrimSpace(fmt.Sprintf("%v", f.Interface()))
		if valstr != "" {
			result[fname] = valstr
		}
	}
	s = reflect.ValueOf(&zot.ZData.Data.ItemDataBase).Elem()
	typeOfT = s.Type()
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		fname := typeOfT.Field(i).Name
		valstr := strings.TrimSpace(fmt.Sprintf("%v", f.Interface()))
		if valstr != "" {
			result[fname] = valstr
		}
	}
	return &result
}

func (zot *SourceZoteroDeprecated) GetExtra() *Metalist {
	var result = Metalist{}
	ml := zot.GetMeta()
	for key, val := range *ml {
		if InList(zoteroDeprecatedIgnoreMetaFields, key) {
			continue
		}
		result[key] = val
	}
	return &result
}

func (zot *SourceZoteroDeprecated) GetAbstract() string {
	return strings.TrimSpace(zot.ZData.Data.AbstractNote + "\n" + zot.ZData.Data.Extra)
}

func (zot *SourceZoteroDeprecated) GetContentType() string {
	am := strings.TrimSpace(zot.ZData.Data.ArtworkMedium)
	if am != "" {
		return strings.ToLower(am)
	}
	pt := strings.TrimSpace(zot.ZData.Data.PresentationType)
	if pt != "" {
		return strings.ToLower(pt)
	}

	return strings.ToLower(zot.ZData.Data.ItemDataBase.ItemType)
}

func (zot *SourceZoteroDeprecated) GetPersons() []Person {
	var persons []Person
	for _, c := range zot.ZData.Data.ItemDataBase.Creators {
		name := strings.Trim(fmt.Sprintf("%s, %s", c.LastName, c.FirstName), " ,")
		if name != "" {
			persons = append(persons, Person{
				Name: name,
				Role: c.CreatorType,
			})
		}
	}
	return persons
}

func (zot *SourceZoteroDeprecated) getColl(key string) (*ZoteroCollection, error) {
	if key == "" {
		return nil, errors.New("empty collection Key")
	}
	for _, c := range zot.ZData.Data.Collections {
		for _, coll := range zot.ZData.Collections {
			if coll.Key == c {
				return &coll, nil
			}
		}
	}
	return nil, fmt.Errorf("collection %s not found", key)
}

func (zot *SourceZoteroDeprecated) GetTags() []string {
	var tags []string
	for _, t := range zot.ZData.Data.Tags {
		// ignore variables (i.e. <Name>:<value>
		if !zoteroDeprecatedTagVariable.MatchString(t.Tag) {
			tags = AppendIfMissing(tags, strings.ToLower(t.Tag))
		}
	}
	tags = AppendIfMissing(tags, strings.ToLower(zot.ZData.Group.Data.Name))

	for _, c := range zot.ZData.Data.Collections {
		for _, coll := range zot.ZData.Collections {
			if coll.Key == c {
				tags = AppendIfMissing(tags, strings.ToLower(coll.Data.Name))
				for ok := true; ok; ok = (coll.Data.ParentCollection == "") {
					coll, err := zot.getColl(string(coll.Data.ParentCollection))
					if err != nil {
						break
					}
					tags = AppendIfMissing(tags, strings.ToLower(coll.Data.Name))
				}
			}
		}
	}

	return tags
}

func (zot *SourceZoteroDeprecated) GetChildren(itemType, linkMode string) []ZoteroData {
	var children []ZoteroData
	for _, child := range zot.ZData.Children {
		if child.Data.ItemType != itemType {
			continue
		}
		if linkMode != "" && child.Data.LinkMode != linkMode {
			continue
		}
		children = append(children, child)
	}
	return children
}

func (zot *SourceZoteroDeprecated) GetNotes() []Note {
	var notes []Note
	if zot.ZData.Data.Note != "" {
		notes = append(notes, Note{
			Title: "",
			Note:  template.HTML(zot.ZData.Data.Note),
		})
	}
	for _, child := range zot.GetChildren("note", "") {
		note := strings.Trim(child.Data.Note, " ")
		if note == "" {
			continue
		}

		title := strings.TrimSpace(child.Data.Title)
		if title == "" {
			title = "Note"
		}
		notes = append(notes, Note{
			Title: title,
			Note:  template.HTML(note),
		})
	}
	return notes
}

var zoteroDeprecatedlinkregexp = regexp.MustCompile("^https?://zotero.org/Groups/([^/]+)/items/([^/]+)$")

func (zot *SourceZoteroDeprecated) GetReferences() []Reference {
	var references []Reference
	for key, values := range zot.ZData.Data.ItemDataBase.Relations {
		for _, value := range values {
			if matches := zoteroDeprecatedlinkregexp.FindStringSubmatch(value); matches != nil {
				signature := fmt.Sprintf("zotero-%s.%s", matches[1], matches[2])
				references = append(references, Reference{
					Type:      key,
					Signature: signature,
				})
			}
		}
	}
	return references
}

func (zot *SourceZoteroDeprecated) GetMedia(ms mediaserver.Mediaserver) map[string]MediaList {
	if zot.medias != nil {
		return zot.medias
	}
	zot.medias = make(map[string]MediaList)
	var types []string
	for _, child := range zot.GetChildren("attachment", "") {
		meta := child.Data.Media.Metadata
		t := strings.ToLower(meta.Type)
		// empty type == no media
		if t == "" {
			if strings.HasSuffix(child.Data.Url, ".mp4") {
				t = "video"
				meta.Mimetype = "video/mp4"
			} else {
				continue
			}
		}
		// if type not in list create it
		if _, ok := zot.medias[t]; !ok {
			zot.medias[t] = MediaList{}
			types = append(types, t)
		}
		zot.medias[t] = append(zot.medias[t], Media{
			Name:     child.Data.Title,
			Mimetype: meta.Mimetype,
			Type:     t,
			Uri:      child.Data.Url,
			Width:    int64(meta.Width),
			Height:   int64(meta.Height),
			Duration: int64(meta.Duration),
		})
	}
	// sort medias according to their Name
	for _, t := range types {
		sort.Sort(zot.medias[t])
	}
	return zot.medias
}

func (zot *SourceZoteroDeprecated) GetPoster(ms mediaserver.Mediaserver) *Media {
	medias := zot.GetMedia(ms)
	images, ok := medias["image"]
	if !ok {
		return nil
	}
	if images.Len() == 0 {
		return nil
	}
	return &images[0]
}

func (zot *SourceZoteroDeprecated) GetQueries() []Query {
	queries := []Query{}
	catBase := `fhnw!!hgk!!pub`
	catGroup := catBase + `!!` + zot.ZData.Group.Data.Name
	for _, coll := range zot.ZData.Collections {
		parents := coll.GetParents()
		for i := len(parents); i > 0; i-- {
			elements := parents[0:i]
			queries = append(queries, Query{
				Label:  fmt.Sprintf("%s - %s", zot.ZData.Group.Data.Name, strings.Join(elements, ` - `)),
				Search: fmt.Sprintf("cat:\"%d!!%s!!%s\"", 3+len(elements), catGroup, strings.Join(elements, `!!`)),
			})
		}
	}
	queries = append(queries, Query{
		Label:  fmt.Sprintf("%s", zot.ZData.Group.Data.Name),
		Search: fmt.Sprintf("cat:\"%d!!%s\"", 3, catGroup),
	})
	if zot.ZData.Data.ArchiveLocation != "" {
		queries = append(queries, Query{
			Label:  "Group",
			Search: zot.ZData.Data.ArchiveLocation,
		})
	}
	return queries
}
