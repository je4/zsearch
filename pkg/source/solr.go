package source

import (
	"errors"
	"fmt"
	"github.com/bluele/gcache"
	"github.com/goph/emperror"
	"github.com/vanng822/go-solr/solr"
	"regexp"
	"sync"
	"time"
)

type MTSolr struct {
	si    *solr.SolrInterface
	cache gcache.Cache
	sync.Mutex
}

func escapeSolrString(str string) string {
	var re = regexp.MustCompile(`([-\\!():^\[\]"{}~*?|&;/+]|[[:space:]])`)
	return re.ReplaceAllString(str, "\\$1")
}

func NewMTSolr(url, core string, expiration time.Duration, cachesize int) (*MTSolr, error) {
	si, err := solr.NewSolrInterface(url, core)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot create solr interface for %s/%s", url, core)
	}
	mts := &MTSolr{
		si:    si,
		cache: gcache.New(cachesize).ARC().Expiration(expiration).Build(),
	}
	return mts, nil
}

type cacheEntry struct {
	content Source
	doc     solr.Document
}

func (mts *MTSolr) LoadData(id string) (*cacheEntry, error) {
	mts.Lock()
	defer mts.Unlock()
	var entry *cacheEntry
	// try to get content from cache
	entryInt, err := mts.cache.Get(id)
	if err == nil {
		entry, _ = entryInt.(*cacheEntry)
	}
	// if not found load it from index
	if entry == nil {
		query := solr.NewQuery()
		query.Q(fmt.Sprintf("id:%s", escapeSolrString(id)))
		s := mts.si.Search(query)
		r, _ := s.Result(nil)
		if r == nil || r.Results.NumFound == 0 {
			return nil, errors.New(fmt.Sprintf("id %s not found", id))
		}
		if r.Results.NumFound > 1 {
			return nil, errors.New(fmt.Sprintf("id %s not unique, %v found", id, r.Results.NumFound))
		}
		if len(r.Results.Docs) != 1 {
			return nil, errors.New(fmt.Sprintf("got %v documents for id %s", len(r.Results.Docs), id))
		}
		doc := r.Results.Docs[0]
		if !doc.Has("metagz") {
			return nil, errors.New(fmt.Sprintf("id %s has no metagz field", id))
		}
		mgz := doc.Get("metagz")
		metaGZString, ok := mgz.(string)
		if !ok {
			return nil, errors.New(fmt.Sprintf("metagz of id %s is not a string", id))
		}
		metadata, err := GUnzip(metaGZString)
		if err != nil {
			return nil, emperror.Wrapf(err, "cannot gunzip metadata of id %s", id)
		}
		if !doc.Has("source") {
			return nil, errors.New(fmt.Sprintf("id %s has no source field", id))
		}
		src := doc.Get("source")
		srcstr, ok := src.(string)
		if !ok {
			return nil, errors.New(fmt.Sprintf("source of id %s is not a string", id))
		}
		var content Source
		switch srcstr {
		case "zotero":
			content, err = NewZotero(metadata, mts)
		default:
			err = errors.New(fmt.Sprintf("invalid source %s", srcstr))
		}
		if err != nil {
			return nil, emperror.Wrapf(err, "cannot load source data")
		}
		entry = &cacheEntry{
			content: content,
			doc:     doc,
		}
		mts.cache.Set(id, entry)
	}
	return entry, nil
}

func (mts *MTSolr) LoadEntity(id string) (*Document, error) {
	entry, err := mts.LoadData(id)
	if err != nil {
		return nil, err
	}
	sourceData := &SourceData{
		Source:          entry.content.Name(),
		Title:           entry.content.GetTitle(),
		Place:           entry.content.GetPlace(),
		Date:            entry.content.GetDate(),
		CollectionTitle: entry.content.GetCollectionTitle(),
		Persons:         entry.content.GetNames(),
		Tags:            entry.content.GetTags(),
		Media:           entry.content.GetMedia(),
		Notes:           entry.content.GetNotes(),
		Abstract:        entry.content.GetAbstract(),
		References:      entry.content.GetReferences(),
	}
	sourceData.HasMedia = len(sourceData.Media) > 0

	acl := map[string][]string{}
	acl["meta"] = []string{}
	a := entry.doc.Get("acl_meta")
	alist, ok := a.([]interface{})
	if ok {
		for _, al := range alist {
			s, ok := al.(string)
			if ok {
				acl["meta"] = append(acl["meta"], s)
			}
		}
	}

	acl["content"] = []string{}
	a = entry.doc.Get("acl_content")
	alist, ok = a.([]interface{})
	if ok {
		for _, al := range alist {
			s, ok := al.(string)
			if ok {
				acl["content"] = append(acl["content"], s)
			}
		}
	}

	acl["preview"] = []string{}
	a = entry.doc.Get("acl_preview")
	alist, ok = a.([]interface{})
	if ok {
		for _, al := range alist {
			s, ok := al.(string)
			if ok {
				acl["preview"] = append(acl["preview"], s)
			}
		}
	}

	result := &Document{
		Source: sourceData,
		ACL:    acl,
		Id:     id,
	}
	return result, nil
}
