package search

import (
	"fmt"
	"github.com/je4/zsearch/v2/pkg/mediaserver"
	"github.com/je4/zsync/v2/pkg/zotero"
	"github.com/vanng822/go-solr/solr"
	"html/template"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

/* *******************************
Functions of Source interface
******************************* */

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
	"Note",
	"Series",
	"Url",
	"VideoRecordingFormat",

	"ItemType",
	"MTime",
	"Version",
}

type ZoteroItem struct {
	zotero.Item
	ms mediaserver.Mediaserver
}

func NewZoteroItem(zItem zotero.Item, ms mediaserver.Mediaserver) *ZoteroItem {
	item := &ZoteroItem{
		Item: zItem,
		ms:   ms,
	}
	return item
}

func (item *ZoteroItem) GetChildrenLocal() (*[]ZoteroItem, error) {
	zItem := item.Item
	result, err := zItem.GetChildrenLocal()
	if err != nil {
		return nil, err
	}
	returns := []ZoteroItem{}
	for _, i := range *result {
		returns = append(returns, *NewZoteroItem(i, item.ms))
	}
	return &returns, nil
}

func (item *ZoteroItem) Name() string {
	return "zotero2"
}

func (item *ZoteroItem) GetSource() string {
	return fmt.Sprintf("%s-%v", item.Name(), item.Group.Id)
}

func (item *ZoteroItem) GetSignature() string {
	return fmt.Sprintf("%s-%v.%v", item.Name(), item.Group.Id, item.Key)
}

func (item *ZoteroItem) GetSignatureOriginal() string {
	return fmt.Sprintf("%v.%v", item.Group.Id, item.Key)
}

func (item *ZoteroItem) GetTitle() string {
	return item.Data.Title
}

func (item *ZoteroItem) GetSeries() string {
	return item.Data.Series
}

func (item *ZoteroItem) GetUrl() string {
	return item.Data.Url
}

func (item *ZoteroItem) GetPlace() string {
	return item.Data.Place

}

func (item *ZoteroItem) GetDate() string {
	return item.Data.Date

}

func (item *ZoteroItem) GetDateAdded() time.Time {
	d, err := time.Parse("2006-01-02T15:04:05Z", item.Data.ItemDataBase.DateAdded)
	if err != nil {
		return time.Now()
	}
	return d
}

func (item *ZoteroItem) GetCollectionTitle() string {
	meta := zotero.Text2Metadata(item.Group.Data.Description)
	if titles, ok := meta["title"]; ok {
		if len(titles) > 0 {
			return titles[0]
		}
	}
	return item.Group.Data.Name

}

func (item *ZoteroItem) GetPublisher() string {
	p := strings.TrimSpace(item.Data.Publisher)
	if p == "" {
		return "Mediathek HGK FHNW"
	}
	return p
}

