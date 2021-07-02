package search

import (
	"encoding/json"
	"fmt"
	"github.com/goph/emperror"
	"github.com/je4/zsearch/v2/pkg/mediaserver"
	"github.com/vanng822/go-solr/solr"
	"time"
)

type SourceCDK struct {
	mts        *MTSolr
	CDKData    SourceCDKData  `json:"CDKData"`
	doc        *solr.Document `json:"-"`
	contentStr string
	medias     map[string]MediaList `json:"-"`
}

func (cdk *SourceCDK) GetDateAdded() time.Time {
	return time.Now()
}

func (cdk *SourceCDK) GetACL() map[string][]string {
	panic("implement me")
}

func (cdk *SourceCDK) GetCatalogs() []string {
	panic("implement me")
}

func (cdk *SourceCDK) GetCategories() []string {
	panic("implement me")
}

type SourceCDKData struct {
}

func NewSourceCDK(entry *cacheEntry, mts *MTSolr) (*SourceCDK, error) {

	cdk := &SourceCDK{
		mts:        mts,
		doc:        entry.Doc,
		contentStr: entry.ContentStr,
	}
	return cdk, cdk.Init(entry)
}

func (cdk *SourceCDK) Init(entry *cacheEntry) error {
	if err := json.Unmarshal([]byte(entry.ContentStr), &cdk.CDKData); err != nil {
		return emperror.Wrapf(err, "cannot unmarshal json\n%s", entry.ContentStr)
	}
	cdk.doc = entry.Doc
	return nil
}

func (cdk *SourceCDK) GetContentString() string {
	return cdk.contentStr
}

func (cdk *SourceCDK) GetContentMime() string {
	return "text/json"
}

func (cdk *SourceCDK) Name() string { return "diplomhgk" }

func (cdk *SourceCDK) GetSignature() string {
	return fmt.Sprintf("%s-%v.%v", cdk.Name(), "cdk.DData.ID")
}

func (cdk *SourceCDK) GetSolrDoc() *solr.Document {
	return cdk.doc
}

func (cdk *SourceCDK) GetNotes() []Note {
	var notes []Note

	return notes
}

func (cdk *SourceCDK) GetCollectionTitle() string {
	return "Diplom HGK"
}

func (cdk *SourceCDK) GetTitle() string {
	return "cdk.DData.Title"
}

func (cdk *SourceCDK) GetSeries() string {
	return "cdk.DData.Series"
}

func (cdk *SourceCDK) GetUrl() string {
	return "cdk.DData.Url"
}

func (cdk *SourceCDK) GetPlace() string {
	return "Basel"
}

func (cdk *SourceCDK) GetDate() string {
	return "cdk.DData.Year"
}

func (cdk *SourceCDK) GetMeta() *Metalist {
	var result = make(Metalist)
	return &result
}

func (cdk *SourceCDK) GetExtra() *Metalist {
	var result = make(Metalist)
	return &result
}

func (cdk *SourceCDK) GetVars() *Varlist {
	return &Varlist{}
}

func (cdk *SourceCDK) GetAbstract() string {
	return "strings.TrimSpace(cdk.DData.Beschreibung)"
}

func (cdk *SourceCDK) GetContentType() string {
	return "project"
}

func (cdk *SourceCDK) GetPersons() []Person {
	persons := []Person{}
	return persons
}

func (cdk *SourceCDK) GetTags() []string {
	tags := []string{"diplomhgk"}
	return tags
}

func (cdk *SourceCDK) GetReferences() []Reference {
	var references []Reference
	return references
}

func (cdk *SourceCDK) GetMedia(ms mediaserver.Mediaserver) map[string]MediaList {
	if cdk.medias != nil {
		return cdk.medias
	}
	cdk.medias = make(map[string]MediaList)
	return cdk.medias
}

func (cdk *SourceCDK) GetPoster(ms mediaserver.Mediaserver) *Media {
	medias := cdk.GetMedia(ms)
	images, ok := medias["image"]
	if !ok {
		return nil
	}
	if images.Len() == 0 {
		return nil
	}
	return &images[0]
}

func (cdk *SourceCDK) GetQueries() []Query {
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
