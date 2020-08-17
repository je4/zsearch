/*
Copyright 2020 Center for Digital Matter HGK FHNW, Basel.
Copyright 2020 info-age GmbH, Basel.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS-IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package gsearch

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dgraph-io/badger"
	"github.com/goph/emperror"
	"github.com/op/go-logging"
	"github.com/vanng822/go-solr/solr"
	"regexp"
	"sync"
	"time"
)

type MTSolr struct {
	si *solr.SolrInterface
	db *badger.DB
	//cache gcache.Cache
	sync.Mutex
	log *logging.Logger
}

type FacetCountResult map[string]map[string]int

func EscapeSolrString(str string) string {
	var re = regexp.MustCompile(`([-\\!():^\[\]"{}~*?|&;/+]|[[:space:]])`)
	return re.ReplaceAllString(str, "\\$1")
}

func orQuery(field string, values []string) string {
	q := ""
	for key, val := range values {
		if key > 0 {
			q += " OR "
		}
		q += fmt.Sprintf("(%s:%s)", field, val)
	}
	return q
}

func NewMTSolr(url, core string, expiration time.Duration, cachesize int, db *badger.DB, log *logging.Logger) (*MTSolr, error) {
	si, err := solr.NewSolrInterface(url, core)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot create solr interface for %s/%s", url, core)
	}
	mts := &MTSolr{
		si:  si,
		db:  db,
		log: log,
	}
	return mts, nil
}

type cacheEntry struct {
	Id         string `json:"id"`
	Source     string `json:"source"`
	ContentStr string `json:"content"`
	//	Content    Source              `json:"-"`
	Acl     map[string][]string `json:"acl"`
	Catalog []string            `json:"catalog"`
	Tag     []string            `json:"tag"`
	Doc     *solr.Document
}

func (mts *MTSolr) GetContent(entry *cacheEntry) (Source, error) {
	var content Source
	var err error
	switch entry.Source {
	case "zotero":
		content, err = NewSourceZotero(entry, mts)
	case "diplomhgk":
		content, err = NewSourceDiplomHGK(entry, mts)
	default:
		err = errors.New(fmt.Sprintf("invalid Source %s", entry.ContentStr))
	}
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot load Source data")
	}
	return content, nil
}

func interface2StringSlice(d interface{}) (result []string) {
	list, ok := d.([]interface{})
	if !ok {
		return
	}
	for _, i := range list {
		str, ok := i.(string)
		if !ok {
			return
		}
		result = append(result, str)
	}
	return
}

func (mts *MTSolr) cacheEntryFromDoc(doc *solr.Document) (*cacheEntry, string, error) {
	if !doc.Has("id") {
		return nil, "", errors.New(fmt.Sprintf("doc has no id field"))
	}
	idv := doc.Get("id")
	id, ok := idv.(string)
	if !ok {
		return nil, "", errors.New(fmt.Sprintf("id not a string"))
	}

	if !doc.Has("metagz") {
		return nil, "", errors.New(fmt.Sprintf("id %s has no metagz field", id))
	}
	mgz := doc.Get("metagz")
	metaGZString, ok := mgz.(string)
	if !ok {
		return nil, "", errors.New(fmt.Sprintf("metagz of id %s is not a string", id))
	}
	metadata, err := GUnzip(metaGZString)
	if err != nil {
		return nil, "", emperror.Wrapf(err, "cannot gunzip metadata of id %s", id)
	}
	if !doc.Has("source") {
		return nil, "", errors.New(fmt.Sprintf("id %s has no source field", id))
	}
	src := doc.Get("source")
	srcstr, ok := src.(string)
	if !ok {
		return nil, "", errors.New(fmt.Sprintf("source of id %s is not a string", id))
	}
	if !doc.Has("acl_meta") {
		return nil, "", errors.New(fmt.Sprintf("id %s has no acl_meta field", id))
	}
	acl_metaI := doc.Get("acl_meta")
	acl_meta := interface2StringSlice(acl_metaI)

	if !doc.Has("acl_content") {
		return nil, "", errors.New(fmt.Sprintf("id %s has no acl_content field", id))
	}

	acl_contentI := doc.Get("acl_content")
	acl_content := interface2StringSlice(acl_contentI)

	if !doc.Has("catalog") {
		return nil, "", errors.New(fmt.Sprintf("id %s has no catalog field", id))
	}
	catalogI := doc.Get("catalog")
	catalog := interface2StringSlice(catalogI)

	if !doc.Has("cluster") {
		return nil, "", errors.New(fmt.Sprintf("id %s has no cluster field", id))
	}
	clusterI := doc.Get("cluster")
	cluster := interface2StringSlice(clusterI)

	entry := &cacheEntry{
		Id:         id,
		Source:     srcstr,
		ContentStr: metadata,
		Acl: map[string][]string{
			"meta":    acl_meta,
			"content": acl_content,
		},
		Catalog: catalog,
		Tag:     cluster,
		Doc:     doc,
	}
	return entry, id, nil
}

func (mts *MTSolr) getSolrDocRaw(id string) (solr.Document, error) {
	docs, err := mts.getSolrDocsRaw([]string{id})
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot load solr doc %v", id)
	}
	if len(docs) != 1 {
		return nil, fmt.Errorf("cannot find solr doc %v", id)
	}
	return docs[0], nil
}
func (mts *MTSolr) getSolrDocsRaw(ids []string) ([]solr.Document, error) {
	numIDs := len(ids)
	if numIDs == 0 {
		return []solr.Document{}, nil
	}
	qstr := ""
	for _, id := range ids {
		if qstr != "" {
			qstr += " OR "
		}
		qstr += fmt.Sprintf("(id:%s)", EscapeSolrString(id))
	}
	query := solr.NewQuery()
	query.Q(qstr)
	mts.log.Infof("solr query: %s", query.String())
	s := mts.si.Search(query)
	r, _ := s.Result(nil)
	if r == nil || r.Results.NumFound == 0 {
		return nil, errors.New(fmt.Sprintf("ids %v not found", ids))
	}
	mts.log.Infof("%v document(s) found", len(r.Results.Docs))
	if len(r.Results.Docs) != numIDs {
		return nil, errors.New(fmt.Sprintf("got %v documents for %v ids %v", len(r.Results.Docs), numIDs, ids))
	}
	return r.Results.Docs, nil
}

func (mts *MTSolr) getSolrDocs(ids []string) (map[string]*cacheEntry, error) {
	docs, err := mts.getSolrDocsRaw(ids)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot load solr docs")
	}
	result := make(map[string]*cacheEntry)
	for _, doc := range docs {
		entry, id, err := mts.cacheEntryFromDoc(&doc)
		if err != nil {
			return nil, emperror.Wrapf(err, "cannot create cache entry from document")
		}
		result[id] = entry
	}
	return result, nil
}

func (mts *MTSolr) LoadEntity(id string) (*Document, error) {
	entries, err := mts.LoadEntities([]string{id})
	if err != nil {
		return nil, err
	}
	entry, ok := entries[id]
	if !ok {
		return nil, fmt.Errorf("could not get entity %s", id)
	}
	if entry.Error != "" {
		return nil, fmt.Errorf("could not load entity %s: %s", entry.Error)
	}
	return entry, nil
}

func (mts *MTSolr) storeCache(doc *Document) error {
	// marshal entry
	jsonstr, err := json.Marshal(*doc)
	if err != nil {
		return emperror.Wrap(err, "cannot marshal entry")
	}
	// compress it
	data := Compress([]byte(jsonstr))
	if err := mts.db.Update(func(txn *badger.Txn) error {
		txn.Set([]byte(doc.Id), data)
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (mts *MTSolr) getFromCache(id string) (*Document, error) {
	var result *Document
	if err := mts.db.View(func(txn *badger.Txn) error {
		it, err := txn.Get([]byte(id))
		if err != nil {
			return emperror.Wrapf(err, "cannot get item %s", id)
		}
		if it == nil {
			return fmt.Errorf("item %s not in cache", id)
		}
		if err := it.Value(func(v []byte) error {
			var doc = &Document{}

			// decompress...
			data, err := Decompress(v)
			if err != nil {
				return emperror.Wrapf(err, "cannot deocmpress %s", string(v))
			}
			// ...unmarshal
			if err := json.Unmarshal(data, doc); err != nil {
				return emperror.Wrapf(err, "cannot unmarshal json %s", string(v))
			}
			mts.log.Infof("document %s found in cache", id)
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

func (mts *MTSolr) LoadEntities(ids []string) (map[string]*Document, error) {
	// todo: need better locking stragegy
	mts.Lock()
	defer mts.Unlock()

	var result = make(map[string]*Document)
	var toLoad []string

	//
	// try loading from cache
	//
	for _, id := range ids {
		doc, err := mts.getFromCache(id)
		if err != nil {
			toLoad = append(toLoad, id)
		} else {
			result[doc.Id] = doc
		}
	}

	//
	// then load the rest from index
	//
	entries, err := mts.getSolrDocs(toLoad)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot load entities %v", ids)
	}
	var doclist = make(map[string]*Document)
	for _, id := range toLoad {

		// check whether it's found
		entry, ok := entries[id]
		if !ok {
			// keep error in cache as well
			doc := &Document{Id: id, Error: fmt.Sprintf("id %s not found in index", id)}
			doclist[doc.Id] = doc
			continue
		}

		// load the content
		content, err := mts.GetContent(entry)
		if err != nil {
			// keep error in cache as well
			doc := &Document{Id: entry.Id, Error: emperror.Wrapf(err, "cannot load content of %s", entry.Id).Error()}
			doclist[doc.Id] = doc
			continue
		}

		// build full document
		sourceData := &SourceData{
			Source:          content.Name(),
			Title:           content.GetTitle(),
			Place:           content.GetPlace(),
			Date:            content.GetDate(),
			CollectionTitle: content.GetCollectionTitle(),
			Persons:         content.GetNames(),
			Tags:            content.GetTags(),
			Media:           content.GetMedia(),
			Notes:           content.GetNotes(),
			Abstract:        content.GetAbstract(),
			References:      content.GetReferences(),
			Meta:            content.GetMeta(),
			Type:            content.GetType(),
			Queries:         content.GetQueries(),
			ContentStr:      content.GetContentString(),
			ContentMime:     content.GetContentMime(),
		}
		sourceData.HasMedia = len(sourceData.Media) > 0

		doc := &Document{
			Content: sourceData,
			ACL:     entry.Acl,
			Catalog: entry.Catalog,
			Tag:     entry.Tag,
			Source:  entry.Source,
			Id:      entry.Id,
			Error:   "",
		}
		doclist[id] = doc
	}
	for _, doc := range doclist {
		if err := mts.storeCache(doc); err != nil {
			return nil, err
		}
		result[doc.Id] = doc
	}
	return result, nil
}

func (mts *MTSolr) Search(text string, filters []string, facets map[string]map[string]bool, groups []string, contentVisible bool, start, rows int) ([]*Document, int64, FacetCountResult, error) {
	//qstr := EscapeSolrString(text)
	qstr := text
	if qstr == "" {
		qstr = "*:*"
	}
	query := solr.NewQuery()

	// build acl query
	metaacl := orQuery("acl_meta", groups)
	query.FilterQuery(metaacl)
	if contentVisible {
		contentacl := orQuery("acl_content", groups)
		query.FilterQuery(contentacl)
		query.FilterQuery("mediatype:[* TO *]")
	}

	filterQuery := ""
	// build facets with filter exclusion
	for field, vals := range facets {
		solrJSONTermsFacet := CreateJSONTermsFacetMap(field)
		solrJSONDomainMap := CreateJSONDomainMap().WithTagsToExclude("facet")
		solrJSONTermsFacet.JSONFacetMap().withDomain(solrJSONDomainMap)
		json, err := json.Marshal(map[string]*JSONFacetMap{field: solrJSONTermsFacet.JSONFacetMap()})
		if err != nil {
			return nil, 0, nil, emperror.Wrapf(err, "cannot marshal facet %v", field)
		}
		query.AddJsonFacet(string(json))
		//		query.AddFacet(fmt.Sprintf("{!ex=%s}%s", facet, facet))
		// filterquery only needed if selections available
		selected := []string{}
		for val, sel := range vals {
			if sel {
				selected = append(selected, val)
			}
		}
		if len(selected) > 0 {
			q := orQuery(field, selected)
			if filterQuery != "" {
				filterQuery += " AND "
			}
			filterQuery += fmt.Sprintf("(%s)", q)
		}
	}
	if filterQuery != "" {
		query.FilterQuery(fmt.Sprintf("{!tag=%s}%s", "facet", filterQuery))
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
					Persons:         content.GetNames(),
					Tags:            content.GetTags(),
					Media:           content.GetMedia(),
					Notes:           content.GetNotes(),
					Abstract:        content.GetAbstract(),
					References:      content.GetReferences(),
					Meta:            content.GetMeta(),
					Type:            content.GetType(),
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
