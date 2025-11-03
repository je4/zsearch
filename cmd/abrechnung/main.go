package main

import (
	"database/sql"
	"encoding/csv"
	"flag"
	"fmt"

	"emperror.dev/errors"
	_ "github.com/go-sql-driver/mysql"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/je4/zsearch/v2/pkg/mediaserver"
	"github.com/je4/zsearch/v2/pkg/search"
	"github.com/rs/zerolog"

	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const HEIGHT = 150

var mediaserverRegexp = regexp.MustCompile("^mediaserver:([^/]+)/([^/]+)$")

func fmtDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	return fmt.Sprintf("%d:%02d:%02d", h, m, s)
}

func main() {
	var err error
	cfgfile := flag.String("cfg", "./abrechnung.toml", "locations of config file")
	csvFile := flag.String("csv", "", "name of output csvFile file")
	flag.Parse()
	config := LoadConfig(*cfgfile)

	if *csvFile == "" {
		fmt.Printf("no csv file name")
		return
	}

	//	HEIGHT := config.CHeight

	// create logger instance
	_logger := zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()
	logger := zLogger.ZLogger(&_logger)

	mediadb, err := sql.Open(config.Mediaserver.DB.ServerType, string(config.Mediaserver.DB.DSN))
	if err != nil {
		logger.Panic().Msgf(err.Error())
		return
	}
	defer mediadb.Close()
	err = mediadb.Ping()
	if err != nil {
		logger.Panic().Msgf(err.Error())
		return
	}

	ms, err := mediaserver.NewMediaserverMySQL(config.Mediaserver.Url, mediadb, config.Mediaserver.DB.Schema, logger)
	if err != nil {
		logger.Panic().Msgf(err.Error())
		return
	}

	mte, err := search.NewMTElasticSearch(config.ElasticSearch.Endpoint, config.ElasticSearch.Index, config.ElasticSearch.ApiKey.String(), logger)
	if err != nil {
		logger.Panic().Msgf(err.Error())
		return
	}

	//	"category:\"zotero2!!Institut Mode-Design Web\" catalog:\"iid\" catalog:\"Institut_IIG\" catalog:\"Institut_IMD\" catalog:\"hgkfotos\"",
	//	config.Filters
	scrollConfig := &search.ScrollConfig{
		Fields:         nil,
		QStr:           "",
		FiltersFields:  config.Filters,
		Groups:         []string{"global/guest"},
		ContentVisible: false,
		IsAdmin:        true,
	}

	directorTime := map[string]int64{}
	sonstigeTime := map[string]int64{}
	fp, err := os.Create(*csvFile)
	if err != nil {
		logger.Panic().Msgf("cannot create file %s", *csvFile)
	}
	defer fp.Close()
	csvWriter := csv.NewWriter(fp)
	defer csvWriter.Flush()
	csvWriter.Write([]string{
		"Titel",
		"Serienname",
		"ISAN",
		"Spieldauer",
		"Spieldauer (Sekunden)",
		"Sprache",
		"Produktionsland",
		"Regisseur",
		"involvierte Urheber",
		"Produktionsjahr",
		"Filmgenre",
		"Mitwirkende",
		"Enthaltene Musik",
		"Publ. Datum in Mediathek",
		"PID/Handle",
		"Fallgruppe",
		"Sammlungsname",
		"Collection ID Mediaserver",
		"Zotero-Form-ID",
		"Kontaktperson",
		"Bestandsbildung",
	})

	var totalTime int64

	mediaIds := map[string][]string{}

	if err := mte.Scroll(scrollConfig, func(data *search.SourceData) error {
		if data.HasMedia == false {
			return nil
		}
		for mType, mediaList := range data.Media {
			if mType != "video" {
				continue
			}
			for _, media := range mediaList {
				if _, ok := mediaIds[media.Uri]; !ok {
					mediaIds[media.Uri] = []string{data.Signature}
				} else {
					mediaIds[media.Uri] = append(mediaIds[media.Uri], data.Signature)
					continue
				}
				matches := mediaserverRegexp.FindStringSubmatch(media.Uri)
				if matches == nil {
					logger.Error().Msgf("invalid url format: %s", media.Uri)
					return errors.New(fmt.Sprintf("invalid url: %s", media.Uri))
				}
				collection := matches[1]
				signature := matches[2]
				isPublic, err := ms.IsPublic(collection, signature)
				if err != nil {
					return errors.New(fmt.Sprintf("cannot check public flag of %s:%s", collection, signature))
				}
				if !isPublic {
					continue
				}
				logger.Info().Msgf("Loading %s", media.Uri)
				metadata, err := ms.GetMetadata(collection, signature)
				if err != nil {
					return errors.Wrapf(err, "cannot get metadata for %s:%s", collection, signature)
				}
				if metadata.Type != "video" {
					continue
				}

				totalTime += metadata.Duration

				director := []string{}
				sonstige := []string{}
				for _, person := range data.Persons {
					if strings.ToLower(person.Role) == "director" {
						director = append(director, person.Name)
						if _, ok := directorTime[person.Name]; !ok {
							directorTime[person.Name] = 0
						}
						directorTime[person.Name] += metadata.Duration
					} else {
						sonstige = append(sonstige, person.Name)
						if _, ok := sonstigeTime[person.Name]; !ok {
							sonstigeTime[person.Name] = 0
						}
						sonstigeTime[person.Name] += metadata.Duration
					}
				}

				data := []string{
					data.Title.String(), // Titel
					"",                  // Serienname
					"",                  // ISAN
					fmtDuration(time.Duration(metadata.Duration) * time.Second), // Spieldauer
					fmt.Sprintf("%d", metadata.Duration),                        // Spieldauer (Sekunden)
					"",                                                          // Sprache
					data.Place,                                                  // Produktionsland
					strings.Join(director, "; "),                                // Regisseur
					"",                                                          // involvierte Urheber
					data.Date,                                                   // Produktionsjahr
					"",                                                          // Filmgenre
					strings.Join(sonstige, "; "),                                // Mitwirkende
					"",                                                          // Enthaltene Musik
					"",                                                          // Publ. Datum in Mediathek
					"",                                                          // PID/Handle
					"",                                                          // Fallgruppe
					data.CollectionTitle,                                        // Sammlungsname
					fmt.Sprintf("%s:%s", collection, signature), // Collection ID Mediaserver
					data.SignatureOriginal,                      // Zotero-Form-ID
					"",                                          // Kontaktperson
					"",                                          // Bestandsbildung
				}
				if err := csvWriter.Write(data); err != nil {
					logger.Panic().Msgf("cannot write data to csv: %v", err)
				}
			}
		}
		logger.Debug().Msgf("%s", data.Signature)
		return nil
	}); err != nil {
		logger.Panic().Msgf(err.Error())
	}

	name := filepath.Join(filepath.Dir(*csvFile), "director."+filepath.Base(*csvFile))
	directorFp, err := os.Create(name)
	if err != nil {
		logger.Panic().Msgf("cannot create file %s", "director."+*csvFile)
	}
	defer directorFp.Close()
	directorWriter := csv.NewWriter(directorFp)
	defer directorWriter.Flush()
	directorWriter.Write([]string{"Director", "duration"})
	for name, duration := range directorTime {
		directorWriter.Write([]string{name, fmtDuration(time.Duration(duration) * time.Second)})
	}

	name = filepath.Join(filepath.Dir(*csvFile), "sonstige."+filepath.Base(*csvFile))
	sonstigeFp, err := os.Create(name)
	if err != nil {
		logger.Panic().Msgf("cannot create file %s", "sonstige."+*csvFile)
	}
	defer sonstigeFp.Close()
	sonstigeWriter := csv.NewWriter(sonstigeFp)
	defer sonstigeWriter.Flush()
	sonstigeWriter.Write([]string{"sonstige", "duration"})
	for name, duration := range sonstigeTime {
		sonstigeWriter.Write([]string{name, fmtDuration(time.Duration(duration) * time.Second)})
	}

	name = filepath.Join(filepath.Dir(*csvFile), "duplicates."+filepath.Base(*csvFile))
	duplicatesFp, err := os.Create(name)
	if err != nil {
		logger.Panic().Msgf("cannot create file %s", "duplicates."+*csvFile)
	}
	defer duplicatesFp.Close()
	duplicatesWriter := csv.NewWriter(duplicatesFp)
	defer duplicatesWriter.Flush()
	duplicatesWriter.Write([]string{"duplicates", "duration"})
	for name, sigs := range mediaIds {
		if len(sigs) < 2 {
			continue
		}
		duplicatesWriter.Write([]string{name, strings.Join(sigs, "; ")})
	}

	fmt.Printf("Gesamtzeit: %s", fmtDuration(time.Duration(totalTime)*time.Second))

}
