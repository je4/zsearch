package main

import (
	"fmt"
	"github.com/vanng822/go-solr/solr"
	"log"
)

func main() {
	si, err := solr.NewSolrInterface("http://localhost:8983/solr", "cataloguecoll")
	if err != nil {
		log.Panic(err)
	}

	query := solr.NewQuery()
	query.FilterQuery("source:zotero")
	query.Q("*:*")
	s := si.Search(query)
	r, _ := s.Result(nil)
	for _, doc := range r.Results.Docs {
		fmt.Printf("[%v] %v\n", doc["id"], doc["title"])
	}
}