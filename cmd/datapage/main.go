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
	badger "github.com/dgraph-io/badger/v4"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/je4/zsearch/v2/pkg/search"
	"github.com/rs/zerolog"
	"google.golang.org/api/customsearch/v1"
	"google.golang.org/api/option"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
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

	var out io.Writer = os.Stdout
	if config.Logfile != "" {
		fp, err := os.OpenFile(config.Logfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			log.Fatalf("cannot open logfile %s: %v", config.Logfile, err)
		}
		defer fp.Close()
		out = fp
	}

	output := zerolog.ConsoleWriter{Out: out, TimeFormat: time.RFC3339}
	_logger := zerolog.New(output).With().Timestamp().Logger()
	_logger.Level(zLogger.LogLevel(config.Loglevel))
	var logger zLogger.ZLogger = &_logger

	var accesslog io.Writer
	if config.AccessLog == "" {
		accesslog = os.Stdout
	} else {
		f, err := os.OpenFile(config.AccessLog, os.O_WRONLY|os.O_CREATE, 0755)
		if err != nil {
			logger.Panic().Msgf("cannot open file %s: %v", config.AccessLog, err)
			return
		}
		defer f.Close()
		accesslog = f
	}

	stat, err := os.Stat(config.CacheDir)
	if err != nil {
		logger.Panic().Msgf("cannot stat %s", config.CacheDir)
		return
	}
	if !stat.IsDir() {
		logger.Panic().Msgf("%s not a director", config.CacheDir)
		return
	}
	if config.ClearCacheOnStartup {
		logger.Info().Msgf("deleting cache files in %s", config.CacheDir)
		if len(config.CacheDir) < 4 {
			logger.Panic().Msgf("%s too short. will not clear cache", config.CacheDir)
			return
		}
		d, err := os.Open(config.CacheDir)
		if err != nil {
			logger.Panic().Msgf("cannot open directory %s", config.CacheDir)
			return
		}
		names, err := d.Readdirnames(-1)
		if err != nil {
			d.Close()
			logger.Panic().Msgf("cannot read %s", config.CacheDir)
			return
		}
		d.Close()
		for _, name := range names {
			fullpath := filepath.Join(config.CacheDir, name)
			logger.Info().Msgf("delete %s", fullpath)
			if err := os.Remove(fullpath); err != nil {
				logger.Panic().Msgf("cannot delete %s", fullpath)
				return
			}
		}
	}
	/*
		if err := os.RemoveAll(config.CacheDir); err != nil {
			logger.Error().Err(err).Msgf("cannot remove %s: %v", config.CacheDir, err)
		}
	*/
	bconfig := badger.DefaultOptions(config.CacheDir)
	if runtime.GOOS == "windows" {
		// bconfig.Truncate = true
	}
	//	bconfig.Logger = logger
	db, err := badger.Open(bconfig)
	if err != nil {
		logger.Panic().Msgf("cannot open badger database: %v", err)
		return
	}
	defer db.Close()

	mtElasticWrapper, err := search.NewMTElasticSearch(config.ElasticSearch.Endpoint, config.ElasticSearch.Index, config.ElasticSearch.ApiKey, logger)
	if err != nil {
		logger.Panic().Msgf("cannot initialize solr search wrapper: %v", err)
		return
	}

	searchEngine, err := search.NewSearch(mtElasticWrapper, config.Solr.CacheSize, config.CacheExpiry.Duration, db, logger)
	if err != nil {
		logger.Panic().Msgf("cannot initialize solr search engine: %v", err)
		return
	}

	uc, err := search.NewUserCache(config.IdleTimeout.Duration, config.UserCacheSize)
	if err != nil {
		logger.Panic().Err(err)
	}

	googleSvc, err := customsearch.NewService(context.Background(), option.WithAPIKey(config.Google.Apikey))
	if err != nil {
		logger.Panic().Err(err)
	}

	facets := search.SolrFacetList{}
	for _, facet := range config.Facets {
		facets[facet.Name] = search.SolrFacet{
			Label:    facet.Name,
			Name:     facet.Name,
			Field:    facet.Field,
			Prefix:   facet.Prefix,
			Restrict: facet.Restrict,
		}
	}

	locations := search.NetGroups{}
	for _, loc := range config.Locations {
		locations[loc.Group] = []*net.IPNet{}
		for _, n := range loc.Networks {
			locations[loc.Group] = append(locations[loc.Group], &n.IPNet)
		}
	}
	subfilters := []search.SubFilter{}
	for _, sf := range config.Query.SubFilter {
		subfilters = append(subfilters, search.SubFilter{
			Name:   sf.Name,
			Label:  sf.Label,
			Filter: sf.Filter,
		})
	}

	var kt = make(map[string]search.KV)
	for k, v := range config.Google.CustomSearchKeys {
		kt[k] = search.KV{Key: v.Key, Name: v.Name}
	}
	srv, err := search.NewServer(
		config.ServiceName,
		searchEngine,
		uc,
		googleSvc,
		config.Template,
		config.TemplateDev,
		config.InstanceName,
		config.Addr,
		config.AddrExt,
		config.Mediaserver,
		config.MediaserverKey,
		config.MediaserverExp.Duration,
		logger,
		accesslog,
		config.Prefixes,
		config.StaticDir,
		config.SitemapDir,
		config.StaticCacheControl,
		config.TemplateDir,
		config.JWTKey,
		config.JWTAlg,
		config.LinkTokenExp.Duration,
		config.SessionTimeout.Duration,
		config.LoginUrl,
		config.LoginIssuer,
		config.AccessGroup.Guest,
		config.AccessGroup.Admin,
		config.AmpCache,
		config.AmpApiKey,
		config.SearchFields,
		facets,
		locations,
		config.Icons,
		config.Query.BaseCatalog,
		subfilters,
		config.CollectionsCatalog,
		config.ClusterCatalog,
		kt,
		config.FacebookAppId,
	)

	if err != nil {
		logger.Error().Err(err).Msgf("error initializing server: %v", err)
		return
	}
	go func() {
		if err := srv.ListenAndServe(config.CertPEM, config.KeyPEM); err != nil {
			logger.Fatal().Msgf("server died: %v", err)
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
		logger.Info().Msgf("shutdown requested")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		srv.Shutdown(ctx)

		end <- true
	}()

	<-end
	logger.Info().Msg("server stopped")
}
