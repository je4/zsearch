package search

import "log"

type JSONFacetMap map[string]interface{}

func CreateJSONFacetMap(t string) *JSONFacetMap {
	jfm := &JSONFacetMap{"type": t}
	return jfm
}

func (jfm *JSONFacetMap) withDomain(domain *JSONDomainMap) *JSONFacetMap {
	(*jfm)["domain"] = domain
	return jfm
}

func (jfm *JSONFacetMap) withSubFacet(facetName string, m *JSONFacetMap) *JSONFacetMap {
	_m, ok := (*jfm)["facet"]
	if !ok {
		_m = map[string]*JSONFacetMap{}
		(*jfm)["facet"] = _m
	}
	facetList, ok := _m.(map[string]*JSONFacetMap)
	if !ok {
		log.Panicf("invalid facet type for %s", facetName)
		return jfm
	}
	facetList[facetName] = m
	return jfm
}

func (jfm *JSONFacetMap) withStatSubFacet(facetName string, m string) *JSONFacetMap {
	_m, ok := (*jfm)["facet"]
	if !ok {
		_m = map[string]*JSONFacetMap{}
		(*jfm)["facet"] = _m
	}
	facetList, ok := _m.(map[string]string)
	if !ok {
		log.Panicf("invalid facet type for %s", facetName)
		return jfm
	}
	facetList[facetName] = m
	return jfm
}
