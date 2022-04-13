package search

import (
	"context"
	"time"
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

type ScrollConfig struct {
	Fields         map[string][]string
	QStr           string
	FiltersFields  map[string][]string
	Groups         []string
	ContentVisible bool
	IsAdmin        bool
}

type SearchEngine interface {
	Update(source *SourceData) error
	UpdateTimestamp(source *SourceData, timestamp time.Time) error
	LoadDocs(ids []string, ctx context.Context) (map[string]*SourceData, error)
	Search(cfg *SearchConfig) ([]map[string][]string, []*SourceData, int64, FacetCountResult, error)
	Delete(cfg *ScrollConfig) (int64, error)
	StatsByACL(catalog []string) (int64, FacetCountResult, error)
	LastUpdate(cfg *ScrollConfig) (time.Time, error)
	Scroll(cfg *ScrollConfig, f func(data *SourceData) error) error
}
