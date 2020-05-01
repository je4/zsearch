package mtsolr

import (
	"errors"
	"fmt"
	"github.com/goph/emperror"
	"github.com/vanng822/go-solr/solr"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/source"
	"regexp"
)

type MTSolr struct {
	si *solr.SolrInterface
}

func escapeSolrString(str string) string {
	var re = regexp.MustCompile(`([-\\!():^\[\]"{}~*?|&;/+]|[[:space:]])`)
	return re.ReplaceAllString(str, "\\$1")
}

func NewMTSolr(url, core string) (*MTSolr, error) {
	si, err := solr.NewSolrInterface(url, core)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot create solr interface for %s/%s", url, core)
	}
	mts := &MTSolr{si: si}
	return mts, nil
}

func (mts *MTSolr) LoadEntity(id string) (source.Source, error) {
	query := solr.NewQuery()
	query.Q(fmt.Sprintf("id:%s", escapeSolrString(id)))
	s := mts.si.Search(query)
	r, _ := s.Result(nil)
	if r.Results.NumFound == 0 {
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
	metadata, err := source.GUnzip(metaGZString)
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
	switch(srcstr) {
	case "zotero":
		return source.NewZotero(metadata)
	default:
		return nil, errors.New(fmt.Sprintf("invalid source %s", srcstr))
	}
}
