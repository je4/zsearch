package source

import (
	"github.com/vanng822/go-solr/solr"
)

type SolrFacet struct {
	Label    string
	Name     string
	Field    string
	Query    string
	Prefix   string
	Restrict []string
}

func Create(Label, Name, Field, Query, Prefix string, Restrict []string) (*SolrFacet, error) {
	facet := &SolrFacet{
		Label: Label,
		Name:     Name,
		Field:    Field,
		Query:    Query,
		Prefix:   Prefix,
		Restrict: Restrict,
	}
	return facet, nil
}

func (sf *SolrFacet) Add(query *solr.Query, selected []string) error {
	facetQuery := make(map[string]interface{})
	if sf.Query == "" {
		facetQuery["type"] = "query"
		facetQuery["q"] = sf.Query
		if sf.Field != "" {
			facetQuery["facet"] = map[string]interface{}{
				sf.Name: map[string]interface{}{
					"type": "term",
					"field": sf.Field,
					"prefix": sf.Prefix,
			}}
		}
	} else {
		facetQuery["type"] = "query"
		facetQuery["q"] = sf.Query
	}
	return nil
}

