/*
Copyright 2020 Center for Digital Matter HGK FHNW, Basel.
Copyright 2020 info-age GmbH, Basel.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS-IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"database/sql"
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/je4/elasticdsl/v2/pkg/bulk"
	"github.com/je4/zsearch/v2/pkg/search"
	"os"
	"time"
)

func main() {
	var err error
	cfgfile := flag.String("cfg", "./search.toml", "locations of config file")
	flag.Parse()
	config := LoadConfig(*cfgfile)

	// create logger instance
	logger, lf := search.CreateLogger("zsearch", config.Logfile, config.Loglevel)
	defer lf.Close()

	var target bulk.Indexer
	if config.Target.V8 {
		target, err = bulk.NewIndexer8(config.Target.Endpoint, config.Target.Index, config.Target.APIKey, logger)
		if err != nil {
			logger.Fatalf("cannot initialize elastic client")
		}
	} else {
		target, err = bulk.NewIndexer7(config.Target.Endpoint, config.Target.Index, logger)
		if err != nil {
			logger.Fatalf("cannot initialize elastic client")
		}
	}
	if config.Target.IndexCreate != "" {
		logger.Infof("loading schema '%s'", config.Target.IndexCreate)
		schemabytes, err := os.ReadFile(config.Target.IndexCreate)
		if err != nil {
			logger.Fatalf("cannot read schema %s", config.Target.IndexCreate)
		}
		if err := target.CreateIndex(schemabytes); err != nil {
			logger.Fatalf("Cannot delete index: %s", err)
		}
	}

	db, err := sql.Open("mysql", config.MediaserverDSN)
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	if err := target.StartBulk(3, int(5e+6), 30*time.Second); err != nil {
		logger.Fatalf("cannot start bulk indexing: %v", err)
	}
	var counter int64
	rows, err := db.Query("SELECT collection_name, signature, type, masterid, parentid, storage, mimetype, filesize, width, height, duration FROM fullcachewithurl")
	if err != nil {
		logger.Fatalf("cannot query mediaserver: %v", err)
	}
	defer rows.Close()

	var collectionName string
	var signature string
	var mType string
	var masterid int
	var parentid sql.NullInt64
	var storage string
	var mimetype string
	var filesize int
	var width int
	var height int
	var dur int

	for rows.Next() {
		counter++
		if err := rows.Scan(
			&collectionName,
			&signature,
			&mType,
			&masterid,
			&parentid,
			&storage,
			&mimetype,
			&filesize,
			&width,
			&height,
			&dur,
		); err != nil {
			logger.Fatalf("cannot get values: %v", err)
		}
		sig := fmt.Sprintf("%v/%v", collectionName, signature)
		fmt.Printf("%05d Signature: %s\n", counter, sig)
		var vals = map[string]any{
			"collectionName": collectionName,
			"signature":      signature,
			"type":           mType,
			"masterid":       masterid,
			"parentid":       parentid.Int64,
			"storage":        storage,
			"mimetype":       mimetype,
			"filesize":       filesize,
			"width":          width,
			"height":         height,
			"duration":       dur,
		}
		if err := target.Index(sig, vals); err != nil {
			logger.Fatalf("cannot index signature: %s", sig)
		}
	}
	target.CloseBulk()
}
