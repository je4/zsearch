package search

import (
	"context"
	"fmt"
	"github.com/dgraph-io/badger/v2"
	"github.com/goph/emperror"
	"github.com/op/go-logging"
	"go.mongodb.org/mongo-driver/bson"
	"sync"
	"time"
)

type Search struct {
	db *badger.DB
	sync.Mutex
	log *logging.Logger
	se  SearchEngine
}

type FacetCountResult map[string]map[string]int

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
	jsonstr, err := bson.Marshal(*src)
	if err != nil {
		return emperror.Wrapf(err, "cannot marshal source data of %v", src.Signature)
	}
	data := Compress([]byte(jsonstr))
	if err := s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(src.Signature), data)
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
			if err := bson.Unmarshal(data, doc); err != nil {
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
	entries, err := s.se.LoadDocs(toLoad, context.Background())
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot load entities %v", ids)
	}
	// store results in cache
	for _, sdata := range entries {
		result[sdata.Signature] = sdata
		_ = s.storeCache(sdata)
	}
	return result, nil
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

func (s *Search) Search(cfg *SearchConfig) ([]map[string][]string, []*SourceData, int64, FacetCountResult, error) {

	highlights, result, num, fts, err := s.se.Search(cfg)
	if err != nil {
		return nil, nil, 0, nil, emperror.Wrap(err, "cannot search")
	}

	return highlights, result, num, fts, nil
}
