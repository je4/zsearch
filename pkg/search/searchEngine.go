package search

import (
	"context"
	"github.com/je4/zsearch/pkg/mediaserver"
)

type TermFacet struct {
	Selected map[string]bool
	Prefix   string
	Limit    int64
}

type SearchConfig struct {
	Fields         map[string][]string
	QStr           string
	FiltersFields  map[string][]string
	Facets         map[string]TermFacet
	Groups         []string
	ContentVisible bool
	Start          int
	Rows           int
	IsAdmin        bool
}

type SearchEngine interface {
	Update(source Source, ms mediaserver.Mediaserver) error
	LoadDocs(ids []string, ctx context.Context) (map[string]*SourceData, error)
	Search(cfg *SearchConfig) ([]map[string][]string, []*SourceData, int64, FacetCountResult, error)
}
