package mtsolr

import "gitlab.fhnw.ch/mediathek/search/gsearch/pkg/source"

type Document struct {
	Source *source.SourceData
	ACL    map[string][]string
	Id     string
}

func (d *Document) IsPublicMeta() bool {
	for acl, groups := range d.ACL {
		if acl == "acl_meta" {
			for _, group := range groups {
				if group == "global/guest" {
					return true
				}
			}
			// fast return. other rights do not matter
			return false
		}
	}
	return false
}
