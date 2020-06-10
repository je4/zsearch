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
	"context"
	"flag"
	"github.com/dgraph-io/badger"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/generic"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/service"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/source"
	"io"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

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

	cfgfile := flag.String("cfg", "./search.toml", "location of config file")
	flag.Parse()
	config := LoadConfig(*cfgfile)

	// create logger instance
	log, lf := generic.CreateLogger("memostream", config.Logfile, config.Loglevel)
	defer lf.Close()

	var accesslog io.Writer
	if config.AccessLog == "" {
		accesslog = os.Stdout
	} else {
		f, err := os.OpenFile(config.AccessLog, os.O_WRONLY|os.O_CREATE, 0755)
		if err != nil {
			log.Panicf("cannot open file %s: %v", config.AccessLog, err)
			return
		}
		defer f.Close()
		accesslog = f
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
	/*
		if err := os.RemoveAll(config.CacheDir); err != nil {
			log.Errorf("cannot remove %s: %v", config.CacheDir, err)
		}
	*/
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

	mts, err := source.NewMTSolr(
		config.Solr.Url,
		config.Solr.Core,
		config.Solr.CacheExpiration.Duration,
		config.Solr.CacheSize,
		db,
		log)
	if err != nil {
		log.Panic(err)
	}

	uc, err := service.NewUserCache(config.IdleTimeout.Duration, config.UserCacheSize)
	if err != nil {
		log.Panic(err)
	}

	srv, err := service.NewServer(
		mts,
		uc,
		config.Template.Detail,
		config.Template.Error,
		config.Template.Forbidden,
		config.Template.Search,
		config.Addr,
		config.AddrExt,
		config.Mediaserver,
		config.MediaserverKey,
		config.MediaserverExp.Duration,
		log,
		accesslog,
		config.StaticPrefix,
		config.StaticDir,
		config.JWTKey,
		config.JWTAlg,
		config.LinkTokenExp.Duration,
		config.LoginUrl,
		config.LoginIssuer,
		config.AccessGroup.Guest,
		config.AccessGroup.Admin,
		config.DetailPrefix,
		config.UpdatePrefix,
		config.SearchPrefix,
		config.ApiPrefix,
		config.AmpCache,
		config.AmpApiKey,
		config.SearchFields,
		config.Facets,
	)

	if err != nil {
		log.Errorf("error initializing server: %v", err)
		return
	}
	go func() {
		if err := srv.ListenAndServe(config.CertPEM, config.KeyPEM); err != nil {
			log.Errorf("server died: %v", err)
		}
	}()

	end := make(chan bool, 1)

	// process waiting for interrupt signal (TERM or KILL)
	go func() {
		sigint := make(chan os.Signal, 1)

		// interrupt signal sent from terminal
		signal.Notify(sigint, os.Interrupt)

		signal.Notify(sigint, syscall.SIGTERM)
		signal.Notify(sigint, syscall.SIGKILL)

		<-sigint

		// We received an interrupt signal, shut down.
		log.Infof("shutdown requested")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		srv.Shutdown(ctx)

		end <- true
	}()

	<-end
	log.Info("server stopped")
}
