package mtsolr

import "github.com/vanng822/go-solr/solr"
import "gitlab.fhnw.ch/mediathek/search/gsearch/pgk/source"

type MTSolr struct {
	si *solr.SolrInterface
}

func NewMTSolr(si *solr.SolrInterface) (*MTSolr, error) {
	mts := &MTSolr{si: si}
	return mts, nil
}

func (mts *MTSolr) LoadEntity(id string) (*source.Source, error) {

}
