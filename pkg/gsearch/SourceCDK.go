package gsearch

import (
	"encoding/json"
	"github.com/goph/emperror"
	"github.com/vanng822/go-solr/solr"
)

type SourceCDK struct {
	mts        *MTSolr
	CDKData    SourceCDKData  `json:"CDKData"`
	doc        *solr.Document `json:"-"`
	contentStr string
	medias     map[string]MediaList `json:"-"`
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

func (cdk *SourceCDK) GetPlace() string {
	return "Basel"
}

func (cdk *SourceCDK) GetDate() string {
	return "cdk.DData.Year"
}

func (cdk *SourceCDK) GetMeta() map[string]string {
	var result = make(map[string]string)
	return result
}

func (cdk *SourceCDK) GetAbstract() string {
	return "strings.TrimSpace(cdk.DData.Beschreibung)"
}

func (cdk *SourceCDK) GetType() string {
	return "project"
}

func (cdk *SourceCDK) GetNames() []Person {
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

func (cdk *SourceCDK) GetMedia() map[string]MediaList {
	if cdk.medias != nil {
		return cdk.medias
	}
	cdk.medias = make(map[string]MediaList)
	return cdk.medias
}

func (cdk *SourceCDK) GetPoster() *Media {
	medias := cdk.GetMedia()
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
