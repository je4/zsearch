package search

import (
	"bytes"
	"context"
	"encoding/json"
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

/* ***********************************
Queries
*********************************** */

/*
Basic query
*/
func elasticBaseQuery(query tElasticQueryContext, filter tElasticFilterContext) tElasticFieldValue {
	var queryDSL tElasticFieldValue
	if query != nil {
		queryDSL["query"] = query
	}
	if filter != nil {
		queryDSL["filter"] = filter
	}
	return queryDSL
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
	if text == "" {
		query.withMatchAllQuery(elasticMatchAllQuery(1.0))
	} else {
		query.withBooleanQuery(
			elasticBooleanQuery(1, 1.0).
				withShould(
					elasticMatchQuery("title", text).FieldValue(),
					elasticMatchQuery("persons.name", text).FieldValue(),
					elasticMatchQuery("abstract", text).FieldValue(),
					elasticMatchQuery("notes", text).FieldValue(),
				))
	}
	fq := map[string]interface{}{"query": query}
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
	var mapResp map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&mapResp); err != nil {
		return nil, 0, nil, emperror.Wrap(err, "cannot unmarshal result")
	}

	return nil, 0, nil, nil
}
