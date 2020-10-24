package search

type tElasticSearchFacet struct {
	Type string            `json:"type"`
	Name string            `json:"name,omitempty"`
	Sort map[string]string `json:"sort,omitempty"`
	Size int64             `json:"size,omitempty"`
}

func elasticSearchFacet(Name, Type string, Sort map[string]string, Size int64) *tElasticSearchFacet {
	return &tElasticSearchFacet{
		Type: Type,
		Name: Name,
		Sort: Sort,
		Size: Size,
	}
}

type tElasticSearchFacets map[string][]*tElasticSearchFacet

func (ef *tElasticSearchFacets) AddFacet(Field string, facets ...*tElasticSearchFacet) *tElasticSearchFacets {
	(*ef)[Field] = facets
	return ef
}
func elasticSearchFacets() *tElasticSearchFacets {
	return &tElasticSearchFacets{}
}
