package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	elasticsearch8 "github.com/elastic/go-elasticsearch/v8"
	"github.com/je4/utils/v2/pkg/zLogger"
	esapi7 "github.com/opensearch-project/opensearch-go/opensearchapi"
	"github.com/pkg/errors"
	"io"
	"log"
	"regexp"
	"strings"
	"time"
)

type tElasticFieldValue map[string]interface{}
type tElasticQueryContext map[string]interface{}
type tElasticFilterContext map[string]interface{}
type tElasticFieldValueList []tElasticFieldValue

type tElasticResultErrorCause struct {
	Type   string `json:"type"`
	Reason string `json:"reason"`
	Line   int64  `json:"line"`
	Col    int64  `json:"col"`
}
type tElasticResultError struct {
	RootCause []interface{}            `json:"root_cause"`
	Type      string                   `json:"type"`
	Reason    string                   `json:"reason"`
	CausedBy  tElasticResultErrorCause `json:"caused_by"`
}
type tElasticResultShards struct {
	Total      int64 `json:"total"`
	Successful int64 `json:"successful"`
	Skipped    int64 `json:"skipped"`
	Failed     int64 `json:"failed"`
}

type tElasticResultHitsEntry struct {
	Index     string              `json:"_index"`
	Type      string              `json:"_type"`
	Id        string              `json:"_id"`
	Score     float64             `json:"_score"`
	Source    SourceData          `json:"_source"`
	Highlight map[string][]string `json:"highlight,omitempty"`
}

type tElasticResultHitsTotal struct {
	Value    int64  `json:"value"`
	Relation string `json:"relation"`
}

type tElasticResultHits struct {
	Total    tElasticResultHitsTotal   `json:"total"`
	MaxScore float64                   `json:"max_score"`
	Hits     []tElasticResultHitsEntry `json:"hits"`
}

type tElasticResultAggregationBucket struct {
	Key      string `json:"Key"`
	DocCount int64  `json:"doc_count"`
}

type tElasticResultAggregation struct {
	DocCountErrorUpperBound int64                             `json:"doc_count_error_upper_bound"`
	SumOtherDocCount        int64                             `json:"sum_other_doc_count"`
	Buckets                 []tElasticResultAggregationBucket `json:"buckets,omitempty"`
}
type tElasticResultAggregations map[string]tElasticResultAggregation

type tElasticSearchResult struct {
	Error        tElasticResultError        `json:"error,omitempty"`
	Took         int64                      `json:"took,omitempty"`
	TimedOut     bool                       `json:"timed_out,omitempty"`
	Shards       tElasticResultShards       `json:"_shards,omitempty"`
	Hits         tElasticResultHits         `json:"hits,omitempty"`
	Aggregations tElasticResultAggregations `json:"aggregations,omitempty"`
	Status       float64                    `json:"status"`
	ScrollId     string                     `json:"_scroll_id,omitempty"`
}

type tElasticDeleteResult struct {
	Error                tElasticResultError `json:"error,omitempty"`
	Took                 int64               `json:"took,omitempty"`
	TimedOut             bool                `json:"timed_out,omitempty"`
	Total                int64               `json:"total,omitempty"`
	Deleted              int64               `json:"deleted,omitempty"`
	Batches              int64               `json:"batches,omitempty"`
	VersionConflicts     int64               `json:"version_conflicts,omitempty"`
	Noops                int64               `json:"noops,omitempty"`
	ThrottledMillis      int64               `json:"throttled_millis,omitempty"`
	RequestsPerSecond    float64             `json:"requests_per_second,omitempty"`
	ThrottledUntilMillis int64               `json:"throttled_until_millis,omitempty"`
	Failures             []interface{}       `json:"failures,omitempty"`
}

type tElasticMGetResultDoc struct {
	Index       string     `json:"_index"`
	Type        string     `json:"_type"`
	Id          string     `json:"_id"`
	Version     int64      `json:"_version"`
	SeqNo       int64      `json:"_seq_no"`
	PrimaryTerm int64      `json:"_primary_term"`
	Found       bool       `json:"found"`
	Source      SourceData `json:"_source"`
}

