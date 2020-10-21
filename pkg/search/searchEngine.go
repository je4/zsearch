package search

import "github.com/je4/zsearch/pkg/mediaserver"

type SearchEngine interface {
	Update(source Source, ms mediaserver.Mediaserver) error
	LoadDocs(ids []string) (map[string]*SourceData, error)
}