func (item *ZoteroItem) GetPersons() []Person {
	var persons []Person
	for _, c := range item.Data.Creators {
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

// Name:value
var zoteroTagACLVariable = regexp.MustCompile(`^acl_(meta|content|preview):(.+)$`)

func (item *ZoteroItem) GetACL() map[string][]string {
	meta := zotero.Text2Metadata(item.Group.Data.Description)
	meta2 := zotero.Text2Metadata(item.Data.AbstractNote)
	first := true
	for key, vals := range meta2 {
		if strings.HasPrefix(key, "acl_") {
			// if we have the first acl clear the acl from library
			if first {
				meta = make(map[string][]string)
				first = false
			}
			if _, ok := meta[key]; !ok {
				meta[key] = []string{}
			}
			meta[key] = append(meta[key], vals...)
		}
	}
	acls := make(map[string][]string)
	for _, t := range item.Data.Tags {
		matches := zoteroTagACLVariable.FindStringSubmatch(t.Tag)
		if matches != nil {
			acltype := matches[1]
			group := matches[2]
			if _, ok := acls[acltype]; !ok {
				acls[acltype] = []string{}
			}
			acls[acltype] = append(acls[acltype], group)
		}
	}

	if len(acls) == 0 || len(meta2) > 0 {
		for key, val := range meta {
			if strings.HasPrefix(key, "acl_") {
				acltype := key[4:] // get rid of acl_
				if _, ok := acls[acltype]; !ok {
					acls[acltype] = []string{}
				}
				for _, a := range val {
					acls[acltype] = append(acls[acltype], strings.TrimSpace(a))
				}
			}
		}
	}

	return acls
}

func (item *ZoteroItem) GetCatalogs() []string {
	meta := zotero.Text2Metadata(item.Group.Data.Description)
	meta2 := zotero.Text2Metadata(item.Data.AbstractNote)
	for key, val := range meta2 {
		meta[key] = val
	}
	catalogs := []string{}
	for key, val := range meta {
		if strings.Index(key, "catalog") == 0 {
			catalogs = append(catalogs, val...)
		}
	}
	return catalogs
}

func (item *ZoteroItem) GetCategories() []string {
	categories := []string{}
	for _, collection := range item.Data.Collections {
		parentColl, err := item.Group.GetCollectionByKeyLocal(collection)
		if err != nil {
			continue
		}
		if parentColl.Data.ParentCollection != "" {
			coll2, err := item.Group.GetCollectionByKeyLocal(string(parentColl.Data.ParentCollection))
			if err != nil {
				break
			}
			categories = append(categories, fmt.Sprintf("%v!!%v!!%v!!%v", item.Name(), item.Group.Data.Name, coll2.Data.Name, parentColl.Data.Name))
		} else {
			categories = append(categories, fmt.Sprintf("%v!!%v!!%v", item.Name(), item.Group.Data.Name, parentColl.Data.Name))
		}

	}
	if len(categories) == 0 {
		categories = append(categories, fmt.Sprintf("%v!!%v", item.Name(), item.Group.Data.Name))
	}
	return categories
}

func (item *ZoteroItem) GetTags() []string {
	var tags []string
	for _, t := range item.Data.Tags {
		// ignore variables (i.e. <Name>:<value>
		if !zoteroTagACLVariable.MatchString(t.Tag) {
			tags = AppendIfMissing(tags, strings.ToLower(t.Tag))
		}
	}
	tags = AppendIfMissing(tags, strings.ToLower(item.Group.Data.Name))

	for _, c := range item.Data.Collections {
		for _, collKey := range item.Data.Collections {
			coll, err := item.Group.GetCollectionByKeyLocal(collKey)
			if err != nil {
				item.Group.Zot.Logger.Errorf("could not load collection #%v.%v", item.Group.Data.Id, collKey)
				continue
			}
			if coll.Key == c {
				tags = AppendIfMissing(tags, strings.ToLower(coll.Data.Name))
				if coll.Data.ParentCollection != "" {
					coll2, err := item.Group.GetCollectionByKeyLocal(string(coll.Data.ParentCollection))
					if err != nil {
						break
					}
					tags = AppendIfMissing(tags, strings.ToLower(coll2.Data.Name))
				}
			}
		}
	}
	return tags
}

// https://mediathek.hgk.fhnw.ch/indexer.ng/media.php?id=4.4419.2211214
// http://hdl.handle.net/20.500.11806/mediathek/inventory/B0000078520/4.4421.2227476

func getOrientation(metadata *mediaserver.Metadata) int64 {
	var orientation int64 = 1
	if metadata.Image != nil {
		if image, ok := metadata.Image.(map[string]interface{}); ok {
			if image["properties"] != nil {
				if props, ok := image["properties"].(map[string]interface{}); ok {
					if props["exif:Orientation"] != nil {
						if oStr, ok := props["exif:Orientation"].(string); ok {
							if oVal, err := strconv.ParseInt(oStr, 10, 64); err == nil {
								orientation = oVal
							}
						}
					}
				}

			}
		}
	}
	return orientation
}

func (item *ZoteroItem) GetMedia() map[string]MediaList {
	if item.ms == nil {
		return map[string]MediaList{}
	}
	medias := make(map[string]MediaList)
	//var types []string
	children, err := item.GetChildrenLocal()
	if err != nil {
		return medias
	}
	for _, child := range *children {
		if child.Data.ItemType != "attachment" {
			continue
		}
		var collection, signature string
		if child.Data.LinkMode == "linked_url" || child.Data.LinkMode == "imported_url" {

			// check for mediaserver url
			var ok bool
			collection, signature, ok = item.ms.IsMediaserverURL(child.Data.Url)
			if !ok {
				collection = ""
				signature = ""
				/*
					// if not, create mediaserver entry
					collection = fmt.Sprintf("zotero_%v", item.Group.Id)
					signature = fmt.Sprintf("%v.%v_url", item.Group.Id, child.Key)
					if err := ms.CreateMasterUrl(collection, signature, child.Data.Link); err != nil {
						item.Group.Zot.Logger.Errorf("cannot create mediaserver entry for item #%v.%s %s/%s",
							item.Group.Id,
							child.Key,
							collection,
							signature)
						continue
					}
				*/
			}
		} else { // not only url
			collection = fmt.Sprintf("zotero_%v", item.Group.Id)
			signature = fmt.Sprintf("%v.%v_enclosure", item.Group.Id, child.Key)
			folder, err := item.Group.GetFolder()
			if err != nil {
				item.Group.Zot.Logger.Errorf("cannot get folder of attachment file: %v", err)
				continue
			}
			filepath := fmt.Sprintf("%s/%s", folder, child.Key)
			found, err := item.Group.Zot.Fs.FileExists(folder, child.Key)
			if err != nil {
				item.Group.Zot.Logger.Errorf("cannot check existence of file %s: %v", filepath, err)
				continue
			}
			if !found {
				item.Group.Zot.Logger.Warningf("file %s does not exist", filepath)
				continue
			}
			url := fmt.Sprintf("%s/%s", item.Group.Zot.Fs.Protocol(), filepath)
			if err := item.ms.CreateMasterUrl(collection, signature, url, true); err != nil {
				item.Group.Zot.Logger.Errorf("cannot create mediaserver entry for item #%s.%s %s/%s",
					item.Group.Id,
					item.Key,
					collection,
					signature)
				continue
			}
		}

		if collection != "" && signature != "" {
			metadata, err := item.ms.GetMetadata(collection, signature)
			if err != nil {
				item.Group.Zot.Logger.Errorf("cannot get metadata for %s/%s", collection, signature)
				continue
			}
			var fulltext string
			if metadata.Type == "pdf" {
				fulltext, err = item.ms.GetFulltext(collection, signature)
				if err != nil {
					item.Group.Zot.Logger.Errorf("cannot get fulltext for %s/%s", collection, signature)
					continue
				}
			}
			name := child.Data.Title
			if name == "" {
				name = fmt.Sprintf("#%v.%v", item.Group.Id, child.Key)
			}

			media := Media{
				Name:        name,
				Mimetype:    metadata.Mimetype,
				Type:        metadata.Type,
				Uri:         fmt.Sprintf("mediaserver:%s/%s", collection, signature),
				Width:       metadata.Width,
				Height:      metadata.Height,
				Orientation: getOrientation(metadata),
				Duration:    metadata.Duration,
				Fulltext:    fulltext,
			}

			if _, ok := medias[media.Type]; !ok {
				medias[media.Type] = []Media{}
			}

			medias[media.Type] = append(medias[media.Type], media)
		}
	}
	return medias
}

var mediaserverRegexp = regexp.MustCompile("^mediaserver:([^/]+)/(.+)$")

func (item *ZoteroItem) GetPoster() *Media {
	if item.ms == nil {
		return nil
	}
	medias := item.GetMedia()
	if _, ok := medias["video"]; ok {
		if len(medias["video"]) > 0 {
			vid := medias["video"][0]
			if matches := mediaserverRegexp.FindStringSubmatch(vid.Uri); matches != nil {
				collection := matches[1]
				signature := fmt.Sprintf("%s$$timeshot$$3", matches[2])
				metadata, err := item.ms.GetMetadata(collection, signature)
				if err == nil {
					return &Media{
						Name:        "poster",
						Mimetype:    metadata.Mimetype,
						Type:        metadata.Type,
						Uri:         fmt.Sprintf("mediaserver:%v/%v", collection, signature),
						Width:       metadata.Width,
						Height:      metadata.Height,
						Orientation: getOrientation(metadata),
						Duration:    metadata.Duration,
					}
				}
			}
		}
	}
	if _, ok := medias["image"]; ok {
		for _, media := range medias["image"] {
			if strings.ToLower(media.Name) == "poster" {
				return &media
			}
		}
		if len(medias["image"]) > 0 {
			return &(medias["image"][0])
		}
	}
	if _, ok := medias["audio"]; ok {
		if len(medias["audio"]) > 0 {
			aud := medias["audio"][0]
			if matches := mediaserverRegexp.FindStringSubmatch(aud.Uri); matches != nil {
				collection := matches[1]
				signature := fmt.Sprintf("%s$$poster", matches[2])
				metadata, err := item.ms.GetMetadata(collection, signature)
				if err == nil {
					return &Media{
						Name:     "poster",
						Mimetype: metadata.Mimetype,
						Type:     metadata.Type,
						Uri:      fmt.Sprintf("mediaserver:%v/%v", collection, signature),
						Width:    metadata.Width,
						Height:   metadata.Height,
						Duration: metadata.Duration,
					}
				}
			}
		}
	}
	return nil
}

func (item *ZoteroItem) GetNotes() []Note {
	notes := []Note{}
	note := strings.TrimSpace(item.Data.Note)
	if note != "" {
		notes = append(notes, Note{
			Title: item.Data.Title,
			Note:  template.HTML(note),
		})
	}

	children, err := item.GetChildrenLocal()
	if err != nil {
		item.Group.Zot.Logger.Errorf("cannot load children of #%v.%v", item.Group.Id, item.Key)
		return notes
	}
	for _, child := range *children {
		note := strings.TrimSpace(child.Data.Note)
		if note == "" {
			continue
		}
		notes = append(notes, Note{
			Title: child.Data.Title,
			Note:  template.HTML(note),
		})
	}

	return notes
}

func (item *ZoteroItem) GetAbstract() string {
	return zotero.TextNoMeta(item.Data.AbstractNote + "\n" + item.Data.Extra)
}

func (item *ZoteroItem) GetRights() string {
	return item.Data.Rights
}

var urlRegex = regexp.MustCompile(`(https?:\/\/[^@\/\n]+[^: ;):\n]+[^: ;)?:\n.])`)

func (item *ZoteroItem) GetLicense() string {
	matches := urlRegex.FindStringSubmatch(item.Data.Rights)
	if matches == nil {
		return ""
	}
	return matches[1]
}

var zoterolinkregexp = regexp.MustCompile("^https?://zotero.org/groups/([^/]+)/items/([^/]+)$")

func (item *ZoteroItem) GetReferences() []Reference {
	var references []Reference
	for key, values := range item.Data.ItemDataBase.Relations {
		for _, value := range values {
			if matches := zoterolinkregexp.FindStringSubmatch(value); matches != nil {
				signature := fmt.Sprintf("%s-%s.%s", item.Name(), matches[1], matches[2])
				references = append(references, Reference{
					Type:      key,
					Signature: signature,
				})
			}
		}
	}
	return references
}

func (item *ZoteroItem) GetMeta() *Metalist {
	var result = Metalist{}
	s := reflect.ValueOf(&item.Data).Elem()
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
	s = reflect.ValueOf(&item.Data.ItemDataBase).Elem()
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

func (item *ZoteroItem) GetExtra() *Metalist {
	var result = Metalist{}
	ml := item.GetMeta()
	for key, val := range *ml {
		if InList(zoteroIgnoreMetaFields, key) {
			continue
		}
		result[key] = val
	}
	return &result
}

func (item *ZoteroItem) GetVars() *Varlist {
	ml := Varlist{}
	meta := zotero.Text2Metadata(item.Group.Data.Description)
	ml.AppendMap(meta)
	meta2 := zotero.Text2Metadata(item.Data.AbstractNote)
	ml.AppendMap(meta2)
	for _, tag := range item.GetTags() {
		meta := zotero.Text2Metadata(tag)
		ml.AppendMap(meta)
	}
	return ml.Unique()
}

func (item *ZoteroItem) GetContentType() string {
	if t := strings.TrimSpace(item.Data.VideoRecordingFormat); t != "" {
		return strings.ToLower(t)
	}
	if t := strings.TrimSpace(item.Data.Genre); t != "" {
		return strings.ToLower(t)
	}
	if t := strings.TrimSpace(item.Data.ArtworkMedium); t != "" {
		return strings.ToLower(t)
	}
	if t := strings.TrimSpace(item.Data.PresentationType); t != "" {
		return strings.ToLower(t)
	}

	return strings.ToLower(item.Data.ItemDataBase.ItemType)
}

func (item *ZoteroItem) GetQueries() []Query {
	queries := []Query{}

	appendQuery := func(qs []Query, newqueries ...Query) []Query {
		for _, newquery := range newqueries {
			found := false
			for _, q := range qs {
				if q.Search == newquery.Search {
					found = true
					break
				}
			}
			if !found {
				qs = append(qs, newquery)
			}
		}
		return qs
	}

	title := item.GetCollectionTitle()
	for _, collection := range item.Data.Collections {
		parentColl, err := item.Group.GetCollectionByKeyLocal(collection)
		if err != nil {
			continue
		}
		if parentColl.Data.ParentCollection != "" {
			subParentColl, err := item.Group.GetCollectionByKeyLocal(string(parentColl.Data.ParentCollection))
			if err != nil {
				break
			}
			queries = appendQuery(queries, Query{
				Label:  fmt.Sprintf("%s - %s - %s", title, subParentColl.Data.Name, parentColl.Data.Name),
				Search: fmt.Sprintf(`cat:"%v!!%v!!%v!!%v"`, item.Name(), item.Group.Data.Name, subParentColl.Data.Name, parentColl.Data.Name),
			}, Query{
				Label:  fmt.Sprintf("%s - %s", title, subParentColl.Data.Name),
				Search: fmt.Sprintf(`cat:"%v!!%v!!%v"`, item.Name(), item.Group.Data.Name, subParentColl.Data.Name),
			})
		} else {
			queries = appendQuery(queries, Query{
				Label:  fmt.Sprintf("%s - %s", title, parentColl.Data.Name),
				Search: fmt.Sprintf(`cat:"%v!!%v!!%v"`, item.Name(), item.Group.Data.Name, parentColl.Data.Name),
			}, Query{
				Label:  fmt.Sprintf("%s", title),
				Search: fmt.Sprintf(`cat:"%v!!%v"`, item.Name(), item.Group.Data.Name),
			})
		}

	}
	queries = appendQuery(queries, Query{
		Label:  fmt.Sprintf("%s", item.Group.Data.Name),
		Search: fmt.Sprintf(`cat:"%v!!%v"`, item.Name(), item.Group.Data.Name),
	})
	if item.Data.ArchiveLocation != "" {
		queries = appendQuery(queries, Query{
			Label:  "Group",
			Search: item.Data.ArchiveLocation,
		})
	}
	for _, catalog := range item.GetCatalogs() {
		queries = appendQuery(queries, Query{
			Label:  fmt.Sprintf("catalog - %s", catalog),
			Search: fmt.Sprintf(`catalog:"%v"`, catalog),
		})
	}
	return queries
}

func (item *ZoteroItem) GetSolrDoc() *solr.Document {
	return nil
}

func (item *ZoteroItem) GetContentString() string {
	return ""

}

func (item *ZoteroItem) GetContentMime() string {
	return ""

}
