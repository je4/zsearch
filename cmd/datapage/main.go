package main

import (
	"context"
	"flag"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/mtsolr"
	"gitlab.fhnw.ch/mediathek/search/gsearch/pkg/service"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {

	cfgfile := flag.String("cfg", "./search.toml", "location of config file")
	flag.Parse()
	config := LoadConfig(*cfgfile)

	// create logger instance
	log, lf := service.CreateLogger("memostream", config.Logfile, config.Loglevel)
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

	mts, err := mtsolr.NewMTSolr(config.Solr.Url, config.Solr.Core)
	if err != nil {
		log.Panic(err)
	}

	srv, err := service.NewServer(
		mts,
		config.Template.Detail,
		config.Template.Error,
		config.Addr,
		config.Template.Mediaserver,
		log,
		accesslog,
		config.StaticPrefix,
		config.StaticDir,
		config.PrivatePrefix,
		config.PublicPrefix,
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