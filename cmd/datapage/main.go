package main

import (
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/mtsolr"
	"log"
)

func main() {

	mts, err := mtsolr.NewMTSolr("http://localhost:8983/solr", "cataloguecoll")
	if err != nil {
		log.Panic(err)
	}

	_, err = mts.LoadEntity("zotero-2486551.9JYAMQE8")
	if err != nil {
		log.Panic(err)
	}
}