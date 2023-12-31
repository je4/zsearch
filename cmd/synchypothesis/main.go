/*
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
	"flag"
	"github.com/dgraph-io/badger/v4"
	"github.com/je4/zsearch/v2/pkg/hypothesis"
	"github.com/je4/zsearch/v2/pkg/search"
	"os"
	"path/filepath"
	"runtime"
)

func main() {
	cfgfile := flag.String("cfg", "./synczotero.toml", "locations of config file")
	flag.Parse()
	config := LoadConfig(*cfgfile)

	// create logger instance
	log, lf := search.CreateLogger("synchypothesis", config.Logfile, config.Loglevel)
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
		//		bconfig.Truncate = true
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
	log.Infof("%v", mte)

	hy, err := hypothesis.NewHypothesis(config.Hypothesis.Endpoint, config.Hypothesis.Apikey, log)
	if err != nil {
		log.Panicf("cannot create hypothesis instance: %v", err)
		return
	}

	if err := hy.GetGroups(func(grp hypothesis.Group) error {
		log.Infof("Group: #%s - %s", grp.Id, grp.Name)
		return nil
	}); err != nil {
		log.Error("error loading groups")
	}

	if grp, err := hy.GetGroup("RzR53ZGq"); err != nil {
		log.Errorf("cannot get group: %v", err)
	} else {
		log.Infof("Group: #%s - %s", grp.Id, grp.Name)
		if err := grp.GetAnnotations(func(ann hypothesis.Annotation) error {
			log.Infof("Annotation: %v", ann.Uri)
			return nil
		}); err != nil {
			log.Errorf("error getting annotations: %v", err)
		}
	}

	if user, err := hy.GetUser("jenge@hypothes.is"); err != nil {
		log.Errorf("cannot get user: %v", err)
	} else {
		log.Infof("User: %s - %s", user.UserId, user.Username)
		if err := user.GetAnnotations(func(ann hypothesis.Annotation) error {
			log.Infof("Annotation: %v", ann.Uri)
			return nil
		}); err != nil {
			log.Errorf("error getting annotations: %v", err)
		}
	}

}