type tElasticMGetResult struct {
	Docs []tElasticMGetResultDoc `json:"docs"`
}

type tElasticSearch struct {
	From           int64                       `json:"from,omitempty"`
	Size           int64                       `json:"size,omitempty"`
	Query          *tElasticQuery              `json:"query"`
	Aggregations   *tElasticSearchAggregations `json:"aggs,omitempty"`
	PostFilter     *tElasticQuery              `json:"post_filter,omitempty"`
	Highlight      *tElasticHighlight          `json:"highlight,omitempty"`
	TrackTotalHits bool                        `json:"track_total_hits,omitempty"`
}

var wordsRegexp = regexp.MustCompile(`([\p{L}\d_]+)+`)

func appendStar(str string) string {
	var ret string
	ret = wordsRegexp.ReplaceAllString(str, `$1*`)
	return ret
}

func (s *tElasticSearch) withTrackTotalHits() *tElasticSearch {
	s.TrackTotalHits = true
	return s
}
func elasticSearch(query *tElasticQuery, aggregations *tElasticSearchAggregations, postfilter *tElasticQuery, highlight *tElasticHighlight, from, size int64) *tElasticSearch {
	return &tElasticSearch{
		From:         from,
		Size:         size,
		Query:        query,
		Aggregations: aggregations,
		PostFilter:   postfilter,
		Highlight:    highlight,
	}
}

type tElasticScroll struct {
	Query *tElasticQuery `json:"query"`
}

func elasticScroll(query *tElasticQuery) *tElasticScroll {
	return &tElasticScroll{
		Query: query,
	}
}

type MTElasticSearch struct {
	es    *elasticsearch8.Client
	index string
	log   zLogger.ZLogger
}

func NewMTElasticSearch(urls []string, index string, apikey string, log zLogger.ZLogger) (*MTElasticSearch, error) {
	es, err := elasticsearch8.NewClient(elasticsearch8.Config{
		APIKey:    apikey,
		Addresses: urls,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create solr interface for %v", urls)
	}
	mte := &MTElasticSearch{
		es:    es,
		index: index,
		log:   log,
	}
	return mte, nil
}

func (mte *MTElasticSearch) Update(source *SourceData) error {
	return mte.UpdateTimestamp(source, time.Now())
}

func (mte *MTElasticSearch) UpdateTimestamp(source *SourceData, timestamp time.Time) error {
	source.Timestamp = timestamp
	jsonStr, err := json.Marshal(source)
	if err != nil {
		return errors.Wrapf(err, "cannot marshal json")
	}
	req := esapi7.IndexRequest{
		Index:      mte.index,
		DocumentID: source.GetSignature(),
		Body:       bytes.NewBuffer(jsonStr),
		Refresh:    "true",
	}
	// Perform the request with the zsearchclient.
	res, err := req.Do(context.Background(), mte.es)
	if err != nil {
		log.Fatalf("Error getting response: %s", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		log.Printf("[%s] Error indexing document ID=%v", res.Status(), req.DocumentID)
		data, _ := io.ReadAll(res.Body)
		return errors.Errorf("[%s] Error indexing document ID=%v: %s", res.Status(), req.DocumentID, string(data))
	} else {
		// Deserialize the response into a map.
		var r map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
			log.Printf("Error parsing the response body: %s", err)
		} else {
			// Print the response status and indexed document version.
			log.Printf("%s/%s - [%s] %s; version=%d", req.Index, req.DocumentID, res.Status(), r["result"], int(r["_version"].(float64)))
		}
	}
	return nil
}

func (mte *MTElasticSearch) LoadDocs(ids []string, ctx context.Context) (map[string]*SourceData, error) {
	jsonstr, err := json.Marshal(struct {
		Ids []string `json:"ids"`
	}{Ids: ids})
	if err != nil {
		return nil, errors.Wrapf(err, "cannot marshal idq's %v", ids)
	}
	buf := bytes.NewBuffer([]byte(jsonstr))
	res, err := mte.es.Mget(
		buf,
		mte.es.Mget.WithIndex(mte.index),
		mte.es.Mget.WithContext(ctx),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot search documents %v", jsonstr)
	}
	defer res.Body.Close()
	var mgresult tElasticMGetResult
	if err := json.NewDecoder(res.Body).Decode(&mgresult); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal response")
	}
	result := make(map[string]*SourceData)
	for _, doc := range mgresult.Docs {
		result[doc.Id] = &doc.Source
	}
	return result, nil
}

