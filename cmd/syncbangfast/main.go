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
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/Masterminds/sprig"
	"github.com/dgraph-io/badger/v4"
	"github.com/gosimple/slug"
	"github.com/je4/FairService/v2/pkg/fair"
	"github.com/je4/FairService/v2/pkg/fairclient"
	"github.com/je4/utils/v2/pkg/openai"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/je4/zsearch/v2/cmd/locales"
	"github.com/je4/zsearch/v2/pkg/apply"
	"github.com/je4/zsearch/v2/pkg/fairservice"
	"github.com/je4/zsearch/v2/pkg/mediaserver"
	"github.com/je4/zsearch/v2/pkg/search"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/rs/zerolog"
	"io"
	"io/fs"
	"log"
	"os"

	"github.com/je4/zsearch/v2/pkg/translate"
	"github.com/je4/zsearch/v2/pkg/zsearchclient"
	"github.com/pkg/errors"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
	"strings"
	"text/template"
	"time"
)

//go:embed embedding.gotmpl
var embeddingTemplate string

var doFair = false

var cfgfile = flag.String("cfg", "./syncbang.toml", "locations of config file")
var clear = flag.Bool("clear", false, "clear all data")

var languageNamer = map[string]display.Namer{
	"de": display.German.Tags(),
	"en": display.English.Tags(),
	"fr": display.French.Tags(),
	"it": display.Italian.Tags(),
}

func funcMap(bundle *i18n.Bundle) template.FuncMap {
	fm := sprig.FuncMap()

	fm["langName"] = func(langSrc, langTarget string) string {
		if namer, ok := languageNamer[langTarget]; ok {
			return namer.Name(language.MustParse(langSrc))
		}
		return langSrc
	}

	fm["localize"] = func(key, lang string) string {
		localizer := i18n.NewLocalizer(bundle, lang)

		result, err := localizer.LocalizeMessage(&i18n.Message{
			ID: key,
		})
		if err != nil {
			return key
			// return fmt.Sprintf("cannot localize '%s': %v", key, err)
		}
		return result // fmt.Sprintf("%s (%s)", result, lang)
	}
	fm["slug"] = func(s string, lang string) string {
		return strings.Replace(slug.MakeLang(s, lang), "-", "_", -1)
	}

	type size struct {
		Width  int64 `json:"width"`
		Height int64 `json:"height"`
	}
	fm["calcAspectSize"] = func(width, height, maxWidth, maxHeight int64) size {
		aspect := float64(width) / float64(height)
		maxAspect := float64(maxWidth) / float64(maxHeight)
		if aspect > maxAspect {
			return size{
				Width:  maxWidth,
				Height: int64(float64(maxWidth) / aspect),
			}
		} else {
			return size{
				Width:  int64(float64(maxHeight) * aspect),
				Height: maxHeight,
			}
		}
	}
	fm["multiLang"] = func(mf []string) *translate.MultiLangString {
		if len(mf) == 0 {
			return nil
		}
		m := &translate.MultiLangString{}
		if err := m.SetMultiString(mf); err != nil {
			return nil
		}
		return m
	}
	return fm
}

