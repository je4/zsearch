package search

import (
	"encoding/json"
	"fmt"
	"github.com/dgraph-io/badger"
	elasticsearch8 "github.com/elastic/go-elasticsearch/v8"
	"github.com/goph/emperror"
	"github.com/op/go-logging"
	"github.com/vanng822/go-solr/solr"
	"sync"
	"time"
)

type Search struct {
	db *badger.DB
	sync.Mutex
	log *logging.Logger
	se  SearchEngine
}

func NewSearch(se SearchEngine, expiration time.Duration, cachesize int, db *badger.DB, log *logging.Logger) (*Search, error) {
	s := &Search{
		db:    db,
		Mutex: sync.Mutex{},
		log:   log,
		se:    se,
	}
	return s, nil
}

/*
store SourceData in cache
 */
func (s *Search) storeCache(src *SourceData) error {
	jsonstr, err := json.Marshal(*s)
	if err != nil {
		return emperror.Wrapf(err, "cannot marshal source data of %v", src.Signature)
	}
	data := Compress([]byte(jsonstr))
	if err := s.db.Update(func(txn *badger.Txn) error {
		txn.Set([]byte(src.Signature), data)
		return nil
	}); err != nil {
		return err
	}
	return nil
}

/*
retrieve SourceData from cache
 */
func (s *Search) getFromCache(id string) (*SourceData, error) {
	var result *SourceData
	if err := s.db.View(func(txn *badger.Txn) error {
		it, err := txn.Get([]byte(id))
		if err != nil {
			return emperror.Wrapf(err, "cannot get item %s", id)
		}
		if it == nil {
			return fmt.Errorf("item %s not in cache", id)
		}
		if err := it.Value(func(v []byte) error {
			var doc = &SourceData{}

			// decompress...
			data, err := Decompress(v)
			if err != nil {
				return emperror.Wrapf(err, "cannot deocmpress %s", string(v))
			}
			// ...unmarshal
			if err := json.Unmarshal(data, doc); err != nil {
				return emperror.Wrapf(err, "cannot unmarshal json %s", string(v))
			}
			s.log.Infof("document %s found in cache", id)
			result = doc
			return nil
		}); err != nil {
			return emperror.Wrapf(err, "cannot load item %s", id)
		}
		return nil
	}); err != nil {
		return nil, emperror.Wrap(err, "item not found")
	}
	return result, nil
}

func (s *Search) LoadEntities(ids []string) (map[string]*SourceData, error) {
	// todo: need better locking stragegy
	s.Lock()
	defer s.Unlock()

	var result = make(map[string]*SourceData)
	var toLoad []string

	//
	// try loading from cache
	//
	for _, id := range ids {
		doc, err := s.getFromCache(id)
		if err != nil {
			toLoad = append(toLoad, id)
		} else {
			if doc.Source != "" {
				result[doc.Signature] = doc
			}
		}
	}

	//
	// then load the rest from index
	//
	entries, err := s.se.LoadDocs(toLoad)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot load entities %v", ids)
	}
	// store results in cache
	for _, sdata := range entries {
		_ = s.storeCache(sdata)
	}
	return entries, nil
}

func (s *Search) LoadEntity(id string) (*SourceData, error) {
	entities, err := s.LoadEntities([]string{id})
	if err != nil {
		return nil, err
	}
	e, ok := entities[id]
	if !ok {
		return nil, fmt.Errorf("could not load entity %s", id)
	}
	return e, nil
}

