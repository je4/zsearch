package source

import (
	"encoding/json"
	"fmt"
	"github.com/goph/emperror"
	"regexp"
	"sort"
	"strings"
)

type Zotero struct {
	zdata    ZoteroData
	collMeta map[string]string
}

func NewZotero(data string) (*Zotero, error) {
	zot := &Zotero{zdata: ZoteroData{}, collMeta: map[string]string{}}
	return zot, zot.Init(data)
}

func (zot *Zotero) Init(data string) error {
	err := json.Unmarshal([]byte(data), &zot.zdata)
	if err != nil {
		return emperror.Wrapf(err, "cannot unmarshal json\n%s", data)
	}
	r := regexp.MustCompile("([^<>]+):([^<>]+)")
	matches := r.FindAllStringSubmatch(zot.zdata.Group.Data.Description, -1)
	for _, match := range matches {
		zot.collMeta[strings.TrimSpace(strings.ToLower(match[1]))] = strings.TrimSpace(match[2])
	}
	return nil
}

func (zot *Zotero) Name() string { return "zotero" }

func (zot *Zotero) GetCollectionTitle() string {
	t, ok := zot.collMeta["title"]
	if !ok {
		return ""
	}
	return t
}

func (zot *Zotero) GetTitle() string {
	return zot.zdata.Data.Title
}

func (zot *Zotero) GetAbstract() string {
	return zot.zdata.Data.AbstractNote
}

func (zot *Zotero) GetNames() []Person {
	var persons []Person
	for _, c := range zot.zdata.Data.ItemDataBase.Creators {
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

func (zot *Zotero) GetTags() []string {
	var tags []string
	for _, t := range zot.zdata.Data.Tags {
		tags = append(tags, t.Tag)
	}
	tags = append(tags, zot.zdata.Group.Data.Name)
	return tags
}

func (zot *Zotero) GetChildren(itemType, linkMode string) []ZoteroData {
	var children []ZoteroData
	for _, child := range zot.zdata.Children {
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

func (zot *Zotero) GetNotes() []string {
	var notes []string
	for _, child := range zot.GetChildren("note", "") {
		note := strings.Trim(child.Data.Note, " ")
		if note == "" {
			continue
		}
		notes = append(notes, note)
	}
	return notes
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
