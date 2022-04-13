package search

type JSONQueryFacetMap JSONFacetMap

func CreateJSONQueryFacetMap() *JSONQueryFacetMap {
	jfm := CreateJSONFacetMap("query")
	return (*JSONQueryFacetMap)(jfm)
}
