package search

type SolrFacet struct {
	Label    string          `json:"label,omitempty"`
	Name     string          `json:"Name,omitempty"`
	Field    string          `json:"field"`
	Prefix   string          `json:"Prefix,omitempty"`
	Restrict map[string]bool `json:"restrict,omitempty"`
}

type SolrFacetList map[string]SolrFacet

func Create(Label, Name, Field, Prefix string, Restrict map[string]bool) (*SolrFacet, error) {
	facet := &SolrFacet{
		Label:    Label,
		Name:     Name,
		Field:    Field,
		Prefix:   Prefix,
		Restrict: Restrict,
	}
	return facet, nil
}
