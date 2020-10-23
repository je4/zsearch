package search

import (
	"context"
	"github.com/je4/zsearch/pkg/mediaserver"
)

type SearchConfig struct {
	filters        []string
	facets         map[string]termFacet
	groups         []string
	contentVisible bool
	start          int
	rows           int
	isAdmin        bool
}

type SearchEngine interface {
	Update(source Source, ms mediaserver.Mediaserver) error
	LoadDocs(ids []string, ctx context.Context) (map[string]*SourceData, error)
	Search(
		text string,
		cfg *SearchConfig) ([]*SourceData, int64, FacetCountResult, error)
}
