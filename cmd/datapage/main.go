package main

import (
	"context"
	"flag"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/generic"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/service"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/source"
	"io"
	"os"
	"os/signal"
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

	mts, err := source.NewMTSolr(config.Solr.Url, config.Solr.Core, config.Solr.CacheExpiration.Duration, config.Solr.CacheSize)
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