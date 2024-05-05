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
	"github.com/araddon/dateparse"
	_ "github.com/go-sql-driver/mysql"
	"github.com/je4/FairService/v2/pkg/fair"
	"github.com/je4/FairService/v2/pkg/fairclient"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/je4/zsearch/v2/pkg/fairservice"
	"github.com/je4/zsearch/v2/pkg/mediaserver"
	"github.com/je4/zsearch/v2/pkg/search"
	"github.com/je4/zsearch/v2/pkg/zsearchclient"
	"github.com/je4/zsync/v2/pkg/filesystem"
	"github.com/je4/zsync/v2/pkg/zotero"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	cfgfile := flag.String("cfg", "./synczotero.toml", "locations of config file")
	sinceFlag := flag.String("since", "1970-01-01T00:00:00", "time of last sync")
	updateAll := flag.Bool("updateAll", false, "timestamps should be ignored")
	syncgroupid := flag.Int64("group", 0, "id of zotero group to sync")
	clear := flag.Bool("clear", false, "clear data")
	syncfair := flag.Bool("fair", false, "sync to fair service")
	synczotero := flag.Bool("zotero", false, "sync zotero cloud")
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

	var archiveStrategy = map[int64]string{}
	for key, val := range config.ArchiveStrategy {
		intKey, err := strconv.ParseInt(key, 10, 64)
		if err != nil {
			logger.Panic().Msgf("cannot parse archive strategy key %s", key)
			return
		}
		archiveStrategy[intKey] = val
	}

	if *syncgroupid > 0 {
		config.Groups = []int64{*syncgroupid}
		config.ClearBeforSync = []int64{}
		config.FairGroups = []int64{}
		if *clear {
			config.ClearBeforSync = append(config.ClearBeforSync, *syncgroupid)
		}
		if *syncfair {
			config.FairGroups = append(config.FairGroups, *syncgroupid)
		}
	}

	since, err := dateparse.ParseAny(*sinceFlag)
	if err != nil {
		logger.Panic().Msgf("cannot parse since parameter %v", *sinceFlag)
		return
	}

	// get database connection handle
	zoteroDB, err := sql.Open(config.Zotero.DB.ServerType, config.Zotero.DB.DSN)
	if err != nil {
		logger.Panic().Err(err)
		return
	}
	defer zoteroDB.Close()

	// Open doesn't open a connection. Validate DSN data:
	err = zoteroDB.Ping()
	if err != nil {
		logger.Panic().Err(err)
		return
	}

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

	fs, err := filesystem.NewS3Fs(
		config.S3.Endpoint,
		config.S3.AccessKeyId,
		config.S3.SecretAccessKey,
		config.S3.UseSSL)
	if err != nil {
		logger.Panic().Msgf("cannot connect to s3 instance: %v", err)
		return
	}

	fservice, err := fairclient.NewFairService(
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

	zot, err := zotero.NewZotero(
		config.Zotero.Endpoint,
		config.Zotero.Apikey,
		zoteroDB,
		fs,
		config.Zotero.DB.Schema,
		false,
		logger,
		false)
	if err != nil {
		logger.Panic().Msgf("cannot create zotero instance: %v", err)
		return
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
		logger.Panic().Msgf("cannot create zsearch zsearchclient: %v", err)
		return
	}
	if err := zsClient.Ping(); err != nil {
		logger.Panic().Msgf("cannot ping zsearch zsearchclient: %v", err)
		return
	}

	logger.Info().Msgf("current key: %v", zot.CurrentKey)
	groupVersions, err := zot.GetUserGroupVersions(zot.CurrentKey)
	if err != nil {
		logger.Error().Msgf("cannot get group versions: %v", err)
		return
	}
	logger.Info().Msgf("group versions: %#v", groupVersions)

	for _, groupId := range config.Groups {
		group, err := zot.LoadGroupLocal(groupId)
		if err != nil {
			logger.Error().Msgf("cannot load groups: %v", err)
			break
		}
		//since := time.Date(1970, 01, 01, 0, 0, 0, 0, time.Local)
		var doClear = false
		for _, cleargroupid := range config.ClearBeforSync {
			if cleargroupid == groupId {
				doClear = true
				break
			}
		}

		if *synczotero {
			group, err := zot.LoadGroupLocal(groupId)
			if err != nil {
				logger.Error().Msgf("cannot load group local %v: %v", groupId, err)
				return
			}
			if !group.Active {
				logger.Info().Msgf("ignoring inactive group #%v", group.Id)
				continue
			}
			if doClear {
				if err := group.ClearLocal(); err != nil {
					logger.Error().Msgf("cannot clear group %v: %v", groupId, err)
					return
				}
			}
			if err := group.Sync(); err != nil {
				logger.Error().Msgf("cannot sync group #%v: %v%+v", group.Id, err, GetErrorStacktrace(err)) // top two frames

				return
			}
			version, ok := (*groupVersions)[groupId]
			if !ok {
				logger.Error().Msgf("no group version for group %v", group.Id)
			}
			// store new group data if necessary
			logger.Info().Msgf("group %v[%v <-> %v]", groupId, group.Version, version)
			// check whether version is newer online...
			if group.Version < version ||
				group.Deleted ||
				group.IsModified {
				newGroup, err := zot.GetGroupCloud(groupId)
				if err != nil {
					logger.Error().Msgf("cannot get group %v: %v", groupId, err)
					return
				}
				newGroup.CollectionVersion = group.CollectionVersion
				newGroup.ItemVersion = group.ItemVersion
				newGroup.TagVersion = group.TagVersion
				newGroup.Deleted = group.Deleted

				logger.Info().Msgf("group %v[%v]", groupId, version)
				if err := newGroup.UpdateLocal(); err != nil {
					logger.Error().Msgf("cannot update group %v: %v", groupId, err)
					return
				}
			}
		}
		/*
			if err := zot.DeleteUnknownGroupsLocal(groupIds); err != nil {
				logger.Error().Msgf("cannot delete unknown groups: %v", err)
			}
		*/

		if doClear {
			since = time.Date(1970, 01, 01, 0, 0, 0, 0, time.Local)
			sPrefix := fmt.Sprintf("zotero2-%v.", groupId)
			num, err := zsClient.SignaturesClear(sPrefix)
			//					num, err := mte.Delete(cfg)
			if err != nil {
				logger.Error().Msgf("cannot delete items with signature prefix %s: %v", groupId, sPrefix, err)
				break
			}
			logger.Info().Msgf("%v items with signature prefix %s deleted", num, sPrefix)
		} else {
			if *updateAll {
				since = time.Date(1970, 01, 01, 0, 0, 0, 0, time.Local)
			} else {
				since, err = zsClient.LastUpdate(fmt.Sprintf("zotero2-%v.", groupId))
				if err != nil {
					logger.Error().Msgf("cannot get last update of group #%v: %v", groupId, err)
					break
				}
			}
		}
		var doFair = false
		for _, fairgroupid := range config.FairGroups {
			if fairgroupid == groupId {
				doFair = true
				break
			}
		}
		if doFair && fservice == nil {
			logger.Panic().Msg("no fair service configured")
		}

		srcPrefix := fmt.Sprintf("zotero2-%v", group.Id)
		if doFair {
			src := &fair.Source{
				ID:          0,
				Name:        fmt.Sprintf("zotero2-%v", group.Id),
				DetailURL:   "https://mediathek.hgk.fhnw.ch/amp/detail/{signature}/plain",
				Description: group.Data.Name,
				OAIDomain:   "dataverse.hgk.fhnw.ch",
				Partition:   "mediathek",
			}
			if err := fservice.SetSource(src); err != nil {
				logger.Panic().Msgf("cannot set source %#v: %v", src, err)
			}
			if doClear || *updateAll {
				if err := fservice.StartUpdate(srcPrefix); err != nil {
					logger.Panic().Msgf("cannot start fairservice update: %v", err)
				}
			}
		}

		counter := 0
		// starting update
		if err := group.IterateItemsAllLocal(
			&since,
			func(item *zotero.Item) error {
				counter++
				logger.Info().Msgf("#%v item: %v.%v", counter, item.Group.Id, item.Key)
				if item.Deleted || item.Trashed {
					return nil
				}
				if item.Data.ParentItem != "" {
					return nil
				}
				if strings.TrimSpace(item.Data.Title) == "" && len(item.Data.Creators) == 0 {
					return nil
				}
				_type, err := item.GetType()
				if err != nil {
					return errors.Wrapf(err, "cannot get item type")
				}
				if _type == "attachment" {
					return nil
				}
				i, err := search.NewSourceData(search.NewZoteroItem(*item, ms))
				if err != nil {
					return errors.Wrap(err, "cannot create source item")
				}

				// create fair entry if necessary
				if doFair {
					fItem := fairservice.SourceToFairItem(i)
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
					i.AddIdentifiers(identifiers)
					rawOriginal, err := json.Marshal(i)
					if err != nil {
						return errors.Wrapf(err, "cannot marshal zotero item")
					}
					if err := fservice.WriteOriginalData(fairItem, rawOriginal); err != nil {
						return errors.Wrapf(err, "cannot write original data to fair service")
					}
					switch archiveStrategy[group.Id] {
					case "single":
						archiveName := fmt.Sprintf("%s", i.GetSignature())
						if err := fservice.AddArchive(archiveName, i.GetAbstract().String()); err != nil {
							return errors.Wrapf(err, "cannot create archive %s", archiveName)
						}
						if err := fservice.AddArchiveItem(archiveName, fairItem.UUID); err != nil {
							return errors.Wrapf(err, "cannot add item %s to archive %s", i.GetSignature(), archiveName)
						}
					}
				}
				if err := zsClient.SignatureCreate(i); err != nil {
					return errors.Wrapf(err, "cannot update item")
				}
				/*
					if err := mte.UpdateTimestamp(i, now); err != nil {
						return errors.Wrapf(err, "cannot update item")
					}
				*/
				counter++
				return nil
			},
		); err != nil {
			logger.Error().Msgf("error getting items: %v", err)
			if doFair {
				if err := fservice.AbortUpdate(srcPrefix); err != nil {
					logger.Panic().Msgf("cannot abort fairservice update: %v", err)
				}
			}
		}
		if doFair {
			if doClear || *updateAll {
				if err := fservice.EndUpdate(srcPrefix); err != nil {
					logger.Panic().Msgf("cannot end fairservice update: %v", err)
				}
			}
		}
	}
	if err := zsClient.ClearCache(); err != nil {
		logger.Error().Msgf("cannot clear cache: %v", err)
	}
	if err := zsClient.BuildSitemap(); err != nil {
		logger.Panic().Err(err)
	}
}
