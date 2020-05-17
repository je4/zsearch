package mtsolr

import "gitlab.fhnw.ch/mediathek/search/gsearch/pkg/source"

type Document struct {
	Source     *source.SourceData
	ACL        map[string][]string
	Id         string
}
