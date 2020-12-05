package search

type JSONDomainMap map[string]interface{}

func CreateJSONDomainMap() *JSONDomainMap {
	return &JSONDomainMap{}
}

/**
 * Indicates that the domain should be narrowed by the specified filter
 *
 * May be called multiple times.  Each added filter is retained and used to narrow the domain.
 */
func (jdm *JSONDomainMap) WithFilter(filter string) *JSONDomainMap {
	if _, ok := (*jdm)["filter"]; !ok {
		(*jdm)["filter"] = []string{}
	}
	(*jdm)["filter"] = append((*jdm)["filter"].([]string), filter)
	return jdm
}

/**
 * Indicates that the domain should be the following query
 *
 * May be called multiple times.  Each specified query is retained and included in the domain.
 */
func (jdm *JSONDomainMap) WithQuery(query string) *JSONDomainMap {
	if _, ok := (*jdm)["query"]; !ok {
		(*jdm)["query"] = []string{}
	}
	(*jdm)["query"] = append((*jdm)["query"].([]string), query)
	return jdm
}

/**
 * Provide a tag or tags that correspond to filters or queries to exclude from the domain
 *
 * May be called multiple times.  Each exclude-string is retained and used for removing queries/filters from the
 * domain specification.
 *
 * @param excludeTagsValue a comma-delimited String containing filter/query tags to exclude
 */
func (jdm *JSONDomainMap) WithTagsToExclude(excludeTagsValue string) *JSONDomainMap {
	if _, ok := (*jdm)["excludeTags"]; !ok {
		(*jdm)["excludeTags"] = []string{}
	}
	(*jdm)["excludeTags"] = append((*jdm)["excludeTags"].([]string), excludeTagsValue)

	//	(*jdm)["excludeTags"] = excludeTagsValue
	return jdm
}

/**
 * Indicates that the resulting domain will contain all parent documents of the children in the existing domain
 *
 * @param allParentsQuery a query used to identify all parent documents in the collection
 */
func (jdm *JSONDomainMap) SetBlockParentQuery(allParentsQuery string) *JSONDomainMap {
	(*jdm)["blockParent"] = allParentsQuery
	return jdm
}

/**
 * Indicates that the resulting domain will contain all child documents of the parents in the current domain
 *
 * @param allChildrenQuery a query used to identify all child documents in the collection
 */
func (jdm *JSONDomainMap) setBlockChildQuery(allChildrenQuery string) *JSONDomainMap {
	(*jdm)["blockChildren"] = allChildrenQuery
	return jdm
}

/**
 * Transforms the domain by running a join query with the provided {@code from} and {@code to} parameters
 *
 * Join modifies the current domain by selecting the documents whose values in field {@code to} match values for the
 * field {@code from} in the current domain.
 *
 * @param from a field-Name whose values are matched against {@code to} by the join
 * @param to a field Name whose values should match values specified by the {@code from} field
 */
func (jdm *JSONDomainMap) setJoinTransformation(from, to string) *JSONDomainMap {
	(*jdm)["join"] = map[string]interface{}{"from": from, "to": to}
	return jdm
}