func main() {
	var err error
	flag.Parse()
	config := LoadConfig(*cfgfile)

	// create logger instance
	//	logger, lf := search.CreateLogger("synczotero", config.Logfile, config.Loglevel)
	//	defer lf.Close()

	// create logger instance
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

	mediadb, err := sql.Open(config.Mediaserver.DB.ServerType, config.Mediaserver.DB.DSN)
	if err != nil {
		logger.Panic().Err(err)
		return
	}
	defer mediadb.Close()
	err = mediadb.Ping()
	if err != nil {
		logger.Panic().Err(err)
		return
	}

	ms, err := mediaserver.NewMediaserverMySQL(config.Mediaserver.Url, mediadb, config.Mediaserver.DB.Schema, logger)
	if err != nil {
		logger.Panic().Err(err)
		return
	}

	applicationDB, err := sql.Open(config.ApplicationDB.ServerType, config.ApplicationDB.DSN)
	if err != nil {
		logger.Panic().Err(err)
		return
	}
	defer applicationDB.Close()
	err = applicationDB.Ping()
	if err != nil {
		logger.Panic().Err(err)
		return
	}

	badgerDB, err := badger.Open(badger.DefaultOptions(config.TanslateDBPath))
	if err != nil {
		logger.Panic().Err(err)
		return
	}
	defer badgerDB.Close()
	translator := translate.NewDeeplTranslator(string(config.DeeplApiKey), config.DeeplApiUrl, badgerDB, logger)
	kvBadger := openai.NewKVBadger(badgerDB)
	embeddings := openai.NewClientV2(string(config.OpenaiApiKey), kvBadger, logger)

	glang, err := language.Parse(config.Locale.Default)
	if err != nil {
		logger.Panic().Err(err)
		return
	}

	// template for creating embedding
	bundle := i18n.NewBundle(glang)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	for _, lang := range config.Locale.Available {
		localeFile := fmt.Sprintf("active.%s.toml", lang)
		if _, err := fs.Stat(locales.LocaleFS, localeFile); err != nil {
			logger.Panic().Msgf("cannot find locale file [%v] %s", locales.LocaleFS, localeFile)
		}

		if _, err := bundle.LoadMessageFileFS(locales.LocaleFS, localeFile); err != nil {
			logger.Panic().Msgf("cannot load locale file [%v] %s: %v", locales.LocaleFS, localeFile, err)
		}

	}
	tpl, err := template.New("embedding.gotmpl").Funcs(funcMap(bundle)).Parse(embeddingTemplate)
	if err != nil {
		logger.Panic().Err(err)
		return
	}

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
		logger.Panic().Msgf("cannot create zsearch zsearchclient: %v", err)
		return
	}
	if err := zsClient.Ping(); err != nil {
		logger.Panic().Msgf("cannot ping zsearch zsearchclient: %v", err)
		return
	}
	/*
		sPrefix := "bangbang-"
		num, err := zsClient.SignaturesClear(sPrefix)
		logger.Info().Msgf("%v items deleted from INK", num)
		//					num, err := mte.Delete(cfg)
		if err != nil {
			logger.Panic().Msgf("cannot delete items with signature prefix %s: %v", sPrefix, err)
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
			logger.Panic().Msgf("cannot instantiate fair service: %v", err)
		}
		if err := fservice.Ping(); err != nil {
			logger.Panic().Msgf("cannot ping fair service: %v", err)
		}
	}

	app, err := apply.NewApply(logger, applicationDB, config.ApplicationDB.Schema, config.FilePath, ms, "bangbang")
	if err != nil {
		logger.Panic().Err(err)
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
			logger.Panic().Msgf("cannot set source %#v: %v", src, err)
		}
		if err := fservice.StartUpdate(srcPrefix); err != nil {
			logger.Panic().Msgf("cannot start fairservice update: %v", err)
		}
	}

	if clear != nil && *clear {
		if _, err := zsClient.SignaturesClear("bangbang"); err != nil {
			logger.Panic().Msgf("cannot clear signatures with prefix 'bangbang': %v", err)
		}
	}

	if err := app.IterateFormsAll(func(form *apply.Form) error {
		formItems = append(formItems, form)

		// todo: use fair service
		src, err := search.NewSourceData(form)
		if err != nil {
			return errors.Wrap(err, "cannot create sourcedata from iid item")
		}
		logger.Info().Msgf("work %v", src.GetSignature())
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
		src.CreateEmbedding(embeddings, tpl, config.Locale.Available)

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
		logger.Error().Msgf("error iterating works: %v", err)
	}
	if doFair {
		if err := fservice.EndUpdate(srcPrefix); err != nil {
			logger.Panic().Msgf("cannot end fairservice update: %v", err)
		}
	}

	if counter > 0 {
		zsClient.ClearCache()
		if err := zsClient.BuildSitemap(); err != nil {
			logger.Panic().Err(err)
		}
	}

}