func (mte *MTElasticSearch) StatsByACL(catalog []string) (int64, FacetCountResult, error) {
	query := elasticQuery()
	filters := []*tElasticFieldValue{}
	if len(catalog) > 0 {
		for _, c := range catalog {
			filters = append(filters, elasticTermsQuery("catalog.keyword", 0, c).FieldValue())
		}
	}
	bq := elasticBooleanQuery(0)
	if len(filters) > 0 {
		bq.withFilter(filters...)
	}
	query.withBooleanQuery(bq)

	aggregations := elasticSearchAggregations()
	aggregations.AddAggregation("acl.meta", elasticSearchAggregation(nil).withTerms("acl.meta.keyword", 0, nil))
	aggregations.AddAggregation("acl.content", elasticSearchAggregation(nil).withTerms("acl.content.keyword", 0, nil))
	aggregations.AddAggregation("mediatype", elasticSearchAggregation(nil).withTerms("mediatype.keyword", 0, nil))

	fq := elasticSearch(query, aggregations, nil, nil, 0, 0).withTrackTotalHits()

	jsonstr, err := json.Marshal(fq)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "cannot marshal %v", fq)
	}
	mte.log.Debug().Msgf("%v", string(jsonstr))
	buf := bytes.NewBuffer(jsonstr)
	res, err := mte.es.Search(
		mte.es.Search.WithIndex(mte.index),
		mte.es.Search.WithBody(buf),
		mte.es.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "cannot query %v", string(jsonstr))
	}
	defer res.Body.Close()

	var result tElasticSearchResult
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return 0, nil, errors.Wrap(err, "cannot unmarshal result")
	}
	if res.IsError() {
		errstr := fmt.Sprintf(
			"Elastic error: %v - %v at %v:%v",
			result.Error.Type,
			result.Error.Reason,
			result.Error.CausedBy.Line,
			result.Error.CausedBy.Col,
		)
		return 0, nil, fmt.Errorf("%s\n%s", errstr, jsonstr)
	}

	var fcr FacetCountResult = make(FacetCountResult)
	for name, agg := range result.Aggregations {
		fcr[name] = map[string]int{}
		for _, bucket := range agg.Buckets {
			fcr[name][bucket.Key] = int(bucket.DocCount)
		}
	}
	return result.Hits.Total.Value, fcr, nil
}

