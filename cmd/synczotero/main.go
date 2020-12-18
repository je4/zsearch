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
	"github.com/araddon/dateparse"
	"github.com/dgraph-io/badger/v2"
	"github.com/goph/emperror"
	"github.com/je4/sitemap"
	"github.com/je4/zsearch/pkg/mediaserver"
	"github.com/je4/zsearch/pkg/search"
	"github.com/je4/zsync/pkg/filesystem"
	"github.com/je4/zsync/pkg/zotero"
	"github.com/op/go-logging"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

func buildSitemap(mte *search.MTElasticSearch, config *Config, log *logging.Logger) error {
	var size int64 = 3000
	cfg := &search.ScrollConfig{
		FiltersFields:  map[string][]string{},
		QStr:           "",
		Groups:         []string{"global/guest"},
		ContentVisible: true,
		IsAdmin:        false,
	}

	var counter int64 = 0
	var sitemapNo int64 = 0
	var sitemapindex *sitemap.SitemapIndex = sitemap.NewSitemapIndex()
	var sm *sitemap.Sitemap = sitemap.New()

	if err := mte.Scroll(cfg, func(data *search.SourceData) error {
		//		log.Infof("%0.5d - %v", counter, data.Signature)
		if counter%size == 0 {
			if counter > 0 {
				filename := fmt.Sprintf("%s/%s-%05d.xml", config.StaticDir, config.SitemapPrefix, sitemapNo)
				file, err := os.OpenFile(filename, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					return emperror.Wrapf(err, "cannot create file %s", filename)
				}
				if _, err := sm.WriteTo(file); err != nil {
					log.Errorf("cannot write xml to %s: %v", filename, err)
				}
				file.Close()
				log.Infof("%v written", filename)

				lastMod := time.Now()
				u := &sitemap.URL{
					Loc:     fmt.Sprintf("%s/%s/%s-%05d.xml", config.AddrExt, config.Prefixes["static"], config.SitemapPrefix, sitemapNo),
					LastMod: &lastMod,
				}
				sitemapindex.Add(u)
				sitemapNo++
				sm = sitemap.New()
			}
		}
		us := fmt.Sprintf("%s/%s/%s", config.AddrExt, config.Prefixes["detail"], data.Signature)
		u := &sitemap.URL{
			Loc:     us,
			LastMod: &data.Timestamp,
		}
		sm.Add(u)
		counter++
		return nil
	}); err != nil {
		return err
	}
	if len(sm.URLs) > 0 {
		filename := fmt.Sprintf("%s/%s-%05d.xml", config.StaticDir, config.SitemapPrefix, sitemapNo)
		file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return emperror.Wrapf(err, "cannot create file %s", filename)
		}
		if _, err := sm.WriteTo(file); err != nil {
			log.Errorf("cannot write xml to %s: %v", filename, err)
		}
		file.Close()
		log.Infof("%v written", filename)
		lastMod := time.Now()
		u := &sitemap.URL{
			Loc:     fmt.Sprintf("%s/%s/%s-%05d.xml", config.AddrExt, config.Prefixes["static"], config.SitemapPrefix, sitemapNo),
			LastMod: &lastMod,
		}
		sitemapindex.Add(u)
	}

	filename := fmt.Sprintf("%s/%s.xml", config.StaticDir, config.SitemapPrefix)
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return emperror.Wrapf(err, "cannot create file %s", filename)
	}
	sitemapindex.WriteTo(file)
	file.Close()
	log.Infof("%v written", filename)

	return nil
}

func main() {

	cfgfile := flag.String("cfg", "./synczotero.toml", "locations of config file")
	sinceFlag := flag.String("since", "1970-01-01T00:00:00", "time of last sync")
	loop := flag.Bool("loop", false, "true for endless looping")
	flag.Parse()
	config := LoadConfig(*cfgfile)

	// create logger instance
	log, lf := search.CreateLogger("synczotero", config.Logfile, config.Loglevel)
	defer lf.Close()

	since, err := dateparse.ParseAny(*sinceFlag)
	if err != nil {
		log.Panicf("cannot parse since parameter %v", *sinceFlag)
		return
	}

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

	mte, err := search.NewMTElasticSearch(config.ElasticSearch.Endpoint, config.ElasticSearch.Index, log)
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

	first := true

	for {

		var counter int64 = 0
		now := time.Now()
		for _, groupid := range config.Groups {
			group, err := zot.LoadGroupLocal(groupid)
			if err != nil {
				log.Errorf("cannot load groups: %v", err)
				break
			}
			//since := time.Date(1970, 01, 01, 0, 0, 0, 0, time.Local)
			if first {
				cfg := &search.ScrollConfig{
					FiltersFields: map[string][]string{
						"signature": []string{fmt.Sprintf("zotero2-%v.*", groupid)},
					},
					QStr:           "",
					Groups:         []string{},
					ContentVisible: false,
					IsAdmin:        true,
				}

				var doClear = false
				for _, cleargroupid := range config.ClearBeforSync {
					if cleargroupid == groupid {
						doClear = true
						break
					}
				}
				if doClear {
					since = time.Date(1970, 01, 01, 0, 0, 0, 0, time.Local)
					num, err := mte.Delete(cfg)
					if err != nil {
						log.Errorf("cannot delete items with signature prefix zotero2-%v: %v", groupid, err)
						break
					}
					log.Infof("%v items with signature prefix zotero2-%v deleted", num, groupid)
				} else {
					since, err = mte.LastUpdate(cfg)
					if err != nil {
						log.Errorf("cannot get last update of group #%v: %v", groupid, err)
						break
					}
				}
			}
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
					if err := mte.UpdateTimestamp(i, ms, now); err != nil {
						return emperror.Wrapf(err, "cannot update item")
					}
					counter++
					return nil
				},
			)
		}
		if counter > 0 || !*loop {
			if err := buildSitemap(mte, &config, log); err != nil {
				log.Panic(err)
			}
		}
		if !*loop {
			break
		}
		since = now
		log.Infof("sleeping %v", config.Sleep.Duration.String())
		time.Sleep(config.Sleep.Duration)
		first = false
	}
}
