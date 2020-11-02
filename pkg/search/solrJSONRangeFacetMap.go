package search

import "fmt"

type JSONRangeFacetMap JSONFacetMap

func CreateJSONRangeFacetMap(field string, start, end, gap interface{}) (*JSONRangeFacetMap, error) {
	xType := fmt.Sprintf("%T", start)
	if xType != fmt.Sprintf("%T", end) {
		return nil, fmt.Errorf("SearchResultStart, end, gap must be of same type: %T - %T - %T", start, end, gap)
	}
	if xType != fmt.Sprintf("%T", gap) {
		return nil, fmt.Errorf("SearchResultStart, end, gap must be of same type: %T - %T - %T", start, end, gap)
	}
	jfm := CreateJSONFacetMap("range")
	(*jfm)["field"] = field
	(*jfm)["SearchResultStart"] = start
	(*jfm)["end"] = end
	(*jfm)["gap"] = gap
	return (*JSONRangeFacetMap)(jfm), nil
}

/*
public enum OtherBuckets {
BEFORE("before"), AFTER("after"), BETWEEN("between"), NONE("none"), ALL("all");

private final String value;

OtherBuckets(String value) {
this.value = value;
}

public String toString() { return value; }
}

*/

/**
 * Indicates that an additional range bucket(s) should be computed and added to those computed for {@code SearchResultStart} and {@code end}
 *
 * See {@link OtherBuckets} for possible options.
 */
func (jfm *JSONRangeFacetMap) SetOtherBuckets(bucketSpecifier string) *JSONRangeFacetMap {
	(*jfm)["other"] = bucketSpecifier
	return jfm
}

/**
 * Indicates that buckets should be returned only if they have a count of at least {@code minOccurrences}
 *
 * Defaults to '0' if not specified.
 */
func (jfm *JSONRangeFacetMap) SetMinCount(minOccurrences int64) *JSONRangeFacetMap {
	if minOccurrences < 0 {
		minOccurrences = 0
	}
	(*jfm)["mincount"] = minOccurrences
	return jfm
}