func (mte *MTElasticSearch) Scroll(cfg *ScrollConfig, callback func(data *SourceData) error) error {
	query := elasticQuery()

	filters := []*tElasticFieldValue{}
	if cfg.IsAdmin == false {
		if len(cfg.Groups) > 0 {
			filters = append(filters, elasticTermsQuery("acl.meta", 0, cfg.Groups...).FieldValue())
		}
	}
	if cfg.ContentVisible {
		if len(cfg.Groups) > 0 && !cfg.IsAdmin {
			filters = append(filters, elasticTermsQuery("acl.content", 0, cfg.Groups...).FieldValue())
		}
		filters = append(filters, elasticExistsQuery("mediatype").FieldValue())
	}

	matchqueries := []*tElasticFieldValue{}
	if len(cfg.FiltersFields) > 0 {
		filters2 := []*tElasticFieldValue{}
		for fld, vals := range cfg.FiltersFields {
			for _, val := range vals {
				switch fld {
				case "category":
					filters2 = append(filters2, elasticPrefixQuery(fld, val).FieldValue())
				case "persons.name":
					/*
						filters = append(filters, elasticNestedQuery("persons",
							elasticQuery().withTermQuery(elasticTermQuery("persons.name.keyword", val, 0))).FieldValue())
					*/
					filters2 = append(filters2,
						elasticNestedQuery("persons", elasticQuery().withBooleanQuery(elasticBooleanQuery(0).withMust(
							elasticSimpleQueryString(val).
								withFields([]string{"persons.name.stem"}).
								withOperatorOR().
								withAnalyzer("digma_stemmer").
								FieldValue()))).FieldValue())
				default:
					filters2 = append(filters2, elasticTermQuery(fld, val, 0).FieldValue())
				}
			}
		}
		if len(filters2) > 0 {
			filters = append(filters, elasticQuery().withBooleanQuery(elasticBooleanQuery(1.0).withShould(1, filters2...)).FieldValue())
		}
	}

	qstr := strings.TrimSpace(cfg.QStr)
	if len(qstr) > 0 {
		matchqueries = append(matchqueries,
			elasticNestedQuery("media.pdf", elasticQuery().withBooleanQuery(elasticBooleanQuery(0).withMust(
				elasticSimpleQueryString(qstr).
					withFields([]string{"media.pdf.fulltext.stem^1"}).
					withOperatorOR().
					withAnalyzer("digma_stemmer").
					withAnalyzeWildcard().
					FieldValue()))).FieldValue())
		matchqueries = append(matchqueries,
			elasticNestedQuery("persons", elasticQuery().withBooleanQuery(elasticBooleanQuery(0).withMust(
				elasticSimpleQueryString(qstr).
					withFields([]string{"persons.name.stem^5"}).
					withOperatorOR().
					withAnalyzer("digma_stemmer").
					withAnalyzeWildcard().
					FieldValue()))).FieldValue())
		matchqueries = append(matchqueries,
			elasticSimpleQueryString(qstr).
				withFields([]string{"title.stem^4", "abstract.stem^3", "notes.stem^3"}).
				withOperatorOR().
				withAnalyzer("digma_stemmer").
				withAnalyzeWildcard().
				FieldValue())
	}
	bq := elasticBooleanQuery(0)
	if len(matchqueries) > 0 {
		bq.withShould(1, matchqueries...)
	}
	if len(filters) > 0 {
		bq.withFilter(filters...)
	}
	query.withBooleanQuery(bq)

	fq := elasticScroll(query)

	// jsonstr, err := json.MarshalIndent(fq, "", "   ")
	jsonstr, err := json.Marshal(fq)
	if err != nil {
		return errors.Wrapf(err, "cannot marshal %v", fq)
	}
	mte.log.Debug().Msgf("%v", string(jsonstr))
	buf := bytes.NewBuffer(jsonstr)
	res, err := mte.es.Search(
		mte.es.Search.WithIndex(mte.index),
		mte.es.Search.WithBody(buf),
		mte.es.Search.WithSize(5000),
		mte.es.Search.WithScroll(12*time.Hour),
	)
	if err != nil {
		return errors.Wrapf(err, "cannot query %v", string(jsonstr))
	}

	for {

		var result tElasticSearchResult
		if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
			res.Body.Close()
			return errors.Wrap(err, "cannot unmarshal result")
		}
		res.Body.Close()
		if res.IsError() {
			errstr := fmt.Sprintf(
				"Elastic error: %v - %v at %v:%v",
				result.Error.Type,
				result.Error.Reason,
				result.Error.CausedBy.Line,
				result.Error.CausedBy.Col,
			)
			return fmt.Errorf("%s\n%s", errstr, jsonstr)
		}

		for _, sd := range result.Hits.Hits {
			if err := callback(&sd.Source); err != nil {
				return errors.Wrapf(err, "error in callback for id %v", sd.Id)
			}
		}

		if len(result.Hits.Hits) == 0 {
			break
		}

		res, err = mte.es.Scroll(
			mte.es.Scroll.WithScrollID(result.ScrollId),
			mte.es.Scroll.WithScroll(1*time.Minute),
		)
		if err != nil {
			return errors.Wrapf(err, "cannot query %v", string(jsonstr))
		}
	}
	return nil
}

