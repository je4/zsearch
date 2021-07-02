package search

import (
	"encoding/json"
	"fmt"
	"github.com/goph/emperror"
	"github.com/je4/zsearch/v2/pkg/mediaserver"
	"github.com/vanng822/go-solr/solr"
	"sort"
	"strings"
	"time"
)

type SourceDiplomHGK struct {
	mts        *MTSolr
	DData      SourceDiplomHGKData `json:"DData"`
	doc        *solr.Document      `json:"-"`
	contentStr string
	medias     map[string]MediaList `json:"-"`
}

func (dhgk *SourceDiplomHGK) GetDateAdded() time.Time {
	return time.Now()
}

func (dhgk *SourceDiplomHGK) GetACL() map[string][]string {
	panic("implement me")
}

func (dhgk *SourceDiplomHGK) GetCatalogs() []string {
	panic("implement me")
}

func (dhgk *SourceDiplomHGK) GetCategories() []string {
	panic("implement me")
}

type SourceDiplomHGKFileMeta struct {
	Mimetype string      `json:"mimetype"`
	Type     string      `json:"type"`
	Ext      string      `json:"ext"`
	Width    int64       `json:"width"`
	Height   int64       `json:"height"`
	Duration int64       `json:"duration"`
	Image    interface{} `json:"image"`
}

type SourceDiplomHGKFile struct {
	Year       string                  `json:"year"`
	IDPerson   string                  `json:"idperson"`
	FileID     string                  `json:"fileid"`
	Name       string                  `json:"Name"`
	Filename   string                  `json:"filename"`
	Thumbname  string                  `json:"thumbname"`
	Webname    string                  `json:"webname"`
	Mimetype   string                  `json:"mimetype"`
	Size       string                  `json:"size"`
	Tech       string                  `json:"tech"`
	Uploadtime string                  `json:"uploadtime"`
	Metadata   SourceDiplomHGKFileMeta `json:"metadata"`
	Url        string                  `json:"url"`
}

type ResourceParam struct {
	Type     string `json:"type"`
	Mimetype string `json:"resourcetype"`
}

type Resource struct {
	Src   string        `json:"src"`
	Name  string        `json:"Name"`
	Param ResourceParam `json:"param"`
}

type SourceDiplomHGKData struct {
	Year              string                `json:"year"`
	Anlassnummer      string                `json:"anlassnummer"`
	Anlassbezeichnung string                `json:"anlassbezeichnung"`
	IDPerson          string                `json:"idperson"`
	Anrede            string                `json:"anrede"`
	Vornamen          string                `json:"vornamen"`
	Nachname          string                `json:"nachname"`
	Email2            string                `json"e-mail 2"`
	Done              string                `json:"done"`
	Meta              map[string]string     `json:"meta"`
	References        []string              `json:"references"`
	Videos            []string              `json:"videos"`
	Beschreibung      string                `json:"beschreibung"`
	Betreuer1         string                `json:"betreuer1"`
	Betreuer2         string                `json:"betreuer2"`
	Email             string                `json:"email"`
	Title             string                `json:"title"`
	Untertitel        string                `json:"untertitel"`
	Web1              string                `json:"web1"`
	Web2              string                `json:"web2"`
	Webmedia          string                `json:"webmedia"`
	Abschluss         string                `json:"abschluss"`
	Institut          string                `json:"institut"`
	Categories        []string              `json:"categories"`
	Tags              []string              `json:"tags"`
	Slug              string                `json:"slug"`
	Series            []string              `json:"series"`
	Resources         []Resource            `json:"resources"`
	Files             []SourceDiplomHGKFile `json:"files"`
	Images            []string              `json:"images"`
	Pdfs              []string              `json:"pdfs"`
	Rights            string                `json:"rights"`
}

func NewSourceDiplomHGK(entry *cacheEntry, mts *MTSolr) (*SourceDiplomHGK, error) {

	dhgk := &SourceDiplomHGK{
		mts:        mts,
		DData:      SourceDiplomHGKData{},
		doc:        entry.Doc,
		contentStr: entry.ContentStr,
	}
	return dhgk, dhgk.Init(entry)
}

func (dhgk *SourceDiplomHGK) Init(entry *cacheEntry) error {
	if err := json.Unmarshal([]byte(entry.ContentStr), &dhgk.DData); err != nil {
		return emperror.Wrapf(err, "xcannot unmarshal json\n%s", entry.ContentStr)
	}
	dhgk.doc = entry.Doc
	return nil
}

func (dhgk *SourceDiplomHGK) GetContentString() string {
	return dhgk.contentStr
}

func (dhgk *SourceDiplomHGK) GetContentMime() string {
	return "text/json"
}

func (dhgk *SourceDiplomHGK) Name() string { return "diplomhgk" }

func (dhgk *SourceDiplomHGK) GetSignature() string {
	return fmt.Sprintf("%s-%v.%v", dhgk.Name(), dhgk.DData.IDPerson, dhgk.DData.Slug)
}

func (dhgk *SourceDiplomHGK) GetSolrDoc() *solr.Document {
	return dhgk.doc
}

