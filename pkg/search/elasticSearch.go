package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/dgraph-io/badger/v2"
	elasticsearch "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/goph/emperror"
	"github.com/je4/zsearch/pkg/mediaserver"
	"github.com/op/go-logging"
	"log"
	"strings"
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

type tElasticResult struct {
	Error        tElasticResultError        `json:"error,omitempty"`
	Took         int64                      `json:"took,omitempty"`
	TimedOut     bool                       `json:"timed_out,omitempty"`
	Shards       tElasticResultShards       `json:"_shards,omitempty"`
	Hits         tElasticResultHits         `json:"hits,omitempty"`
	Aggregations tElasticResultAggregations `json:"aggregations,omitempty"`
	Status       float64                    `json:"status"`
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
	From           int64                       `json:"from"`
	Size           int64                       `json:"size"`
	Query          *tElasticQuery              `json:"query"`
	Aggregations   *tElasticSearchAggregations `json:"aggs,omitempty"`
	PostFilter     *tElasticQuery              `json:"post_filter,omitempty"`
	Highlight      *tElasticHighlight          `json:"highlight,omitempty"`
	TrackTotalHits bool                        `json:"track_total_hits,omitempty"`
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

type MTElasticSearch struct {
	es    *elasticsearch.Client
	index string
	log   *logging.Logger
}

func NewMTElasticSearch(urls []string, index string, db *badger.DB, log *logging.Logger) (*MTElasticSearch, error) {
	es, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: urls,
	})
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot create solr interface for %v", urls)
	}
	mte := &MTElasticSearch{
		es:    es,
		index: index,
		log:   log,
	}
	return mte, nil
}

func (mte *MTElasticSearch) Update(source Source, ms mediaserver.Mediaserver) error {
	sourceData := InitSourceData(source, ms)
	jsonstr, err := json.Marshal(sourceData)
	if err != nil {
		return emperror.Wrapf(err, "cannot marshal json")
	}
	req := esapi.IndexRequest{
		Index:      mte.index,
		DocumentID: source.GetSignature(),
		Body:       bytes.NewBuffer(jsonstr),
		Refresh:    "true",
	}
	// Perform the request with the client.
	res, err := req.Do(context.Background(), mte.es)
	if err != nil {
		log.Fatalf("Error getting response: %s", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		log.Printf("[%s] Error indexing document ID=%v", res.Status(), req.DocumentID)
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
		return nil, emperror.Wrapf(err, "cannot marshal idq's %v", ids)
	}
	buf := bytes.NewBuffer([]byte(jsonstr))
	res, err := mte.es.Mget(
		buf,
		mte.es.Mget.WithIndex(mte.index),
		mte.es.Mget.WithContext(ctx),
	)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot search documents %v", jsonstr)
	}
	defer res.Body.Close()
	var mgresult tElasticMGetResult
	if err := json.NewDecoder(res.Body).Decode(&mgresult); err != nil {
		return nil, emperror.Wrap(err, "cannot unmarshal response")
	}
	result := make(map[string]*SourceData)
	for _, doc := range mgresult.Docs {
		result[doc.Id] = &doc.Source
	}
	return result, nil
}

func (mte *MTElasticSearch) StatsByACL(catalog string) (int64, FacetCountResult, error) {
	query := elasticQuery()
	filters := []*tElasticFieldValue{}
	if catalog != "" {
		filters = append(filters, elasticTermsQuery("catalog", 0, catalog).FieldValue())
	}
	bq := elasticBooleanQuery(0)
	if len(filters) > 0 {
		bq.withFilter(filters...)
	}
	query.withBooleanQuery(bq)

	aggregations := elasticSearchAggregations()
	aggregations.AddAggregation("acl.meta", elasticSearchAggregation(nil).withTerms("acl.meta", 0, nil))
	aggregations.AddAggregation("acl.content", elasticSearchAggregation(nil).withTerms("acl.content", 0, nil))
	aggregations.AddAggregation("mediatype", elasticSearchAggregation(nil).withTerms("mediatype", 0, nil))

	fq := elasticSearch(query, aggregations, nil, nil, 0, 0).withTrackTotalHits()

	jsonstr, err := json.Marshal(fq)
	if err != nil {
		return 0, nil, emperror.Wrapf(err, "cannot marshal %v", fq)
	}
	mte.log.Debugf("%v", string(jsonstr))
	buf := bytes.NewBuffer(jsonstr)
	res, err := mte.es.Search(
		mte.es.Search.WithIndex(mte.index),
		mte.es.Search.WithBody(buf),
		mte.es.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return 0, nil, emperror.Wrapf(err, "cannot query %v", string(jsonstr))
	}
	defer res.Body.Close()

	var result tElasticResult
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return 0, nil, emperror.Wrap(err, "cannot unmarshal result")
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

func (mte *MTElasticSearch) Search(cfg *SearchConfig) ([]map[string][]string, []*SourceData, int64, FacetCountResult, error) {
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
				case "persons.Name":
					filters = append(filters, elasticNestedQuery("persons",
						elasticQuery().withTermQuery(elasticTermQuery("persons.Name.keyword", val, 0))).FieldValue())
				default:
					filters = append(filters, elasticTermQuery(fld, val, 0).FieldValue())
				}
			}
		}
	}

	qstr := strings.TrimSpace(cfg.QStr)
	if len(qstr) > 0 {
		matchqueries = append(matchqueries,
			elasticNestedQuery("media.pdf", elasticQuery().withBooleanQuery(elasticBooleanQuery(0).withMust(
				elasticSimpleQueryString(qstr).
					withFields([]string{"media.pdf.fulltext^1"}).
					withOperatorOR().
					FieldValue()))).FieldValue())
		matchqueries = append(matchqueries,
			elasticNestedQuery("persons", elasticQuery().withBooleanQuery(elasticBooleanQuery(0).withMust(
				elasticSimpleQueryString(qstr).
					withFields([]string{"persons.Name^5"}).
					withOperatorOR().
					FieldValue()))).FieldValue())
		matchqueries = append(matchqueries,
			elasticSimpleQueryString(qstr).
				withFields([]string{"title^4", "abstract^3", "notes^3"}).
				withOperatorOR().
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

	pfterms := []*tElasticFieldValue{}
	var aggregations *tElasticSearchAggregations
	if cfg.Facets != nil {
		aggregations = elasticSearchAggregations()
		for field, vals := range cfg.Facets {
			aggregations.AddAggregation(field, elasticSearchAggregation(nil).withTerms(field, vals.Limit, nil))
			values := []string{}
			for val, selected := range vals.Selected {
				if selected {
					values = append(values, val)
				}
			}
			if len(values) > 0 {
				pfterms = append(pfterms, elasticTermsQuery(field, 0, values...).FieldValue())
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
		return nil, nil, 0, nil, emperror.Wrapf(err, "cannot marshal %v", fq)
	}
	mte.log.Debugf("%v", string(jsonstr))
	buf := bytes.NewBuffer(jsonstr)
	res, err := mte.es.Search(
		mte.es.Search.WithIndex(mte.index),
		mte.es.Search.WithBody(buf),
		mte.es.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, nil, 0, nil, emperror.Wrapf(err, "cannot query %v", string(jsonstr))
	}
	defer res.Body.Close()

	var result tElasticResult
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, nil, 0, nil, emperror.Wrap(err, "cannot unmarshal result")
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
