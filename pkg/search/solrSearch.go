package search

import (
	"context"
	"fmt"
	"github.com/dgraph-io/badger/v2"
	"github.com/goph/emperror"
	"github.com/je4/zsearch/pkg/mediaserver"
	"github.com/op/go-logging"
)

type MTSOLRSearch struct {
	ss    *MTSolr
	index string
	log   *logging.Logger
}

func NewMTSOLRSearch(urls []string, index string, db *badger.DB, log *logging.Logger) (*MTSOLRSearch, error) {
	if len(urls) < 1 {
		return nil, fmt.Errorf("no url's")
	}
	ss, err := NewMTSolr(urls[0], index, log)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot create solr interface for %v", urls)
	}
	mts := &MTSOLRSearch{
		ss:    ss,
		index: index,
		log:   log,
	}
	return mts, nil
}

func (mte *MTSOLRSearch) Update(source Source, ms mediaserver.Mediaserver) error {
	panic("implement me")
}

func (mte *MTSOLRSearch) LoadDocs(ids []string, ctx context.Context) (map[string]*SourceData, error) {
	docs, err := mte.ss.LoadEntities(ids)
	if err != nil {
		return nil, emperror.Wrap(err, "cannot load docs")
	}
	var result = make(map[string]*SourceData)
	for id, doc := range docs {
		src := doc.Content
		src.Signature = id
		src.ACL = make(map[string][]string)
		for key, val := range doc.ACL {
			src.ACL[key] = val
		}
		result[id] = src
	}
	return result, nil
}

func solrField2String(fields map[string][]string, general []string) string {
	qstr := ""
	for fld, vals := range fields {
		for _, val := range vals {
			val := EscapeSolrString(val)
			if qstr != "" {
				qstr += " AND "
			}
			qstr += fmt.Sprintf("(%s:%s)", fld, val)
		}
	}
	if len(general) > 0 {
		qstr2 := fmt.Sprintf("%s OR %s OR %s OR %s OR %s OR %s OR %s",
			solrOr("title", general, 10, 10),
			solrOr("author", general, 10, 10),
			solrOr("publisher", general, 8, 10),
			solrOr("content", general, 0, 6),
			solrOr("abstract", general, 0, 8),
			solrOr("signature", general, 20, 10),
			solrOr("cluster", general, 5, 2),
		)
		if qstr != "" {
			qstr += " AND "
		}
		qstr += fmt.Sprintf("(%s)", qstr2)
	}
	return qstr
}

func (mte *MTSOLRSearch) Search(cfg *SearchConfig) ([]*SourceData, int64, FacetCountResult, error) {
	// todo: query string wrong
	qstr := solrField2String(cfg.fields, []string{cfg.qstr})
	filter := solrField2String(cfg.filters_fields, []string{cfg.qstr})
	docs, num, fts, err := mte.ss.Search(qstr, []string{filter}, cfg.facets, cfg.groups, cfg.contentVisible, cfg.start, cfg.rows, cfg.isAdmin)
	if err != nil {
		return nil, 0, nil, emperror.Wrap(err, "cannot search docs")
	}
	var result []*SourceData
	for _, doc := range docs {
		src := doc.Content
		src.Signature = doc.Id
		src.ACL = make(map[string][]string)
		for key, val := range doc.ACL {
			src.ACL[key] = val
		}
		result = append(result, src)
	}
	return result, num, fts, nil
}
