package main

import (
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/je4/zsearch/v2/pkg/search"
	"github.com/je4/zsearch/v2/pkg/zsearchclient"
	"github.com/rs/zerolog"
	"io"
	"os"
	"time"
)

/*
servicename = "ZSearch"
#address = "https://localhost:8443/api"
address = "https://mediathek.hgk.fhnw.ch/amp/api"
certskipverify = true
#jwtkey = "geheim"
jwtkey = "io5poi24ajoai36fjoaej2fe4aoij"
jwtalg = "HS512"
*/
func main() {
	var out io.Writer = os.Stdout

	output := zerolog.ConsoleWriter{Out: out, TimeFormat: time.RFC3339}
	_logger := zerolog.New(output).With().Timestamp().Logger()
	_logger.Level(zLogger.LogLevel("DEBUG"))
	var logger zLogger.ZLogger = &_logger

	zsClient, err := zsearchclient.NewZSearchClient(
		"ZSearch",
		"https://mediathek.hgk.fhnw.ch/amp/api",
		//"https://localhost:8443/api",
		"io5poi24ajoai36fjoaej2fe4aoij",
		"HS512",
		false,
		true,
		30*time.Second,
		logger)
	if err != nil {
		panic(err)
	}
	if err := zsClient.Ping(); err != nil {
		panic(err)
		return
	}
	data := &search.SourceData{
		Source: "test",
	}
	if err := zsClient.SignatureCreate(data); err != nil {
		panic(err)
	}

}