func (mte *MTElasticSearch) Search(cfg *SearchConfig) ([]map[string][]string, []*SourceData, int64, FacetCountResult, error) {
	query := elasticQuery()

	filters := []*tElasticFieldValue{}
	if cfg.IsAdmin == false {
		if len(cfg.Groups) > 0 {
			filters = append(filters, elasticTermsQuery("acl.meta.keyword", 0, cfg.Groups...).FieldValue())
		}
	}
	if cfg.ContentVisible {
		if len(cfg.Groups) > 0 && !cfg.IsAdmin {
			filters = append(filters, elasticTermsQuery("acl.content.keyword", 0, cfg.Groups...).FieldValue())
		}
		filters = append(filters, elasticExistsQuery("mediatype.keyword").FieldValue())
	}

	matchqueries := []*tElasticFieldValue{}
	if len(cfg.FiltersFields) > 0 {
		for fld, vals := range cfg.FiltersFields {
			for _, val := range vals {
				switch fld {
				case "category":
					filters = append(filters, elasticPrefixQuery(fld+".keyword", val).FieldValue())
				case "persons.name":
					filters = append(filters, elasticNestedQuery("persons",
						elasticQuery().withTermQuery(elasticTermQuery("persons.name.keyword", val, 0))).FieldValue())
					/*
						filters = append(filters,
							elasticNestedQuery("persons", elasticQuery().withBooleanQuery(elasticBooleanQuery(0).withMust(
								elasticSimpleQueryString(val).
									withFields([]string{"persons.name.stem"}).
									withOperatorOR().
									withAnalyzer("digma_stemmer").
									FieldValue()))).FieldValue())

					*/
				default:
					filters = append(filters, elasticTermQuery(fld /* +".keyword" */, val, 0).FieldValue())
				}
			}
		}
	}

	qstr := strings.TrimSpace(cfg.QStr)
	if len(qstr) > 0 {
		matchqueries = append(matchqueries,
			elasticNestedQuery("media.pdf", elasticQuery().withBooleanQuery(elasticBooleanQuery(0).withMust(
				elasticSimpleQueryString(appendStar(qstr)).
					withFields([]string{"media.pdf.fulltext^1"}).
					withOperatorOR().
					//					withAnalyzer("digma_stemmer").
					withAnalyzeWildcard().
					FieldValue()))).FieldValue())
		matchqueries = append(matchqueries,
			elasticNestedQuery("persons", elasticQuery().withBooleanQuery(elasticBooleanQuery(0).withMust(
				elasticSimpleQueryString(appendStar(qstr)).
					withFields([]string{"persons.name^5"}).
					withOperatorOR().
					//					withAnalyzer("digma_stemmer").
					withAnalyzeWildcard().
					FieldValue()))).FieldValue())
		matchqueries = append(matchqueries,
			elasticSimpleQueryString(appendStar(qstr)).
				withFields([]string{"title^4", "abstract^3", "notes^3"}).
				withOperatorOR().
				//				withAnalyzer("digma_stemmer").
				withAnalyzeWildcard().
				FieldValue())
	}
	bq := elasticBooleanQuery(0)
	if len(matchqueries) > 0 {
		bq.withShould(1, matchqueries...)
	}
	if len(filters) > 0 {
		bq.withFilter(filters...)
	}

	// query.withBooleanQuery(bq)
	boost := elasticBoostingQuery(
		elasticQuery().withBooleanQuery(bq).FieldValue(),
		elasticTermQuery("hasMedia", false, 0.0).FieldValue(),
		0.5,
	)
	query.withBoostingQuery(boost)

	pfterms := []*tElasticFieldValue{}
	var aggregations *tElasticSearchAggregations
	if cfg.Facets != nil {
		aggregations = elasticSearchAggregations()
		for field, vals := range cfg.Facets {
			aggregations.AddAggregation(field, elasticSearchAggregation(nil).withTerms(field+".keyword", vals.Limit, nil))
			values := []string{}
			for val, selected := range vals.Selected {
				if selected {
					values = append(values, val)
				}
			}
			if len(values) > 0 {
				pfterms = append(pfterms, elasticTermsQuery(field /* +".keyword" */, 0, values...).FieldValue())
			}
		}
	}
	var postfilter *tElasticQuery = nil
	if len(pfterms) > 0 {
		postfilter = elasticQuery()
		postfilter.withBooleanQuery(elasticBooleanQuery(1.0).withMust(
			pfterms...,
		))
	}

	var highlight *tElasticHighlight
	if len(matchqueries) > 0 {
		highlight = elasticHighlight().
			withField("abstract", elasticHighlightField()).
			withField("notes", elasticHighlightField()).
			withField("media.pdf.fulltext", elasticHighlightField()).
			withTags([]string{`<span class="highlight">`}, []string{`</span>`})
	}

	fq := elasticSearch(query, aggregations, postfilter, highlight, int64(cfg.Start), int64(cfg.Rows)).withTrackTotalHits()

	// jsonstr, err := json.MarshalIndent(fq, "", "   ")
	jsonstr, err := json.Marshal(fq)
	if err != nil {
		return nil, nil, 0, nil, errors.Wrapf(err, "cannot marshal %v", fq)
	}
	mte.log.Debug().Msgf("%v", string(jsonstr))
	buf := bytes.NewBuffer(jsonstr)
	res, err := mte.es.Search(
		mte.es.Search.WithIndex(mte.index),
		mte.es.Search.WithBody(buf),
		mte.es.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, nil, 0, nil, errors.Wrapf(err, "cannot query %v", string(jsonstr))
	}
	defer res.Body.Close()

	var result tElasticSearchResult
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, nil, 0, nil, errors.Wrap(err, "cannot unmarshal result")
	}
	if res.IsError() {
		errstr := fmt.Sprintf(
			"Elastic error: %v - %v at %v:%v",
			result.Error.Type,
			result.Error.Reason,
			result.Error.CausedBy.Line,
			result.Error.CausedBy.Col,
		)
		return nil, nil, 0, nil, fmt.Errorf("%s\n%s", errstr, jsonstr)
	}

	sdarr := []*SourceData{}
	highlightarr := []map[string][]string{}
	for _, sd := range result.Hits.Hits {
		highlightarr = append(highlightarr, sd.Highlight)
		x := sd.Source
		sdarr = append(sdarr, &x)
	}
	var fcr FacetCountResult = make(FacetCountResult)
	for name, agg := range result.Aggregations {
		fcr[name] = map[string]int{}
		for _, bucket := range agg.Buckets {
			fcr[name][bucket.Key] = int(bucket.DocCount)
		}
	}
	return highlightarr, sdarr, result.Hits.Total.Value, fcr, nil
}

