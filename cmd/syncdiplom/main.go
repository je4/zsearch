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
	"emperror.dev/emperror"
	"flag"
	"fmt"
	"github.com/dgraph-io/badger/v4"
	_ "github.com/go-sql-driver/mysql"
	"github.com/je4/utils/v2/pkg/ssh"
	"github.com/je4/zsearch/v2/pkg/forms2"
	"github.com/je4/zsearch/v2/pkg/mediaserver"
	"github.com/je4/zsearch/v2/pkg/search"
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
	logger, lf := search.CreateLogger("synczotero", config.Logfile, config.Loglevel)
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
	if config.ClearCacheOnStartup {
		logger.Infof("deleting cache files in %s", config.CacheDir)
		if len(config.CacheDir) < 4 {
			logger.Panicf("%s too short. will not clear cache", config.CacheDir)
			return
		}
		d, err := os.Open(config.CacheDir)
		if err != nil {
			logger.Panicf("cannot open directory %s", config.CacheDir)
			return
		}
		names, err := d.Readdirnames(-1)
		if err != nil {
			d.Close()
			logger.Panicf("cannot read %s", config.CacheDir)
			return
		}
		d.Close()
		for _, name := range names {
			fullpath := filepath.Join(config.CacheDir, name)
			logger.Infof("delete %s", fullpath)
			if err := os.Remove(fullpath); err != nil {
				logger.Panicf("cannot delete %s", fullpath)
				return
			}
		}
	}

	bconfig := badger.DefaultOptions(config.CacheDir)
	if runtime.GOOS == "windows" {
		// bconfig.Truncate = true
	}
	bconfig.Logger = logger
	dbx, err := badger.Open(bconfig)
	if err != nil {
		logger.Panicf("cannot open badger database: %v", err)
		return
	}
	defer dbx.Close()

	var tunnels []*ssh.SSHtunnel
	for name, tunnel := range config.Tunnel {
		logger.Infof("starting tunnel %s", name)

		forwards := make(map[string]*ssh.SourceDestination)
		for fwName, fw := range tunnel.Forward {
			forwards[fwName] = &ssh.SourceDestination{
				Local: &ssh.Endpoint{
					Host: fw.Local.Host,
					Port: fw.Local.Port,
				},
				Remote: &ssh.Endpoint{
					Host: fw.Remote.Host,
					Port: fw.Remote.Port,
				},
			}
		}

		t, err := ssh.NewSSHTunnel(
			tunnel.User,
			tunnel.PrivateKey,
			&ssh.Endpoint{
				Host: tunnel.Endpoint.Host,
				Port: tunnel.Endpoint.Port,
			},
			forwards,
			logger,
		)
		if err != nil {
			logger.Errorf("cannot create tunnel %v@%v:%v - %v", tunnel.User, tunnel.Endpoint.Host, tunnel.Endpoint.Port, err)
			return
		}
		if err := t.Start(); err != nil {
			logger.Errorf("cannot create configfile %v - %v", t.String(), err)
			return
		}
		tunnels = append(tunnels, t)
	}
	defer func() {
		for _, t := range tunnels {
			t.Close()
		}
	}()
	// if tunnels are made, wait until connection is established
	if len(config.Tunnel) > 0 {
		time.Sleep(2 * time.Second)
	}

	mte, err := search.NewMTElasticSearch(config.ElasticSearch.Endpoint, config.ElasticSearch.Index, logger)
	if err != nil {
		logger.Panic(err)
		return
	}

	mediadb, err := sql.Open(config.Mediaserver.DB.ServerType, config.Mediaserver.DB.DSN)
	if err != nil {
		logger.Panic(err)
		return
	}
	defer mediadb.Close()
	err = mediadb.Ping()
	if err != nil {
		logger.Panic(err)
		return
	}

	ms, err := mediaserver.NewMediaserverMySQL(config.Mediaserver.Url, mediadb, config.Mediaserver.DB.Schema, logger)
	if err != nil {
		logger.Panic(err)
		return
	}

	formsdb, err := sql.Open(config.FormsDB.ServerType, config.FormsDB.DSN)
	if err != nil {
		logger.Panic(err)
		return
	}
	defer formsdb.Close()
	err = formsdb.Ping()
	if err != nil {
		logger.Panic(err)
		return
	}
	forms, err := forms2.NewForms2(formsdb, config.FormsDB.Schema, config.DataPrefix, ms)
	if err != nil {
		logger.Panic(err)
		return
	}

	groups, err := forms.GetGroups()
	if err != nil {
		logger.Panicf("cannot get groups: %v", err)
		return
	}

	first := true

	var counter int64 = 0
	for _, groupid := range groups {
		/*
			if groupid != 12017 {
				continue
			}

		*/
		now := time.Now()
		//since := time.Date(1970, 01, 01, 0, 0, 0, 0, time.Local)
		if first {
			cfg := &search.ScrollConfig{
				FiltersFields: map[string][]string{
					"signature": []string{fmt.Sprintf("forms2-%v.*", groupid)},
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
				num, err := mte.Delete(cfg)
				if err != nil {
					logger.Errorf("cannot delete items with signature prefix forms2-%v: %v", groupid, err)
					break
				}
				logger.Infof("%v items with signature prefix forms2-%v deleted", num, groupid)
			} else {
				if err != nil {
					logger.Errorf("cannot get last update of group #%v: %v", groupid, err)
					break
				}
			}
		}
		// starting update
		forms.IterateItemsAll(
			groupid,
			func(item *forms2.Item) error {
				// todo: add fair service
				src, err := search.NewSourceData(item)
				if err != nil {
					return emperror.Wrap(err, "cannot create sourcedata from forms2 item")
				}
				if err := mte.UpdateTimestamp(src, now); err != nil {
					return emperror.Wrapf(err, "cannot update item")
				}
				counter++
				return nil
			},
		)
		first = false
	}
}
