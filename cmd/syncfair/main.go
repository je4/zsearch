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
	"github.com/dgraph-io/badger/v4"
	"github.com/je4/FairService/v2/pkg/fairclient"
	"github.com/je4/utils/v2/pkg/ssh"
	"github.com/je4/zsearch/v2/pkg/fairservice"
	"github.com/je4/zsearch/v2/pkg/mediaserver"
	"github.com/je4/zsearch/v2/pkg/search"
	"github.com/je4/zsync/v2/pkg/filesystem"
	"github.com/je4/zsync/v2/pkg/zotero"
	"github.com/pkg/errors"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

func main() {
	cfgfile := flag.String("cfg", "./synczotero.toml", "locations of config file")
	sinceFlag := flag.String("since", "1970-01-01T00:00:00", "time of last sync")
	flag.Parse()
	config := LoadConfig(*cfgfile)

	// create logger instance
	logger, lf := search.CreateLogger("synczotero", config.Logfile, config.Loglevel)
	defer lf.Close()

	since, err := dateparse.ParseAny(*sinceFlag)
	if err != nil {
		logger.Panicf("cannot parse since parameter %v", *sinceFlag)
		return
	}

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
	db, err := badger.Open(bconfig)
	if err != nil {
		logger.Panicf("cannot open badger database: %v", err)
		return
	}
	defer db.Close()

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

	// get database connection handle
	zoteroDB, err := sql.Open(config.Zotero.DB.ServerType, config.Zotero.DB.DSN)
	if err != nil {
		logger.Panic(err)
		return
	}
	defer zoteroDB.Close()

	// Open doesn't open a connection. Validate DSN data:
	err = zoteroDB.Ping()
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

	fs, err := filesystem.NewS3Fs(
		config.S3.Endpoint,
		config.S3.AccessKeyId,
		config.S3.SecretAccessKey,
		config.S3.UseSSL)
	if err != nil {
		logger.Panicf("cannot connect to s3 instance: %v", err)
		return
	}

	fair, err := fairclient.NewFairService(
		config.FairService.ServiceName,
		config.FairService.Address,
		config.FairService.CertSkipVerify,
		config.FairService.JwtKey,
		config.FairService.JwtAlg,
		30*time.Second,
	)
	if err != nil {
		logger.Panicf("cannot instantiate fair service: %v", err)
	}

	zot, err := zotero.NewZotero(
		config.Zotero.Endpoint,
		config.Zotero.Apikey,
		zoteroDB,
		fs,
		config.Zotero.DB.Schema,
		false,
		logger,
		true)
	if err != nil {
		logger.Panicf("cannot create zotero instance: %v", err)
		return
	}
	for _, groupid := range config.Groups {
		group, err := zot.LoadGroupLocal(groupid)
		if err != nil {
			logger.Errorf("cannot load groups: %v", err)
			break
		}
		//since := time.Date(1970, 01, 01, 0, 0, 0, 0, time.Local)
		since = time.Date(1970, 01, 01, 0, 0, 0, 0, time.Local)
		source := fmt.Sprintf("zotero2-%v", group.Id)
		// starting update
		if err := fair.StartUpdate(source); err != nil {
			logger.Errorf("cannot start update of source %v: %v", source, err)
			break
		}
		if err := group.IterateItemsAllLocal(
			&since,
			func(item *zotero.Item) error {
				if item.Deleted || item.Trashed {
					return nil
				}
				if item.Data.ParentItem != "" {
					return nil
				}
				_type, err := item.GetType()
				if err != nil {
					return errors.Wrapf(err, "cannot get item type")
				}
				if _type == "attachment" {
					return nil
				}
				zi := search.NewZoteroItem(*item, ms)
				src, err := search.NewSourceData(zi)
				if err != nil {
					return errors.Wrap(err, "cannot create sourcedata from search.ZoteroItem item")
				}
				uuid, err := fair.Create(fairservice.SourceToFairItem(src))
				if err != nil {
					return errors.Wrap(err, "cannot create fair entity")
				}

				logger.Infof("uuid #%s inserted", uuid)
				return nil
			},
		); err != nil {
			logger.Errorf("error iterating items: %v", err)
			// abort has no effect if already ended
			fair.AbortUpdate(source)
			return
		}
		// update deletions
		fair.EndUpdate(source)
	}
}