func (mte *MTElasticSearch) LastUpdate(cfg *ScrollConfig) (time.Time, error) {
	var lastUpdate time.Time

	query := elasticQuery()

	filters := []*tElasticFieldValue{}
	if cfg.IsAdmin == false {
		if len(cfg.Groups) > 0 {
			filters = append(filters, elasticTermsQuery("acl.meta", 0, cfg.Groups...).FieldValue())
		}
	}
	if cfg.ContentVisible {
		if len(cfg.Groups) > 0 && !cfg.IsAdmin {
			filters = append(filters, elasticTermsQuery("acl.content", 0, cfg.Groups...).FieldValue())
		}
		filters = append(filters, elasticExistsQuery("mediatype").FieldValue())
	}

	matchqueries := []*tElasticFieldValue{}
	if len(cfg.FiltersFields) > 0 {
		for fld, vals := range cfg.FiltersFields {
			for _, val := range vals {
				switch fld {
				case "category":
					filters = append(filters, elasticPrefixQuery(fld, val).FieldValue())
				case "persons.name":
					filters = append(filters, elasticNestedQuery("persons",
						elasticQuery().withTermQuery(elasticTermQuery("persons.name.stem", val, 0))).FieldValue())
				default:
					if strings.HasSuffix(val, "*") {
						filters = append(filters, elasticPrefixQuery(fld, strings.TrimRight(val, "*")).FieldValue())
					} else {
						filters = append(filters, elasticTermQuery(fld, val, 0).FieldValue())
					}
				}
			}
		}
	}

	qstr := strings.TrimSpace(cfg.QStr)
	if len(qstr) > 0 {
		matchqueries = append(matchqueries,
			elasticNestedQuery("media.pdf", elasticQuery().withBooleanQuery(elasticBooleanQuery(0).withMust(
				elasticSimpleQueryString(qstr).
					withFields([]string{"media.pdf.fulltext.stem^1"}).
					withOperatorOR().
					withAnalyzer("digma_stemmer").
					withAnalyzeWildcard().
					FieldValue()))).FieldValue())
		matchqueries = append(matchqueries,
			elasticNestedQuery("persons", elasticQuery().withBooleanQuery(elasticBooleanQuery(0).withMust(
				elasticSimpleQueryString(qstr).
					withFields([]string{"persons.name.keyword^5", "persons.name.stem^4", "persons.name.snow^4"}).
					withOperatorOR().
					withAnalyzer("digma_stemmer").
					withAnalyzeWildcard().
					FieldValue()))).FieldValue())
		matchqueries = append(matchqueries,
			elasticSimpleQueryString(qstr).
				withFields([]string{"title.stem^4", "abstract.stem^3", "notes.stem^3"}).
				withOperatorOR().
				withAnalyzer("digma_stemmer").
				withAnalyzeWildcard().
				FieldValue())
	}
	bq := elasticBooleanQuery(0)
	if len(matchqueries) > 0 {
		bq.withShould(1, matchqueries...)
	}
	if len(filters) > 0 {
		bq.withFilter(filters...)
	}
	query.withBooleanQuery(bq)

	fq := elasticSearch(query, nil, nil, nil, 0, 1)

	jsonstr, err := json.Marshal(fq)
	if err != nil {
		return lastUpdate, errors.Wrapf(err, "cannot marshal %v", fq)
	}
	mte.log.Debug().Msgf("%v", string(jsonstr))
	buf := bytes.NewBuffer(jsonstr)
	res, err := mte.es.Search(
		mte.es.Search.WithIndex(mte.index),
		mte.es.Search.WithBody(buf),
	)
	if err != nil {
		return lastUpdate, errors.Wrapf(err, "cannot query %v", string(jsonstr))
	}
	defer res.Body.Close()
	var result tElasticSearchResult
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return lastUpdate, errors.Wrap(err, "cannot unmarshal result")
	}
	if res.IsError() {
		errstr := fmt.Sprintf(
			"Elastic error: %v - %v at %v:%v",
			result.Error.Type,
			result.Error.Reason,
			result.Error.CausedBy.Line,
			result.Error.CausedBy.Col,
		)
		return lastUpdate, fmt.Errorf("%s\n%s", errstr, jsonstr)
	}

	if len(result.Hits.Hits) != 1 {
		return lastUpdate, nil
	}
	sd := result.Hits.Hits[0]
	return sd.Source.Timestamp, nil
}

