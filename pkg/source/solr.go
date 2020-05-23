package source

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dgraph-io/badger"
	"github.com/goph/emperror"
	"github.com/op/go-logging"
	"github.com/vanng822/go-solr/solr"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/generic"
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

func escapeSolrString(str string) string {
	var re = regexp.MustCompile(`([-\\!():^\[\]"{}~*?|&;/+]|[[:space:]])`)
	return re.ReplaceAllString(str, "\\$1")
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
	Id string `json:"id"`
	Source     string `json:"source"`
	ContentStr string `json:"content"`
	//	Content    Source              `json:"-"`
	Acl     map[string][]string `json:"acl"`
	Catalog []string            `json:"catalog"`
	Tag     []string            `json:"tag"`
	//doc     solr.Document
}

func (mts *MTSolr) GetContent(entry *cacheEntry) (Source, error) {
	var content Source
	var err error
	switch entry.Source {
	case "zotero":
		content, err = NewZotero(entry.ContentStr, mts)
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

func (mts *MTSolr) cacheEntryFromDoc(doc solr.Document) (*cacheEntry, string, error) {
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
		Id: id,
		Source:     srcstr,
		ContentStr: metadata,
		Acl: map[string][]string{
			"meta":       acl_meta,
			"ContentStr": acl_content,
		},
		Catalog: catalog,
		Tag:     cluster,
	}
	return entry, id, nil
}

func (mts *MTSolr) getSolrDocs(ids []string) (map[string]*cacheEntry, error) {
	numIDs := len(ids)
	qstr := ""
	for _, id := range ids {
		if qstr != "" {
			qstr += " OR "
		}
		qstr += fmt.Sprintf("(id:%s)", escapeSolrString(id))
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
	result := make(map[string]*cacheEntry)
	for _, doc := range r.Results.Docs {
		entry, id, err := mts.cacheEntryFromDoc(doc)
		if err != nil {
			return nil, emperror.Wrapf(err, "cannot create cache entry from document")
		}
		result[id] = entry
	}
	return result, nil
}

// todo: refactor fo multiple docs
func (mts *MTSolr) LoadData(ids []string) (map[string]*cacheEntry, error) {
	result := make(map[string]*cacheEntry)
	mts.Lock()
	defer mts.Unlock()
	for _, id := range ids {
		var entry *cacheEntry
		// try to get ContentStr from cache
		if err := mts.db.View(func(txn *badger.Txn) error {
			it, err := txn.Get([]byte(id))
			if err != nil {
				return emperror.Wrapf(err, "cannot get item %s", id)
			}
			if err := it.Value(func(v []byte) error {
				data, err := generic.Decompress(v)
				if err != nil {
					return emperror.Wrapf(err, "cannot deocmpress %s", string(v))
				}
				entry = &cacheEntry{}
				if err := json.Unmarshal(data, entry); err != nil {
					return emperror.Wrapf(err, "cannot unmarshal json %s", string(v))
				}
				return nil
			}); err != nil {
				return emperror.Wrapf(err, "cannot load item %s", id)
			}
			mts.log.Infof("document %s found in cache", id)
			return nil
		}); err != nil {
			entries, err := mts.getSolrDocs([]string{id})
			if err != nil {
				return nil, emperror.Wrapf(err, "cannot load document %s", id)
			}
			var ok bool
			entry, ok = entries[id]
			if !ok {
				return nil, fmt.Errorf("id %v not in result", id)
			}
			if err := mts.db.Update(func(txn *badger.Txn) error {
				// marshal entry
				jsonstr, err := json.Marshal(*entry)
				if err != nil {
					return emperror.Wrap(err, "cannot marshal entry")
				}

				data := generic.Compress([]byte(jsonstr))
				txn.Set([]byte(id), data)
				return nil
			}); err != nil {
				return nil, err
			}
		}
		result[id] = entry
	}
	return result, nil
}

func (mts *MTSolr) LoadEntity(id string) (*Document, error) {
	entries, err := mts.LoadData([]string{id})
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot load entity %s", id)
	}
	entry, ok := entries[id]
	if !ok {
		return nil, emperror.Wrapf(err, "cannot get entity %s from result", id)
	}

	content, err := mts.GetContent(entry)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot load content of %s", id)
	}

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
	}
	sourceData.HasMedia = len(sourceData.Media) > 0

	result := &Document{
		Content: sourceData,
		ACL:     entry.Acl,
		Catalog: entry.Catalog,
		Tag:     entry.Tag,
		Source:  entry.Source,
		Id:      id,
	}
	return result, nil
}
