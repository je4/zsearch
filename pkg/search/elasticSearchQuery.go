package search

/*
Term Query
https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-term-query.html
*/
type tElasticTermQuery map[string]interface{}

func (q *tElasticTermQuery) FieldValue() *tElasticFieldValue {
	return (*tElasticFieldValue)(q)
}
func elasticTermQuery(field string, value string, boost float64) *tElasticTermQuery {
	return &tElasticTermQuery{field: tElasticFieldValue{"value": value, "boost": boost}}
}

/*
Terms Query
https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-term-query.html
*/
type tElasticTermsQuery map[string]interface{}

func (q *tElasticTermsQuery) FieldValue() *tElasticFieldValue {
	return (*tElasticFieldValue)(q)
}
func elasticTermsQuery(field string, boost float64, value ...string) *tElasticTermsQuery {
	return &tElasticTermsQuery{field: value, "boost": boost}
}

/*
Boosting Query
https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-boosting-query.html
*/
type tElasticBoostingQuery map[string]interface{}

func (q *tElasticBoostingQuery) FieldValue() *tElasticFieldValue {
	return (*tElasticFieldValue)(q)
}
func elasticBoostingQuery(positive, negative *tElasticFieldValue, negativeBoost float64) *tElasticBoostingQuery {
	return &tElasticBoostingQuery{
		"positive":       positive,
		"negative":       negative,
		"negative_boost": negativeBoost,
	}
}

/*
Disjunction Max Query
https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-dis-max-query.html
*/
type tElasticDisMaxQuery map[string]interface{}

func (q *tElasticDisMaxQuery) FieldValue() *tElasticFieldValue {
	return (*tElasticFieldValue)(q)
}
func elasticDisMaxQuery(tieBreaker float64, queries ...*tElasticFieldValue) *tElasticDisMaxQuery {
	return &tElasticDisMaxQuery{
		"tie_breaker": tieBreaker,
		"queries":     queries,
	}
}

/*
Constant Score Query
https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-constant-score-query.html#query-dsl-constant-score-query
*/
type tElasticConstantScoreQuery map[string]interface{}

func (q *tElasticConstantScoreQuery) FieldValue() *tElasticFieldValue {
	return (*tElasticFieldValue)(q)
}
func elasticConstantScoreQuery(boost float64, filter ...*tElasticFieldValue) *tElasticConstantScoreQuery {
	return &tElasticConstantScoreQuery{
		"boost":  boost,
		"filter": filter,
	}
}

/*
Boolean Query
https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-bool-query.html
*/
type tElasticBooleanQuery map[string]interface{}

func (q *tElasticBooleanQuery) withShould(val ...*tElasticFieldValue) *tElasticBooleanQuery {
	(*q)["should"] = val
	return q
}
func (q *tElasticBooleanQuery) withMust(val ...*tElasticFieldValue) *tElasticBooleanQuery {
	(*q)["must"] = val
	return q
}
func (q *tElasticBooleanQuery) withMustNot(val ...*tElasticFieldValue) *tElasticBooleanQuery {
	(*q)["must_not"] = val
	return q
}
func (q *tElasticBooleanQuery) withFilter(val ...*tElasticFieldValue) *tElasticBooleanQuery {
	(*q)["filter"] = val
	return q
}
func (q *tElasticBooleanQuery) FieldValue() *tElasticFieldValue {
	return (*tElasticFieldValue)(q)
}
func elasticBooleanQuery(minimumShouldMatch int, boost float64) *tElasticBooleanQuery {
	return &tElasticBooleanQuery{
		"minimum_should_match": minimumShouldMatch,
		"boost":                boost,
	}
}

/*
Match Query
https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-match-query.html
*/
type tElasticMatchQuery map[string]interface{}

func (q *tElasticMatchQuery) withAnalyzer(analyzer string) *tElasticMatchQuery {
	(*q)["analyzer"] = analyzer
	return q
}
func (q *tElasticMatchQuery) withFuzziness(fuzziness string) *tElasticMatchQuery {
	(*q)["fuzziness"] = fuzziness
	return q
}
func (q *tElasticMatchQuery) withOperatorAND() *tElasticMatchQuery {
	(*q)["operator"] = "AND"
	return q
}
func (q *tElasticMatchQuery) withOperatorOR() *tElasticMatchQuery {
	(*q)["operator"] = "OR"
	return q
}
func (q *tElasticMatchQuery) withNoAutoGenerateSynonymsPhraseQuery() *tElasticMatchQuery {
	(*q)["auto_generate_synonyms_phrase_query"] = false
	return q
}
func (q *tElasticMatchQuery) FieldValue() *tElasticFieldValue {
	return &tElasticFieldValue{"match": q}
}
func elasticMatchQuery(field string, query interface{}) *tElasticMatchQuery {
	return &tElasticMatchQuery{
		field: tElasticFieldValue{
			"query": query,
		},
	}
}

/*
Match all query
https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-match-all-query.html
*/
type tElasticMatchAllQuery map[string]interface{}

func (q *tElasticMatchAllQuery) FieldValue() *tElasticFieldValue {
	return &tElasticFieldValue{"match_all": q}
}
func elasticMatchAllQuery(boost float64) *tElasticMatchAllQuery {
	return &tElasticMatchAllQuery{
		"boost": boost,
	}
}

/*
Match None query
https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-match-all-query.html
*/
type tElasticMatchNoneQuery map[string]interface{}

func (q *tElasticMatchNoneQuery) FieldValue() *tElasticFieldValue {
	return &tElasticFieldValue{"match_all": q}
}
func elasticMatchNoneQuery() *tElasticMatchNoneQuery {
	return &tElasticMatchNoneQuery{}
}

/*
Query
https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl.html
*/
type tElasticQuery map[string]interface{}

func (q *tElasticQuery) withBooleanQuery(bq *tElasticBooleanQuery) *tElasticQuery {
	(*q)["bool"] = bq
	return q
}
func (q *tElasticQuery) withConstantScoreQuery(bq *tElasticConstantScoreQuery) *tElasticQuery {
	(*q)["constant_score"] = bq
	return q
}
func (q *tElasticQuery) withDisMaxQuery(bq *tElasticDisMaxQuery) *tElasticQuery {
	(*q)["dis_max"] = bq
	return q
}
func (q *tElasticQuery) withBoostingQuery(bq *tElasticBoostingQuery) *tElasticQuery {
	(*q)["boosting"] = bq
	return q
}
func (q *tElasticQuery) withTermsQuery(bq *tElasticTermsQuery) *tElasticQuery {
	(*q)["terms"] = bq
	return q
}
func (q *tElasticQuery) withTermQuery(bq *tElasticTermQuery) *tElasticQuery {
	(*q)["term"] = bq
	return q
}
func (q *tElasticQuery) withMatchQuery(bq *tElasticMatchQuery) *tElasticQuery {
	(*q)["match"] = bq
	return q
}
func (q *tElasticQuery) withMatchAllQuery(bq *tElasticMatchAllQuery) *tElasticQuery {
	(*q)["match_all"] = bq
	return q
}
func (q *tElasticQuery) FieldValue() *tElasticFieldValue {
	return (*tElasticFieldValue)(q)
}
func elasticQuery() *tElasticQuery {
	return &tElasticQuery{}
}
