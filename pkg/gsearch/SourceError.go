package gsearch

import (
	"fmt"
	"github.com/vanng822/go-solr/solr"
	"time"
)

type SourceError struct {
	mts *MTSolr
	err error
}

func NewSourceError(err error, mts *MTSolr) (*SourceError, error) {

	serr := &SourceError{
		mts: mts,
		err: err,
	}
	return serr, nil
}

func (serr *SourceError) Name() string { return "error" }

func (serr *SourceError) GetSolrDoc() *solr.Document {
	return nil
}

func (serr *SourceError) GetNotes() []Note {
	var notes []Note

	return notes
}

func (serr *SourceError) GetCollectionTitle() string {
	return "Error"
}

func (serr *SourceError) GetTitle() string {
	return fmt.Sprintf("%v", serr.err)
}

func (serr *SourceError) GetPlace() string {
	return "Error"
}

func (serr *SourceError) GetDate() string {
	return time.Now().String()
}

func (serr *SourceError) GetMeta() map[string]string {
	var result = make(map[string]string)
	return result
}

func (serr *SourceError) GetAbstract() string {
	return fmt.Sprintf("%+v", serr.err)
}

func (serr *SourceError) GetType() string {
	return "error"
}

func (serr *SourceError) GetNames() []Person {
	persons := []Person{}
	return persons
}

func (serr *SourceError) GetTags() []string {
	tags := []string{"error"}
	return tags
}

func (serr *SourceError) GetReferences() []Reference {
	var references []Reference
	return references
}

func (serr *SourceError) GetMedia() map[string]MediaList {
	var medias = make(map[string]MediaList)
	return medias
}

func (serr *SourceError) GetQueries() []Query {
	queries := []Query{}
	return queries
}