func (s *Search) Search(text string, filters []string, facets map[string]termFacet, groups []string, contentVisible bool, start, rows int, isAdmin bool) ([]*SourceData, int64, FacetCountResult, error) {

	q :=

	//qstr := EscapeSolrString(text)
	qstr := text
	if qstr == "" {
		qstr = "*:*"
	}
	query := solr.NewQuery()

	// build acl query
	if !isAdmin {
		metaacl := orQuery("acl_meta", groups)
		query.FilterQuery(metaacl)
	}
	if contentVisible {
		var contentacl string = "mediatype:[* TO *]"
		if !isAdmin {
			contentacl = fmt.Sprintf("(%s) AND (%s)", contentacl, orQuery("acl_content", groups))
		}
		query.FilterQuery(fmt.Sprintf("{!tag=facet_mediatype}%s", contentacl))
		//query.FilterQuery("mediatype:[* TO *]")
	}

	filterQuery := map[string]string{}
	// build facets with filter exclusion
	for field, vals := range facets {
		solrJSONTermsFacet := CreateJSONTermsFacetMap(field)
		if vals.limit != 0 {
			solrJSONTermsFacet.setLimit(vals.limit)
		}
		if vals.prefix != "" {
			solrJSONTermsFacet.SetTermPrefix(vals.prefix)
		}
		solrJSONDomainMap := CreateJSONDomainMap().WithTagsToExclude("facet_" + field)
		solrJSONTermsFacet.JSONFacetMap().withDomain(solrJSONDomainMap)
		json, err := json.Marshal(map[string]*JSONFacetMap{field: solrJSONTermsFacet.JSONFacetMap()})
		if err != nil {
			return nil, 0, nil, emperror.Wrapf(err, "cannot marshal facet %v", field)
		}
		query.AddJsonFacet(string(json))
		//		query.AddFacet(fmt.Sprintf("{!ex=%s}%s", facet, facet))
		// filterquery only needed if selections available
		selected := []string{}
		for val, sel := range vals.selected {
			if sel {
				selected = append(selected, val)
			}
		}
		if len(selected) > 0 {
			q := orQuery(field, selected)
			filterQuery[field] = q
		}
	}
	for field, q := range filterQuery {
		mts.log.Infof("Filterquery %s: %s", field, q)
		query.FilterQuery(fmt.Sprintf("{!tag=facet_%s}%s", field, q))
	}

	// filter query
	for _, fq := range filters {
		query.FilterQuery(fq)
	}

	query.Q(qstr)

	// we only need the id's
	//query.FieldList("id")

	// restrict result
	query.Start(start)
	query.Rows(rows)

	mts.log.Infof("solr query: %s - %v", query.String(), facets)
	s := mts.si.Search(query)
	r, err := s.Result(&solr.ExtensiveResultParser{})
	if err != nil {
		return nil, 0, nil, emperror.Wrapf(err, "search error for query %s - %v", query.String(), facets)
	}
	if r == nil {
		return nil, 0, nil, errors.New(fmt.Sprintf("no results for query %s - %v", qstr, facets))
	}
	/*
		if r.Results.NumFound == 0 {
			return []*Document{}, 0, nil, nil
		}
	*/
	mts.log.Infof("%v document(s) found", len(r.Results.Docs))

	ids := []string{}
	result := []*Document{}
	facetFields := make(FacetCountResult)
	var jsonFacets *SolrResultJSONFacets
	if r.JsonFacets != nil {
		jsonFacets, err = NewSolrResultJSONFacets()
		if err != nil {
			return nil, 0, nil, emperror.Wrap(err, "cannot create solr json facets")
		}
		if err := jsonFacets.Init(r.JsonFacets); err != nil {
			return nil, 0, nil, emperror.Wrapf(err, "cannot init json facets with %v", r.JsonFacets)
		}
		for name, elem := range jsonFacets.Elements {
			if _, ok := facetFields[name]; !ok {
				facetFields[name] = make(map[string]int)
			}
			for _, bucket := range elem.Buckets {
				facetFields[name][bucket.Val] = int(bucket.Count)
			}
		}
	}

	for _, doc := range r.Results.Docs {
		if !doc.Has("id") {
			return nil, 0, nil, errors.New(fmt.Sprintf("doc has no id field"))
		}
		idInterface := doc.Get("id")
		id, ok := idInterface.(string)
		if !ok {
			return nil, 0, nil, errors.New(fmt.Sprintf("id not a string"))
		}
		ids = append(ids, id)
		entry, id, err := mts.cacheEntryFromDoc(&doc)
		if err != nil {
			return nil, 0, nil, emperror.Wrapf(err, "cannot create cache entry from document")
		}

		var cDoc *Document
		cDoc, err = mts.getFromCache(id)
		if err != nil {
			// load the content
			content, err := mts.GetContent(entry)
			if err != nil {
				// keep error in cache as well
				cDoc = &Document{Id: entry.Id, Error: emperror.Wrapf(err, "cannot load content of %s", entry.Id).Error()}
			} else {

				// build full document
				sourceData := &SourceData{
					Source:          content.Name(),
					Title:           content.GetTitle(),
					Place:           content.GetPlace(),
					Date:            content.GetDate(),
					CollectionTitle: content.GetCollectionTitle(),
					Persons:         content.GetPersons(),
					Tags:            content.GetTags(),
					Media:           content.GetMedia(nil),
					Poster:          content.GetPoster(nil),
					Notes:           content.GetNotes(),
					Abstract:        content.GetAbstract(),
					References:      content.GetReferences(),
					Extra:           content.GetExtra(),
					Meta:            content.GetMeta(),
					Type:            content.GetContentType(),
					Queries:         content.GetQueries(),
				}
				sourceData.HasMedia = len(sourceData.Media) > 0

				cDoc = &Document{
					Content: sourceData,
					ACL:     entry.Acl,
					Catalog: entry.Catalog,
					Tag:     entry.Tag,
					Source:  entry.Source,
					Id:      entry.Id,
					Error:   "",
				}
			}
			if err := mts.storeCache(cDoc); err != nil {
				return nil, 0, nil, emperror.Wrapf(err, "cannot store document in cache")
			}
		}
		result = append(result, cDoc)
	}
	return result, int64(r.Results.NumFound), facetFields, nil
}