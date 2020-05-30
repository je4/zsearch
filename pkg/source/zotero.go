package source

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/goph/emperror"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/generic"
	"html/template"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

type Zotero struct {
	mts      *MTSolr
	ZData    ZoteroData        `json:"ZData"`
	CollMeta map[string]string `json:"collmeta"`
}

var zoteroIgnoreMetaFields = []string{
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
}

func NewZotero(data string, mts *MTSolr) (*Zotero, error) {
	zot := &Zotero{
		ZData:   ZoteroData{},
		CollMeta: map[string]string{},
		mts:     mts,
	}
	return zot, zot.Init(data)
}

func (zot *Zotero) Init(data string) error {
	err := json.Unmarshal([]byte(data), &zot.ZData)
	if err != nil {
		return emperror.Wrapf(err, "cannot unmarshal json\n%s", data)
	}
	r := regexp.MustCompile("([^<>]+):([^<>]+)")
	matches := r.FindAllStringSubmatch(zot.ZData.Group.Data.Description, -1)
	for _, match := range matches {
		zot.CollMeta[strings.TrimSpace(strings.ToLower(match[1]))] = strings.TrimSpace(match[2])
	}
	return nil
}

func (zot *Zotero) Name() string { return "zotero" }

func (zot *Zotero) GetCollectionTitle() string {
	t, ok := zot.CollMeta["title"]
	if !ok {
		return ""
	}
	return t
}

func (zot *Zotero) GetTitle() string {
	return zot.ZData.Data.Title
}

func (zot *Zotero) GetPlace() string {
	return zot.ZData.Data.Place
}

func (zot *Zotero) GetDate() string {
	return zot.ZData.Data.Date
}

func (zot *Zotero) GetMeta() map[string]string {
	var result = make(map[string]string)
	s := reflect.ValueOf(&zot.ZData.Data).Elem()
	typeOfT := s.Type()
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		fname := typeOfT.Field(i).Name
		if generic.InList(zoteroIgnoreMetaFields, fname) {
			continue
		}
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
		if generic.InList(zoteroIgnoreMetaFields, fname) {
			continue
		}
		valstr := strings.TrimSpace(fmt.Sprintf("%v", f.Interface()))
		if valstr != "" {
			result[fname] = valstr
		}
	}
	return result
}

func (zot *Zotero) GetAbstract() string {
	return strings.TrimSpace(zot.ZData.Data.AbstractNote + "\n" + zot.ZData.Data.Extra)
}

func (zot *Zotero) GetType() string {
	return zot.ZData.Data.ItemDataBase.ItemType
}

func (zot *Zotero) GetNames() []Person {
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

func (zot *Zotero) getColl(key string) (*ZoteroCollection, error) {
	if key == "" {
		return nil, errors.New("empty collection key")
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

func (zot *Zotero) GetTags() []string {
	var tags []string
	for _, t := range zot.ZData.Data.Tags {
		tags = generic.AppendIfMissing(tags, strings.ToLower(t.Tag))
	}
	tags = generic.AppendIfMissing(tags, strings.ToLower(zot.ZData.Group.Data.Name))

	for _, c := range zot.ZData.Data.Collections {
		for _, coll := range zot.ZData.Collections {
			if coll.Key == c {
				tags = generic.AppendIfMissing(tags, strings.ToLower(coll.Data.Name))
				for ok := true; ok; ok = (coll.Data.ParentCollection == "") {
					coll, err := zot.getColl(string(coll.Data.ParentCollection))
					if err != nil {
						break
					}
					tags = generic.AppendIfMissing(tags, strings.ToLower(coll.Data.Name))
				}
			}
		}
	}

	return tags
}

func (zot *Zotero) GetChildren(itemType, linkMode string) []ZoteroData {
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

func (zot *Zotero) GetNotes() []Note {
	var notes []Note
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

var zoterolinkregexp = regexp.MustCompile("^https?://zotero.org/groups/([^/]+)/items/([^/]+)$")

func (zot *Zotero) GetReferences() []Reference {
	var references []Reference
	for key, values := range zot.ZData.Data.ItemDataBase.Relations {
		for _, value := range values {
			if matches := zoterolinkregexp.FindStringSubmatch(value); matches != nil {
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

func (zot *Zotero) GetMedia() map[string]MediaList {
	var medias = make(map[string]MediaList)
	var types []string
	for _, child := range zot.GetChildren("attachment", "") {
		meta := child.Data.Media.Metadata
		t := meta.Type
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
		if _, ok := medias[t]; !ok {
			medias[t] = MediaList{}
			types = append(types, t)
		}
		medias[t] = append(medias[t], Media{
			Name:     child.Data.Title,
			Mimetype: meta.Mimetype,
			Type:     t,
			Uri:      child.Data.Url,
			Width:    int64(meta.Width),
			Height:   int64(meta.Height),
			Duration: int64(meta.Duration),
		})
	}
	// sort medias according to their name
	for _, t := range types {
		sort.Sort(medias[t])
	}
	return medias
}