func (mte *MTElasticSearch) Delete(cfg *ScrollConfig) (int64, error) {
	query := elasticQuery()

	filters := []*tElasticFieldValue{}
	if cfg.IsAdmin == false {
		if len(cfg.Groups) > 0 {
			filters = append(filters, elasticTermsQuery("acl.meta", 0, cfg.Groups...).FieldValue())
		}
	}
	if cfg.ContentVisible {
		if len(cfg.Groups) > 0 && !cfg.IsAdmin {
			filters = append(filters, elasticTermsQuery("acl.content", 0, cfg.Groups...).FieldValue())
		}
		filters = append(filters, elasticExistsQuery("mediatype").FieldValue())
	}

	matchqueries := []*tElasticFieldValue{}
	if len(cfg.FiltersFields) > 0 {
		for fld, vals := range cfg.FiltersFields {
			for _, val := range vals {
				switch fld {
				case "category":
					filters = append(filters, elasticPrefixQuery(fld, val).FieldValue())
				case "persons.name":
					filters = append(filters, elasticNestedQuery("persons",
						elasticQuery().withTermQuery(elasticTermQuery("persons.name.keyword", val, 0))).FieldValue())
				default:
					if strings.HasSuffix(val, "*") {
						filters = append(filters, elasticPrefixQuery(fld, strings.TrimRight(val, "*")).FieldValue())
					} else {
						filters = append(filters, elasticTermQuery(fld, val, 0).FieldValue())
					}
				}
			}
		}
	}

	qstr := strings.TrimSpace(cfg.QStr)
	if len(qstr) > 0 {
		matchqueries = append(matchqueries,
			elasticNestedQuery("media.pdf", elasticQuery().withBooleanQuery(elasticBooleanQuery(0).withMust(
				elasticSimpleQueryString(qstr).
					withFields([]string{"media.pdf.fulltext.stem^1"}).
					withOperatorOR().
					withAnalyzer("digma_stemmer").
					withAnalyzeWildcard().
					FieldValue()))).FieldValue())
		matchqueries = append(matchqueries,
			elasticNestedQuery("persons", elasticQuery().withBooleanQuery(elasticBooleanQuery(0).withMust(
				elasticSimpleQueryString(qstr).
					withFields([]string{"persons.name^5", "persons.name.stem^4", "persons.name.snow^4"}).
					withOperatorOR().
					withAnalyzer("digma_stemmer").
					withAnalyzeWildcard().
					FieldValue()))).FieldValue())
		matchqueries = append(matchqueries,
			elasticSimpleQueryString(qstr).
				withFields([]string{"title.stem^4", "abstract.stem^3", "notes.stem^3"}).
				withOperatorOR().
				withAnalyzer("digma_stemmer").
				withAnalyzeWildcard().
				FieldValue())
	}
	bq := elasticBooleanQuery(0)
	if len(matchqueries) > 0 {
		bq.withShould(1, matchqueries...)
	}
	if len(filters) > 0 {
		bq.withFilter(filters...)
	}
	query.withBooleanQuery(bq)

	fq := elasticSearch(query, nil, nil, nil, 0, 0)

	jsonstr, err := json.Marshal(fq)
	if err != nil {
		return 0, errors.Wrapf(err, "cannot marshal %v", fq)
	}
	mte.log.Debug().Msgf("%v", string(jsonstr))
	buf := bytes.NewBuffer(jsonstr)
	dbq, err := mte.es.DeleteByQuery([]string{mte.index}, buf)
	if err != nil {
		return 0, errors.Wrapf(err, "cannot query %v", string(jsonstr))
	}
	defer dbq.Body.Close()
	var result tElasticDeleteResult
	if err := json.NewDecoder(dbq.Body).Decode(&result); err != nil {
		return 0, errors.Wrap(err, "cannot unmarshal result")
	}
	if dbq.IsError() {
		errstr := fmt.Sprintf(
			"Elastic error: %v - %v at %v:%v",
			result.Error.Type,
			result.Error.Reason,
			result.Error.CausedBy.Line,
			result.Error.CausedBy.Col,
		)
		return 0, fmt.Errorf("%s\n%s", errstr, jsonstr)
	}
	return result.Deleted, nil
}
