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
	"emperror.dev/errors"
	"flag"
	"fmt"
	"github.com/je4/elasticdsl/v2/pkg/bulk"
	"github.com/je4/zsearch/v2/pkg/search"
	"os"
	"time"
	/*
		"github.com/ampproject/amppackager/packager/certcache"
		"github.com/ampproject/amppackager/packager/certloader"
		"github.com/ampproject/amppackager/packager/healthz"
		"github.com/ampproject/amppackager/packager/mux"
		"github.com/ampproject/amppackager/packager/rtv"
		"github.com/ampproject/amppackager/packager/signer"
		"github.com/ampproject/amppackager/packager/util"
		"github.com/ampproject/amppackager/packager/validitymap"
	*/)

/*
Mehrere PDFs
http://localhost:82/public/zotero-2206003.8JA5VIU9


Mehrere Videos

Viele Unterschiedliche Medien

Sehr viele Bilder
http://localhost:82/public/zotero-2206003.J6NR8ADD
http://localhost:82/public/zotero-2206003.BKPE95CU
http://localhost:82/public/zotero-2206003.PAASB4HF
http://localhost:82/public/zotero-2206003.CVCVRS7J
http://localhost:82/public/zotero-2206003.QWM6R7TT
http://localhost:82/public/zotero-2206003.SSEV6K83


Gedrehte Bilder
http://localhost:82/public/zotero-2206003.NJHHX5MB


Fehlerhafte Darstellung
http://localhost:82/public/zotero-2206003.5JH9WCM9
http://localhost:82/public/zotero-2206003.FG8UDJZG


Mehrere Audio
http://localhost:82/public/zotero-2180340.TPS2WVNX


https://mediathek.hgk.fhnw.ch/amp/detail/zotero-1624911.27CDTZ4B
-->
https://mediathek-hgk-fhnw-ch.cdn.ampproject.org/c/s/mediathek.hgk.fhnw.ch/amp/detail/zotero-1624911.27CDTZ4B
*/

func main() {

	cfgfile := flag.String("cfg", "./search.toml", "locations of config file")
	flag.Parse()
	config := LoadConfig(*cfgfile)

	// create logger instance
	logger, lf := search.CreateLogger("zsearch", config.Logfile, config.Loglevel)
	defer lf.Close()

	stat, err := os.Stat(config.CacheDir)
	if err != nil {
		logger.Panicf("cannot stat %s", config.CacheDir)
		return
	}
	if !stat.IsDir() {
		logger.Panicf("%s not a director", config.CacheDir)
		return
	}

	mtElasticWrapper, err := search.NewMTElasticSearch(config.ElasticSearch.Endpoint, config.ElasticSearch.Index, "", logger)
	if err != nil {
		logger.Panicf("cannot initialize solr search wrapper: %v", err)
		return
	}

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

	if err := target.StartBulk(3, int(5e+6), 30*time.Second); err != nil {
		logger.Fatalf("cannot start bulk indexing: %v", err)
	}
	var counter int64
	mtElasticWrapper.Scroll(&search.ScrollConfig{
		Fields:         nil,
		QStr:           "*",
		FiltersFields:  nil,
		Groups:         nil,
		ContentVisible: false,
		IsAdmin:        false,
	}, func(data *search.SourceData) error {
		counter++
		data.SetStatistics()
		fmt.Printf("%05d Signature: %s\n", counter, data.Signature)
		if err := target.Index(data.Signature, data); err != nil {
			return errors.Wrapf(err, "cannot add signature '%s'", data.Signature)
		}
		return nil
	})
	target.CloseBulk()
}
