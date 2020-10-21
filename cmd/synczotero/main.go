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
	"github.com/dgraph-io/badger"
	"github.com/goph/emperror"
	"github.com/je4/zsearch/pkg/mediaserver"
	"github.com/je4/zsearch/pkg/search"
	"github.com/je4/zsync/pkg/filesystem"
	"github.com/je4/zsync/pkg/zotero"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

func main() {

	cfgfile := flag.String("cfg", "./synczotero.toml", "locations of config file")
	flag.Parse()
	config := LoadConfig(*cfgfile)

	// create logger instance
	log, lf := search.CreateLogger("synczotero", config.Logfile, config.Loglevel)
	defer lf.Close()

	stat, err := os.Stat(config.CacheDir)
	if err != nil {
		log.Panicf("cannot stat %s", config.CacheDir)
		return
	}
	if !stat.IsDir() {
		log.Panicf("%s not a director", config.CacheDir)
		return
	}
	if config.ClearCacheOnStartup {
		log.Infof("deleting cache files in %s", config.CacheDir)
		if len(config.CacheDir) < 4 {
			log.Panicf("%s too short. will not clear cache", config.CacheDir)
			return
		}
		d, err := os.Open(config.CacheDir)
		if err != nil {
			log.Panicf("cannot open directory %s", config.CacheDir)
			return
		}
		names, err := d.Readdirnames(-1)
		if err != nil {
			d.Close()
			log.Panicf("cannot read %s", config.CacheDir)
			return
		}
		d.Close()
		for _, name := range names {
			fullpath := filepath.Join(config.CacheDir, name)
			log.Infof("delete %s", fullpath)
			if err := os.Remove(fullpath); err != nil {
				log.Panicf("cannot delete %s", fullpath)
				return
			}
		}
	}

	bconfig := badger.DefaultOptions(config.CacheDir)
	if runtime.GOOS == "windows" {
		bconfig.Truncate = true
	}
	bconfig.Logger = log
	db, err := badger.Open(bconfig)
	if err != nil {
		log.Panicf("cannot open badger database: %v", err)
		return
	}
	defer db.Close()

	mte, err := search.NewMTElasticSearch(config.ElasticSearch.Endpoint, config.ElasticSearch.Index, db, log)
	if err != nil {
		log.Panic(err)
		return
	}

	// get database connection handle
	zoteroDB, err := sql.Open(config.Zotero.DB.ServerType, config.Zotero.DB.DSN)
	if err != nil {
		log.Panic(err)
		return
	}
	defer zoteroDB.Close()

	// Open doesn't open a connection. Validate DSN data:
	err = zoteroDB.Ping()
	if err != nil {
		log.Panic(err)
		return
	}

	mediadb, err := sql.Open(config.Mediaserver.DB.ServerType, config.Mediaserver.DB.DSN)
	if err != nil {
		log.Panic(err)
		return
	}
	defer mediadb.Close()
	err = mediadb.Ping()
	if err != nil {
		log.Panic(err)
		return
	}

	ms, err := mediaserver.NewMediaserverMySQL(config.Mediaserver.Url, mediadb, config.Mediaserver.DB.Schema, log)
	if err != nil {
		log.Panic(err)
		return
	}

	fs, err := filesystem.NewS3Fs(
		config.S3.Endpoint,
		config.S3.AccessKeyId,
		config.S3.SecretAccessKey,
		config.S3.UseSSL)
	if err != nil {
		log.Panicf("cannot connect to s3 instance: %v", err)
		return
	}

	zot, err := zotero.NewZotero(
		config.Zotero.Endpoint,
		config.Zotero.Apikey,
		zoteroDB,
		fs,
		config.Zotero.DB.Schema,
		false,
		log,
		true)
	if err != nil {
		log.Panicf("cannot create zotero instance: %v", err)
		return
	}

	for _, groupid := range config.Groups {
		group, err := zot.LoadGroupLocal(groupid)
		if err != nil {
			log.Errorf("cannot load groups: %v", err)
			break
		}
		since := time.Date(1970, 01, 01, 0, 0, 0, 0, time.Local)
		group.IterateItemsAllLocal(
			&since,
			func(item *zotero.Item) error {
				if item.Data.ParentItem != "" {
					return nil
				}
				_type, err := item.GetType()
				if err != nil {
					return emperror.Wrapf(err, "cannot get item type")
				}
				if _type == "attachment" {
					return nil
				}
				i := (*search.Item)(item)
				if err := mte.Update(i, ms); err != nil {
					return emperror.Wrapf(err, "cannot update item")
				}
				return nil
			},
		)
	}

}