func (dhgk *SourceDiplomHGK) GetNotes() []Note {
	var notes []Note

	return notes
}

func (dhgk *SourceDiplomHGK) GetCollectionTitle() string {
	return "Diplom HGK"
}

func (dhgk *SourceDiplomHGK) GetTitle() string {
	return dhgk.DData.Title
}

func (dhgk *SourceDiplomHGK) GetUrl() string {
	return ""
}

func (dhgk *SourceDiplomHGK) GetSeries() string {
	return strings.Join(dhgk.DData.Series, "/")
}

func (dhgk *SourceDiplomHGK) GetPlace() string {
	return "Basel"
}

func (dhgk *SourceDiplomHGK) GetDate() string {
	return dhgk.DData.Year
}

func (dhgk *SourceDiplomHGK) GetMeta() *Metalist {
	var result = Metalist{}
	return &result
}

func (dhgk *SourceDiplomHGK) GetExtra() *Metalist {
	var result = Metalist{}
	return &result
}

func (dhgk *SourceDiplomHGK) GetVars() *Varlist {
	return &Varlist{}
}

func (dhgk *SourceDiplomHGK) GetAbstract() string {
	return strings.TrimSpace(dhgk.DData.Beschreibung)
}

func (dhgk *SourceDiplomHGK) GetContentType() string {
	return "project"
}

func (dhgk *SourceDiplomHGK) GetPersons() []Person {
	persons := []Person{
		{
			Name: fmt.Sprintf("%s, %s", dhgk.DData.Nachname, dhgk.DData.Vornamen),
			Role: "author",
		},
	}
	if dhgk.DData.Betreuer1 != "" {
		persons = append(persons, Person{
			Name: dhgk.DData.Betreuer1,
			Role: "advisor",
		},
		)
	}
	if dhgk.DData.Betreuer2 != "" {
		persons = append(persons, Person{
			Name: dhgk.DData.Betreuer2,
			Role: "advisor",
		},
		)
	}
	return persons
}

func (dhgk *SourceDiplomHGK) GetTags() []string {
	tags := []string{"diplomhgk"}

	for _, t := range dhgk.DData.Categories {
		tags = AppendIfMissing(tags, strings.ToLower(t))
	}
	for _, t := range dhgk.DData.Tags {
		tags = AppendIfMissing(tags, strings.ToLower(t))
	}
	for _, t := range dhgk.DData.Series {
		tags = AppendIfMissing(tags, strings.ToLower(t))
	}
	tags = AppendIfMissing(tags, strings.ToLower(dhgk.DData.Institut))

	return tags
}

func (dhgk *SourceDiplomHGK) GetReferences() []Reference {
	var references []Reference
	return references
}

func (dhgk *SourceDiplomHGK) GetMedia(ms mediaserver.Mediaserver) map[string]MediaList {
	if dhgk.medias != nil {
		return dhgk.medias
	}
	dhgk.medias = make(map[string]MediaList)
	var types []string
	for _, file := range dhgk.DData.Files {
		t := strings.ToLower(file.Metadata.Type)
		// empty type == no media
		if t == "" {
			continue
		}
		// if type not in list create it
		if _, ok := dhgk.medias[t]; !ok {
			dhgk.medias[t] = MediaList{}
			types = append(types, t)
		}
		dhgk.medias[t] = append(dhgk.medias[t], Media{
			Name:     file.Name,
			Mimetype: file.Metadata.Mimetype,
			Type:     t,
			Uri:      file.Url,
			Width:    file.Metadata.Width,
			Height:   file.Metadata.Height,
			Duration: file.Metadata.Duration,
		})
	}
	// sort medias according to their Name
	for _, t := range types {
		sort.Sort(dhgk.medias[t])
	}
	return dhgk.medias
}

func (dhgk *SourceDiplomHGK) GetPoster(ms mediaserver.Mediaserver) *Media {
	medias := dhgk.GetMedia(ms)
	images, ok := medias["image"]
	if !ok {
		return nil
	}
	if images.Len() == 0 {
		return nil
	}
	return &images[0]
}

func (dhgk *SourceDiplomHGK) GetQueries() []Query {
	queries := []Query{}
	/*
		catBase := `fhnw!!hgk!!pub`
		catGroup := catBase + `!!` + zot.ZData.Group.Data.Name
		for _, coll := range zot.ZData.Collections {
			parents := coll.GetParents()
			for i := len(parents); i > 0; i-- {
				elements := parents[0:i]
				queries = append(queries, Query{
					Label:  fmt.Sprintf("%s - %s", zot.ZData.Group.Data.Name, strings.Join(elements, ` - `)),
					Search: fmt.Sprintf("%d!!%s!!%s", 3+len(elements), catGroup, strings.Join(elements, `!!`)),
				})
			}
		}
		queries = append(queries, Query{
			Label:  fmt.Sprintf("%s", zot.ZData.Group.Data.Name),
			Search: fmt.Sprintf("%d!!%s", 3, catGroup),
		})

	*/
	return queries
}
