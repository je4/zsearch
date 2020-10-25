package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/dgraph-io/badger"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	elasticsearch8 "github.com/elastic/go-elasticsearch/v8"
	"github.com/goph/emperror"
	"github.com/je4/zsearch/pkg/mediaserver"
	"github.com/op/go-logging"
	"log"
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
	Index  string     `json:"_index"`
	Type   string     `json:"_type"`
	Id     string     `json:"_id"`
	Score  float64    `json:"_score"`
	Source SourceData `json:"_source"`
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
	Key      string `json:"key"`
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

type tElasticSearch struct {
	From         int64                       `json:"from"`
	Size         int64                       `json:"size"`
	Query        *tElasticQuery              `json:"query"`
	Aggregations *tElasticSearchAggregations `json:"aggs,omitempty"`
	PostFilter   *tElasticQuery              `json:"post_filter,omitempty"`
}

func elasticSearch(query *tElasticQuery, aggregations *tElasticSearchAggregations, postfilter *tElasticQuery, from, size int64) *tElasticSearch {
	return &tElasticSearch{
		From:         from,
		Size:         size,
		Query:        query,
		Aggregations: aggregations,
		PostFilter:   postfilter,
	}
}

type MTElasticSearch struct {
	es    *elasticsearch8.Client
	index string
	log   *logging.Logger
}

func NewMTElasticSearch(urls []string, index string, db *badger.DB, log *logging.Logger) (*MTElasticSearch, error) {
	es, err := elasticsearch8.NewClient(elasticsearch8.Config{
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
		ids []string `json:"ids"`
	}{ids: ids})
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
	var mapResp map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&mapResp); err != nil {
		return nil, emperror.Wrap(err, "cannot unmarshal response")
	}
	return nil, nil
	/*
		res, err := mte.es.Search(
			mte.es.Search.WithIndex(mte.index),
			mte.es.Search.WithContext(ctx),
			mte.es.Search.WithTrackTotalHits(true),

			)

	*/
}

func (mte *MTElasticSearch) Search(text string, cfg *SearchConfig) ([]*SourceData, int64, FacetCountResult, error) {

	query := elasticQuery()

	filters := []*tElasticFieldValue{}
	if cfg.isAdmin == false {
		filters = append(filters, elasticTermQuery("acl.meta.keyword", cfg.groups[0], 0).FieldValue())
	}
	if cfg.contentVisible {
		filters = append(filters, elasticTermQuery("acl.content.keyword", cfg.groups[0], 0).FieldValue())
		filters = append(filters, elasticExistsQuery("mediatype").FieldValue())
	}
	query.withBooleanQuery(
		elasticBooleanQuery(1.0).withFilter(filters...),
	)
	if text == "" {
		//query.withMatchAllQuery(elasticMatchAllQuery(1.0))
	} else {
		query.withBooleanQuery(
			elasticBooleanQuery(1.0).
				withShould(
					1,
					elasticMatchQuery("title", text).FieldValue(),
					elasticMatchQuery("persons.name", text).FieldValue(),
					elasticMatchQuery("abstract", text).FieldValue(),
					elasticMatchQuery("notes", text).FieldValue(),
				))
	}
	pfterms := []*tElasticFieldValue{}
	aggregations := elasticSearchAggregations()
	for field, vals := range cfg.facets {
		aggregations.AddAggregation(field, elasticSearchAggregation(nil).withTerms(field, vals.limit, nil))
		values := []string{}
		for val, selected := range vals.selected {
			if selected {
				values = append(values, val)
			}
		}
		if len(values) > 0 {
			pfterms = append(pfterms, elasticTermsQuery(field, 0, values...).FieldValue())
		}
	}
	var postfilter *tElasticQuery = nil
	if len(pfterms) > 0 {
		postfilter = elasticQuery()
		postfilter.withBooleanQuery(elasticBooleanQuery(1.0).withMust(
			pfterms...,
		))
	}

	fq := elasticSearch(query, aggregations, postfilter, int64(cfg.start), int64(cfg.rows))
	jsonstr, err := json.MarshalIndent(fq, "", "   ")
	if err != nil {
		return nil, 0, nil, emperror.Wrapf(err, "cannot marshal %v", fq)
	}
	buf := bytes.NewBuffer(jsonstr)
	res, err := mte.es.Search(
		mte.es.Search.WithIndex(mte.index),
		mte.es.Search.WithBody(buf),
		mte.es.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, 0, nil, emperror.Wrapf(err, "cannot query %v", jsonstr)
	}
	defer res.Body.Close()

	/*
		var mapResp map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&mapResp); err != nil {
			return nil, 0, nil, emperror.Wrap(err, "cannot unmarshal result")
		}

	*/

	var result tElasticResult
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, 0, nil, emperror.Wrap(err, "cannot unmarshal result")
	}
	if res.IsError() {
		errstr := fmt.Sprintf(
			"Elastic error: %v - %v at %v:%v",
			result.Error.Type,
			result.Error.Reason,
			result.Error.CausedBy.Line,
			result.Error.CausedBy.Col,
		)
		return nil, 0, nil, fmt.Errorf("%s\n%s", errstr, jsonstr)
	}

	sdarr := []*SourceData{}
	for _, sd := range result.Hits.Hits {
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
	return sdarr, result.Hits.Total.Value, fcr, nil
}
