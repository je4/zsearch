package search

type tElasticSearchAggregation struct {
	Terms        map[string]interface{}      `json:"terms,omitempty"`
	Aggregations *tElasticSearchAggregations `json:"aggs,omitempty"`
}

func (esa *tElasticSearchAggregation) withTerms(field string, size int64, order map[string]string) *tElasticSearchAggregation {
	esa.Terms = map[string]interface{}{
		"field": field,
	}
	if size > 0 {
		esa.Terms["size"] = size
	}
	if order != nil {
		esa.Terms["order"] = order
	}
	return esa
}
func elasticSearchAggregation(aggregations *tElasticSearchAggregations) *tElasticSearchAggregation {
	return &tElasticSearchAggregation{
		Aggregations: aggregations,
	}
}

type tElasticSearchAggregations map[string]*tElasticSearchAggregation

func (ef *tElasticSearchAggregations) AddAggregation(Field string, aggregations *tElasticSearchAggregation) *tElasticSearchAggregations {
	(*ef)[Field] = aggregations
	return ef
}
func elasticSearchAggregations() *tElasticSearchAggregations {
	return &tElasticSearchAggregations{}
}
