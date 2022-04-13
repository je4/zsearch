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
	"github.com/je4/utils/v2/pkg/ssh"
	"github.com/je4/zsearch/v2/pkg/iid"
	"github.com/je4/zsearch/v2/pkg/mediaserver"
	"github.com/je4/zsearch/v2/pkg/search"
	"github.com/je4/zsearch/v2/pkg/zsearchclient"
	"github.com/pkg/errors"
	"time"
)

func main() {
	cfgfile := flag.String("cfg", "./synciid.toml", "locations of config file")
	flag.Parse()
	config := LoadConfig(*cfgfile)

	// create logger instance
	logger, lf := search.CreateLogger("synczotero", config.Logfile, config.Loglevel)
	defer lf.Close()

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

	zsClient, err := zsearchclient.NewZSearchClient(
		config.ZSearchService.ServiceName,
		config.ZSearchService.Address,
		config.ZSearchService.JwtKey,
		config.ZSearchService.JwtAlg,
		config.ZSearchService.CertSkipVerify,
		30*time.Second,
		logger)
	if err != nil {
		logger.Panicf("cannot create zsearch zsearchclient: %v", err)
		return
	}
	if err := zsClient.Ping(); err != nil {
		logger.Panicf("cannot ping zsearch zsearchclient: %v", err)
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

	iidDB, err := sql.Open(config.IIDDB.ServerType, config.IIDDB.DSN)
	if err != nil {
		logger.Panic(err)
		return
	}
	defer iidDB.Close()
	err = iidDB.Ping()
	if err != nil {
		logger.Panic(err)
		return
	}

	IID, err := iid.NewIID(iidDB, config.IIDDB.Schema, config.DataPrefix, ms)
	if err != nil {
		logger.Panic(err)
		return
	}
	defer IID.Close()

	var counter int64 = 0

	if _, err := zsClient.SignaturesClear("iid-"); err != nil {
		logger.Panicf("cannot clear signatures with prefix 'iid-': %v", err)
	}

	//since := time.Date(1970, 01, 01, 0, 0, 0, 0, time.Local)
	// starting update

	if err := IID.IterateModulesAll(func(mod *iid.Module) error {
		// todo: use fair service
		src, err := search.NewSourceData(mod)
		if err != nil {
			return errors.Wrap(err, "cannot create sourcedata from iid item")
		}
		logger.Infof("creating module %v", src.GetSignature())
		if err := zsClient.SignatureCreate(src); err != nil {
			return errors.Wrapf(err, "cannot create module entity")
		}
		counter++
		return nil
	},
	); err != nil {
		logger.Panicf("error iterating modules: %v", err)
	}

	counter = 0
	if err := IID.IterateArbeitenAll(func(a *iid.Arbeit) error {
		counter++
		src, err := search.NewSourceData(a)
		if err != nil {
			return errors.Wrap(err, "cannot create sourcedata from iid item")
		}
		logger.Infof("#%v - creating arbeit %v: %s", counter, src.GetSignature(), src.GetTitle())
		if err := zsClient.SignatureCreate(src); err != nil {
			return errors.Wrapf(err, "cannot create arbeit entity")
		}
		return nil
	},
	); err != nil {
		logger.Panicf("error iterating arbeiten: %v", err)
	}

	if counter > 0 {
		zsClient.ClearCache()
		if err := zsClient.BuildSitemap(); err != nil {
			logger.Panic(err)
		}
	}
}
