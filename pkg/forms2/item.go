package forms2

import (
	"fmt"
	"github.com/je4/zsearch/v2/pkg/mediaserver"
	"github.com/je4/zsearch/v2/pkg/search"
	"github.com/vanng822/go-solr/solr"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type File struct {
	Name     string
	Filename string
}

type Item struct {
	Year              int64
	PersonName        string
	IDPerson          string
	Anlassnummer      string
	Anlassbezeichnung string
	Data              map[string]string
	File              map[int64]*File
	ms                mediaserver.Mediaserver
}

var excludeExta = []string{
	"titel",
	"beschreibung",
	"email",
	"untertitel",
	"web1",
	"rights",
	"tags",
	"anlassbezeichnung",
	"anlassnummer",
}

var studiengang = map[string]string{
	"2-13HS.G-B-In3-1WSRaum.EN": "Modul Erinnerungsraum",
	"2-14HS.G-B-In3-1WSRaum.EN": "Modul Erinnerungsraum",
	"2-15HS.G-B-In3-1WSRaum.EN": "Modul Erinnerungsraum",
	"2-16HS.G-B-In3-1WSRaum.EN": "Modul Erinnerungsraum",
	"2-17HS.G-B-In3-1WSRaum.EN": "Modul Erinnerungsraum",
	"2-18HS.G-B-In3-1WSRaum.EN": "Modul Erinnerungsraum",
	"2-19HS.G-B-In3-1WSRaum.EN": "Modul Erinnerungsraum",
	"2-20HS.G-B-In3-1WSRaum.EN": "Modul Erinnerungsraum",
	"2-G-B-BIKU":                "Kunst",
	"2-G-B-H":                   "Hyperwerk",
	"2-G-B-ID":                  "Industrial Design",
	"2-G-B-In3":                 "Innenarchitektur und Szenografie",
	"2-G-B-KuK":                 "Mode-Design",
	"2-G-B-lgk":                 "Lehrberufe für Gestaltung und Kunst",
	"2-G-B-vico":                "Visuelle Kommunikation",
	"2-G-M-BIKU":                "Master Fine Arts",
	"2-G-M-lgk":                 "Master of Arts in Vermittlung von Kunst und Design",
	"2-G-M-MSD":                 "Master of Arts in Design: Masterstudio Design",
	"2-G-M-vico":                "Master of Art in Visueller Kommunikation und Bildforschung",
	"2-G-M-viwb":                "International Master of Design UIC / HGK",
	"2-G-MAS-UIC":               "MAS Graphic Design I",
}

var institut = map[string]string{
	"2-13HS.G-B-In3-1WSRaum.EN": "Innenarchitektur und Szenografie",
	"2-14HS.G-B-In3-1WSRaum.EN": "Innenarchitektur und Szenografie",
	"2-15HS.G-B-In3-1WSRaum.EN": "Innenarchitektur und Szenografie",
	"2-16HS.G-B-In3-1WSRaum.EN": "Innenarchitektur und Szenografie",
	"2-17HS.G-B-In3-1WSRaum.EN": "Innenarchitektur und Szenografie",
	"2-18HS.G-B-In3-1WSRaum.EN": "Innenarchitektur und Szenografie",
	"2-19HS.G-B-In3-1WSRaum.EN": "Innenarchitektur und Szenografie",
	"2-20HS.G-B-In3-1WSRaum.EN": "Innenarchitektur und Szenografie",
	"2-G-B-BIKU":                "Kunst",
	"2-G-B-H":                   "Hyperwerk",
	"2-G-B-ID":                  "Industrial Design",
	"2-G-B-In3":                 "Innenarchitektur und Szenografie",
	"2-G-B-KuK":                 "Mode",
	"2-G-B-lgk":                 "Lehrberufe für Gestaltung und Kunst",
	"2-G-B-vico":                "Visuelle Kommunikation",
	"2-G-M-BIKU":                "Kunst",
	"2-G-M-lgk":                 "Lehrberufe für Gestaltung und Kunst",
	"2-G-M-MSD":                 "Masterstudio",
	"2-G-M-vico":                "Visuelle Kommunikation",
	"2-G-M-viwb":                "Visuelle Kommunikation",
	"2-G-MAS-UIC":               "Visuelle Kommunikation",
}

func (item *Item) Name() string {
	return "forms2"
}

func (item *Item) GetSource() string {
	return fmt.Sprintf("%s-%v", item.Name(), item.Year)
}

func (item *Item) GetSignature() string {
	return fmt.Sprintf("%s-%v.%v", item.Name(), item.Year, item.IDPerson)
}

func (item *Item) GetSignatureOriginal() string {
	return fmt.Sprintf("%v.%v", item.Year, item.IDPerson)
}

func (item *Item) GetTitle() string {
	title := item.Data["titel"]
	subtitle := strings.TrimSpace(item.Data["untertitel"])
	if subtitle != "" {
		title += ". " + subtitle
	}
	return title
}

func (item *Item) GetSeries() string {
	return ""
}

func (item *Item) GetUrl() string {
	return item.Data["web1"]
}

func (item *Item) GetPlace() string {
	return "Basel"

}

func (item *Item) GetDate() string {
	if item.Year < 10000 {
		return fmt.Sprintf("%v", item.Year)
	} else {
		return fmt.Sprintf("%v", item.Year-10000)
	}
}

func (item *Item) GetDateAdded() time.Time {
	return time.Now()
}

func (item *Item) GetCollectionTitle() string {
	if item.Year < 10000 {
		return fmt.Sprintf("Diplom HGK")
	} else {
		return fmt.Sprintf("Erinnerungsraum")
	}
}

func (item *Item) GetPublisher() string {
	return "Mediathek HGK"
}

func (item *Item) GetPersons() []search.Person {
	var persons []search.Person
	persons = append(persons, search.Person{
		Name: item.PersonName,
		Role: "author",
	})
	return persons
}

// Name:value
var zoteroTagACLVariable = regexp.MustCompile(`^acl_(meta|content|preview):(.+)$`)

func (item *Item) GetACL() map[string][]string {

	var acls = map[string][]string{
		"meta": {"global/guest"},
	}
	if item.Data["rights"] == "off" {
		acls["content"] = []string{"global/admin"}
	} else {
		acls["content"] = []string{"global/guest"}
	}
	return acls
}

func (item *Item) GetCatalogs() []string {
	var catalogs = []string{
		"mediathek",
		"diplom",
		fmt.Sprintf("diplom%v", item.Year),
	}
	return catalogs
}

func (item *Item) GetCategories() []string {
	var categories = []string{}
	anss := strings.Split(item.Anlassnummer, "/")
	if len(anss) == 0 {
		return categories
	}
	inst := institut[anss[0]]
	stud := studiengang[anss[0]]
	if item.Year < 10000 {
		categories = append(categories, fmt.Sprintf("%s!!diplom", item.Name()))
		categories = append(categories, fmt.Sprintf("%s!!diplom!!%v", item.Name(), item.Year))
		categories = append(categories, fmt.Sprintf("%s!!diplom!!%v", item.Name(), inst))
		categories = append(categories, fmt.Sprintf("%s!!diplom!!%v", item.Name(), stud))
		categories = append(categories, fmt.Sprintf("%s!!diplom!!%v!!%v", item.Name(), stud, item.Year))
	} else {
		categories = append(categories, fmt.Sprintf("%s!!modul", item.Name()))
		categories = append(categories, fmt.Sprintf("%s!!modul!!%v", item.Name(), item.Year-10000))
		categories = append(categories, fmt.Sprintf("%s!!modul!!%v", item.Name(), inst))
		categories = append(categories, fmt.Sprintf("%s!!modul!!%v", item.Name(), stud))
		categories = append(categories, fmt.Sprintf("%s!!modul!!%v!!%v", item.Name(), stud, item.Year-10000))
	}
	return categories
}

func (item *Item) GetTags() []string {
	var tags = []string{}
	anss := strings.Split(item.Anlassnummer, "/")
	if len(anss) == 0 {
		return tags
	}
	inst := institut[anss[0]]
	tags = search.AppendIfMissing(tags, inst)
	stud := studiengang[anss[0]]
	tags = search.AppendIfMissing(tags, stud)
	if item.Year < 10000 {
		tags = search.AppendIfMissing(tags, fmt.Sprintf("%v", item.Year))
		tags = search.AppendIfMissing(tags, "Diplom")
		tags = search.AppendIfMissing(tags, "Ausstellung")
	} else {
		tags = search.AppendIfMissing(tags, fmt.Sprintf("%v", item.Year-10000))
		tags = search.AppendIfMissing(tags, "Studienarbeit")
	}
	ts := strings.Split(item.Data["tags"], " ")
	if len(ts) > 0 {
		for _, t := range ts {
			t = strings.Trim(t, "#; ")
			if t != "" {
				tags = search.AppendIfMissing(tags, t)
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

func (item *Item) GetMedia() map[string]search.MediaList {
	if item.ms == nil {
		return map[string]search.MediaList{}
	}
	medias := make(map[string]search.MediaList)

	for mediaid, media := range item.File {
		var collection, signature string

		// check for mediaserver url
		var ok bool
		collection, signature, ok = item.ms.IsMediaserverURL(media.Filename)
		if !ok {
			collection = ""
			signature = ""
		}
		if collection != "" && signature != "" {
			metadata, err := item.ms.GetMetadata(collection, signature)
			if err != nil {
				continue
			}
			var fulltext string
			if metadata.Type == "pdf" {
				fulltext, err = item.ms.GetFulltext(collection, signature)
				if err != nil {
					continue
				}
			}
			name := media.Name
			if name == "" {
				name = fmt.Sprintf("#%v.%v.%v", item.Year, item.IDPerson, mediaid)
			}

			media := search.Media{
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
				medias[media.Type] = []search.Media{}
			}

			medias[media.Type] = append(medias[media.Type], media)
		}
	}
	return medias
}

var mediaserverRegexp = regexp.MustCompile("^mediaserver:([^/]+)/(.+)$")

func (item *Item) GetPoster() *search.Media {
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
					return &search.Media{
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
					return &search.Media{
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

func (item *Item) GetNotes() []search.Note {
	notes := []search.Note{}

	return notes
}

func (item *Item) GetAbstract() string {
	return item.Data["beschreibung"]
}

func (item *Item) GetRights() string {
	return "http://rightsstatements.org/vocab/InC/1.0/"
}

var urlRegex = regexp.MustCompile(`(https?:\/\/[^@\/\n]+[^: ;):\n]+[^: ;)?:\n.])`)

func (item *Item) GetLicense() string {
	return ""
}

var zoterolinkregexp = regexp.MustCompile("^https?://zotero.org/groups/([^/]+)/items/([^/]+)$")

func (item *Item) GetReferences() []search.Reference {
	var references []search.Reference
	return references
}

func (item *Item) GetMeta() *search.Metalist {
	var result = search.Metalist{}
	for k, v := range item.Data {
		v = strings.TrimSpace(v)
		if v != "" {
			result[k] = v
		}
	}
	result["anlassnummer"] = item.Anlassnummer
	result["anlassbezeichnung"] = item.Anlassbezeichnung
	return &result
}

func (item *Item) GetExtra() *search.Metalist {
	var result = search.Metalist{}
	for k, v := range *item.GetMeta() {
		exclude := false
		for _, ex := range excludeExta {
			if k == ex {
				exclude = true
				break
			}
		}
		if !exclude {
			result[k] = v
		}
	}
	return &result
}

func (item *Item) GetVars() *search.Varlist {
	ml := search.Varlist{}
	return ml.Unique()
}

func (item *Item) GetContentType() string {
	if item.Year < 10000 {
		return "thesis"
	} else {
		return "artwork"
	}
}

func (item *Item) GetQueries() []search.Query {
	queries := []search.Query{}
	anss := strings.Split(item.Anlassnummer, "/")
	if len(anss) == 0 {
		return queries
	}
	//inst := institut[anss[0]]
	stud := studiengang[anss[0]]

	appendQuery := func(qs []search.Query, newqueries ...search.Query) []search.Query {
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
	if item.Year < 10000 {
		queries = appendQuery(queries,
			search.Query{
				Label:  fmt.Sprintf("%s - %s - %v", title, stud, item.Year),
				Search: fmt.Sprintf(`cat:"%s!!diplom!!%v!!%v"`, item.Name(), stud, item.Year),
			}, search.Query{
				Label:  fmt.Sprintf("%s - %v", title, item.Year),
				Search: fmt.Sprintf(`cat:"%s!!%v!!%v"`, item.Name(), "diplom", item.Year),
			}, search.Query{
				Label:  fmt.Sprintf("%s", title),
				Search: fmt.Sprintf(`cat:"%s!!%v"`, item.Name(), "diplom"),
			})
	} else {
		queries = appendQuery(queries,
			search.Query{
				Label:  fmt.Sprintf("%s - %s - %v", title, stud, item.Year-10000),
				Search: fmt.Sprintf(`cat:"%s!!modul!!%v!!%v"`, item.Name(), stud, item.Year-10000),
			}, search.Query{
				Label:  fmt.Sprintf("%s - %v", title, item.Year-10000),
				Search: fmt.Sprintf(`cat:"%s!!%v!!%v"`, item.Name(), "modul", item.Year-10000),
			}, search.Query{
				Label:  fmt.Sprintf("%s", title),
				Search: fmt.Sprintf(`cat:"%s!!%v"`, item.Name(), "modul"),
			})
	}
	return queries
}

func (item *Item) GetSolrDoc() *solr.Document {
	return nil
}

func (item *Item) GetContentString() string {
	return ""

}

func (item *Item) GetContentMime() string {
	return ""

}
