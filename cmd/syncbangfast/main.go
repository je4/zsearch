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
	"encoding/json"
	"flag"
	"fmt"
	"github.com/dgraph-io/badger/v4"
	"github.com/je4/FairService/v2/pkg/fair"
	"github.com/je4/FairService/v2/pkg/fairclient"
	"github.com/je4/utils/v2/pkg/ssh"
	"github.com/je4/zsearch/v2/pkg/apply"
	"github.com/je4/zsearch/v2/pkg/fairservice"
	"github.com/je4/zsearch/v2/pkg/mediaserver"
	"github.com/je4/zsearch/v2/pkg/openai"
	"github.com/je4/zsearch/v2/pkg/search"
	"github.com/je4/zsearch/v2/pkg/translate"
	"github.com/je4/zsearch/v2/pkg/zsearchclient"
	"github.com/pkg/errors"
	"golang.org/x/text/language"
	"strings"
	"time"
)

var doFair = false

var cfgfile = flag.String("cfg", "./syncbang.toml", "locations of config file")
var clear = flag.Bool("clear", false, "clear all data")

func main() {
	var err error
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

	applicationDB, err := sql.Open(config.ApplicationDB.ServerType, config.ApplicationDB.DSN)
	if err != nil {
		logger.Panic(err)
		return
	}
	defer applicationDB.Close()
	err = applicationDB.Ping()
	if err != nil {
		logger.Panic(err)
		return
	}

	badgerDB, err := badger.Open(badger.DefaultOptions(config.TanslateDBPath))
	if err != nil {
		logger.Panic(err)
		return
	}
	defer badgerDB.Close()
	translator := translate.NewDeeplTranslator(string(config.DeeplApiKey), config.DeeplApiUrl, badgerDB, logger)
	embeddings := openai.NewClient(config.OpenaiApiUrl, string(config.OpenaiApiKey), badgerDB, logger)

	var zsClient *zsearchclient.ZSearchClient
	zsClient, err = zsearchclient.NewZSearchClient(
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
	/*
		sPrefix := "bangbang-"
		num, err := zsClient.SignaturesClear(sPrefix)
		logger.Infof("%v items deleted from INK", num)
		//					num, err := mte.Delete(cfg)
		if err != nil {
			logger.Panicf("cannot delete items with signature prefix %s: %v", sPrefix, err)
		}
	*/

	var fservice *fairclient.FairClient
	if doFair {
		fservice, err = fairclient.NewFairService(
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
		if err := fservice.Ping(); err != nil {
			logger.Panicf("cannot ping fair service: %v", err)
		}
	}

	app, err := apply.NewApply(logger, applicationDB, config.ApplicationDB.Schema, config.FilePath, ms, "bangbang")
	if err != nil {
		logger.Panic(err)
		return
	}
	defer app.Close()

	var counter int64 = 0

	var items = []*search.SourceData{}
	var formItems = []*apply.Form{}

	srcPrefix := "bangbang"
	if doFair {
		src := &fair.Source{
			ID:          0,
			Name:        srcPrefix,
			DetailURL:   "https://mediathek.hgk.fhnw.ch/amp/detail/{signature}/plain",
			Description: "BangBang",
			OAIDomain:   "dataverse.hgk.fhnw.ch",
			Partition:   "mediathek",
		}
		if err := fservice.SetSource(src); err != nil {
			logger.Panicf("cannot set source %#v: %v", src, err)
		}
		if err := fservice.StartUpdate(srcPrefix); err != nil {
			logger.Panicf("cannot start fairservice update: %v", err)
		}
	}

	if clear != nil && *clear {
		if _, err := zsClient.SignaturesClear("bangbang"); err != nil {
			logger.Panicf("cannot clear signatures with prefix 'bangbang': %v", err)
		}
	}

	if err := app.IterateFormsAll(func(form *apply.Form) error {
		formItems = append(formItems, form)

		// todo: use fair service
		src, err := search.NewSourceData(form)
		if err != nil {
			return errors.Wrap(err, "cannot create sourcedata from iid item")
		}
		logger.Infof("work %v", src.GetSignature())
		if doFair {
			fItem := fairservice.SourceToFairItem(src)
			var fairItem *fair.ItemData
			fairItem, err = fservice.Create(fItem)
			if err != nil {
				return errors.Wrap(err, "cannot create fair entity")
			}
			// add new potential identifiers
			var identifiers = make(map[string]string)
			for _, ident := range fairItem.Identifier {
				parts := strings.SplitN(ident, ":", 2)
				identifiers[parts[0]] = parts[1]
			}
			src.AddIdentifiers(identifiers)
			rawOriginal, err := json.Marshal(src)
			if err != nil {
				return errors.Wrapf(err, "cannot marshal zotero item")
			}
			if err := fservice.WriteOriginalData(fairItem, rawOriginal); err != nil {
				return errors.Wrapf(err, "cannot write original data to fair service")
			}
			archiveName := fmt.Sprintf("%s", src.GetSignature())
			if err := fservice.AddArchive(archiveName, src.GetAbstract().String()); err != nil {
				return errors.Wrapf(err, "cannot create archive %s", archiveName)
			}
			if err := fservice.AddArchiveItem(archiveName, fairItem.UUID); err != nil {
				return errors.Wrapf(err, "cannot add item %s to archive %s", src.GetSignature(), archiveName)
			}
		}
		src.Translate(translator, []language.Tag{language.Italian, language.German, language.English, language.French})
		//		src.ModifyVocabulary()
		_ = embeddings

		if err := zsClient.SignatureCreate(src); err != nil {
			return errors.Wrapf(err, "cannot create work entity")
		}
		counter++
		items = append(items, src)
		/*
			if counter > 500 {
				return errors.New("stop")
			}
		*/
		return nil
	},
	); err != nil {
		logger.Errorf("error iterating works: %v", err)
	}
	if doFair {
		if err := fservice.EndUpdate(srcPrefix); err != nil {
			logger.Panicf("cannot end fairservice update: %v", err)
		}
	}

	if counter > 0 {
		zsClient.ClearCache()
		if err := zsClient.BuildSitemap(); err != nil {
			logger.Panic(err)
		}
	}

}
