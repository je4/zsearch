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
	"bufio"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/blevesearch/bleve/v2"
	"github.com/je4/FairService/v2/pkg/fair"
	"github.com/je4/FairService/v2/pkg/fairclient"
	sdmlcontent "github.com/je4/salon-digital/v2/pkg/content"
	"github.com/je4/utils/v2/pkg/ssh"
	"github.com/je4/zsearch/v2/pkg/apply"
	"github.com/je4/zsearch/v2/pkg/fairservice"
	"github.com/je4/zsearch/v2/pkg/mediaserver"
	"github.com/je4/zsearch/v2/pkg/search"
	"github.com/je4/zsearch/v2/pkg/zsearchclient"
	"github.com/je4/zsync/v2/pkg/filesystem"
	"github.com/je4/zsync/v2/pkg/zotero"
	"github.com/pkg/errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// const doDataUpdateOnly = "c:/temp/Update_20230220.csv"
const doDataUpdateOnly = ""
const doPCB = true
const doFair = true
const doBleve = true
const doZSearch = true
const doWait = false
const doCollage = true
const doCollageOnly = false
const doFullData = true

func main() {
	var err error
	cfgfile := flag.String("cfg", "./syncbang.toml", "locations of config file")
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
	if doDataUpdateOnly != "" {
		if err := correction(applicationDB, doDataUpdateOnly); err != nil {
			fmt.Sprintf("%v", err)
		}

		return
	}
	var zsClient *zsearchclient.ZSearchClient
	if doZSearch {
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
		sPrefix := "bangbang-"
		num, err := zsClient.SignaturesClear(sPrefix)
		logger.Infof("%v items deleted from INK", num)
		//					num, err := mte.Delete(cfg)
		if err != nil {
			logger.Panicf("cannot delete items with signature prefix %s: %v", sPrefix, err)
		}

	}

	// get database connection handle
	zoteroDB, err := sql.Open(config.Zotero.DB.ServerType, config.Zotero.DB.DSN)
	if err != nil {
		logger.Panic(err)
		return
	}
	defer zoteroDB.Close()
	// Open doesn't open a connection. Validate DSN data:
	err = zoteroDB.Ping()
	if err != nil {
		logger.Panic(err)
		return
	}

	fs, err := filesystem.NewS3Fs(
		config.S3.Endpoint,
		config.S3.AccessKeyId,
		config.S3.SecretAccessKey,
		config.S3.UseSSL)
	if err != nil {
		logger.Panicf("cannot connect to s3 instance: %v", err)
		return
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
		logger.Panicf("cannot create zotero instance: %v", err)
		return
	}

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

	if doCollageOnly {
		path := filepath.Join(config.ExportPath, "bangbang.bleve")
		index, err := bleve.Open(path)
		if err != nil {
			logger.Panicf("cannot load bleve index %s: %v", path, err)
		}
		defer index.Close()
		bQuery := bleve.NewMatchAllQuery()
		bSearch := bleve.NewSearchRequest(bQuery)
		var works = []*search.SourceData{}
		bSearch.Size = 100
		for {
			searchResult, err := index.Search(bSearch)
			if err != nil {
				logger.Panicf("cannot load works from index: %v", err)
			}
			for _, val := range searchResult.Hits {
				raw, err := index.GetInternal([]byte(val.ID))
				if err != nil {
					logger.Panicf("cannot get document #%s from index: %v", val.ID, err)
				}
				var src = &search.SourceData{}
				if err := json.Unmarshal(raw, src); err != nil {
					logger.Panicf("cannot unmarshal document #%s: %v", val.ID, err)
				}
				works = append(works, src)
			}
			if len(searchResult.Hits) < 100 {
				break
			}
			bSearch.From += 100
		}
		if err := collage(logger, config.ExportPath, ms, works); err != nil {
			logger.Panic(err)
		}
		return
	}

	var index bleve.Index
	if doBleve {
		path := filepath.Join(config.ExportPath, "bangbang.bleve")
		os.RemoveAll(path)
		mapping := bleve.NewIndexMapping()
		index, err = bleve.New(path, mapping)
	}

	app, err := apply.NewApply(logger, applicationDB, config.ApplicationDB.Schema, config.FilePath, ms, "bangbang")
	if err != nil {
		logger.Panic(err)
		return
	}
	defer app.Close()

	var counter int64 = 0

	/*
		if doZSearch {
				if _, err := zsClient.SignaturesClear("bangbang"); err != nil {
					logger.Panicf("cannot clear signatures with prefix 'bangbang': %v", err)
				}
		}
	*/

	//since := time.Date(1970, 01, 01, 0, 0, 0, 0, time.Local)
	// starting update

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
			if err := fservice.AddArchive(archiveName, src.GetAbstract()); err != nil {
				return errors.Wrapf(err, "cannot create archive %s", archiveName)
			}
			if err := fservice.AddArchiveItem(archiveName, fairItem.UUID); err != nil {
				return errors.Wrapf(err, "cannot add item %s to archive %s", src.GetSignature(), archiveName)
			}
		}
		if doBleve {
			index.Index(src.Signature, src)
			data, err := json.Marshal(src)
			if err != nil {
				return errors.Wrapf(err, "cannot marshal data")
			}
			index.SetInternal([]byte(src.Signature), data)
		}
		if doZSearch {
			if err := zsClient.SignatureCreate(src); err != nil {
				return errors.Wrapf(err, "cannot create work entity")
			}
		}
		counter++
		items = append(items, src)
		return nil
	},
	); err != nil {
		logger.Panicf("error iterating works: %v", err)
	}
	if doFair {
		if err := fservice.EndUpdate(srcPrefix); err != nil {
			logger.Panicf("cannot end fairservice update: %v", err)
		}
	}

	if counter > 0 {
		if doZSearch {
			zsClient.ClearCache()
			if err := zsClient.BuildSitemap(); err != nil {
				logger.Panic(err)
			}
		}
	}

	if doPCB {
		group, err := zot.LoadGroupLocal(1624911)
		if err != nil {
			logger.Panicf("cannot load groups: %v", err)
		}

		since := time.Date(1970, 01, 01, 0, 0, 0, 0, time.Local)
		// starting update
		if err := group.IterateItemsAllLocal(
			&since,
			func(item *zotero.Item) error {

				//return nil // dont do PCB

				counter++
				logger.Infof("#%v item: %v.%v", counter, item.Group.Id, item.Key)
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
				acls := i.GetACL()
				contentACL, ok := acls["content"]
				if !ok {
					logger.Infof("--- no content ACL for #%s", i.GetSignatureOriginal())
					return nil
				}
				aclOK := false
				for _, aclStr := range contentACL {
					if aclStr == "global/guest" {
						aclOK = true
						break
					}
				}
				if !aclOK {
					logger.Infof("--- no public access to content of #%s", i.GetSignatureOriginal())
					return nil
				}
				items = append(items, i)
				if doBleve {
					index.Index(i.Signature, i)
					for key, medias := range i.Media {
						for idx, _ := range medias {
							i.Media[key][idx].Fulltext = ""
						}
					}
					data, err := json.Marshal(i)
					if err != nil {
						return errors.Wrapf(err, "cannot marshal data")
					}
					index.SetInternal([]byte(i.Signature), data)
				}

				counter++
				return nil
			},
		); err != nil {
			logger.Errorf("error getting items: %v", err)
		}
	}
	if doBleve {
		index.Close()
	}

	var persons = map[string][]string{}
	var name = regexp.MustCompile("(^[^, ]+) ([^, ]+)$")
	sqlstr := "SELECT COUNT(*) FROM locked WHERE formid=?"
	sqlstr2 := "INSERT INTO locked(formid) VALUES(?)"
	for _, item := range items {
		var locked int64
		formid, err := strconv.ParseInt(strings.TrimSpace(item.SignatureOriginal), 10, 64)
		if err != nil {
			formid = 0
			//logger.Panicf("cannot parse signatureOriginal of %s: %s // %v", item.Signature, item.SignatureOriginal, err)
		}
		if formid > 0 {
			if err := applicationDB.QueryRow(sqlstr, formid).Scan(&locked); err != nil {
				logger.Panic(err)
			}
		}
		// do not do that....
		if locked == 0 && formid > 0 && false {
			if artists, ok := (*item.Meta)["artists"]; ok {
				matches := name.FindStringSubmatch(strings.TrimSpace(artists))
				if matches != nil {
					newartists := fmt.Sprintf("%s, %s\n", matches[2], matches[1])
					fmt.Printf("%s -> %s", artists, newartists)
					sqlstr := "UPDATE formdata SET value=? WHERE formid=? AND name=?"
					if _, err := applicationDB.Exec(sqlstr, newartists, item.SignatureOriginal, "artists"); err != nil {
						logger.Panic(err)
					}
					(*item.Meta)["artists"] = newartists
				}
			}
			if performers, ok := (*item.Meta)["performers"]; ok {
				matches := name.FindStringSubmatch(strings.TrimSpace(performers))
				if matches != nil {
					newperformers := fmt.Sprintf("%s, %s\n", matches[2], matches[1])
					fmt.Printf("%s -> %s", performers, newperformers)
					sqlstr := "UPDATE formdata SET value=? WHERE formid=? AND name=?"
					if _, err := applicationDB.Exec(sqlstr, newperformers, item.SignatureOriginal, "performers"); err != nil {
						logger.Panic(err)
					}
					(*item.Meta)["performers"] = newperformers
				}
			}
			if camera, ok := (*item.Meta)["camera"]; ok {
				matches := name.FindStringSubmatch(strings.TrimSpace(camera))
				if matches != nil {
					newcamera := fmt.Sprintf("%s, %s\n", matches[2], matches[1])
					fmt.Printf("%s -> %s", camera, newcamera)
					sqlstr := "UPDATE formdata SET value=? WHERE formid=? AND name=?"
					if _, err := applicationDB.Exec(sqlstr, newcamera, item.SignatureOriginal, "camera"); err != nil {
						logger.Panic(err)
					}
					(*item.Meta)["camera"] = newcamera
				}

			}
			// lock it up
			if _, err := applicationDB.Exec(sqlstr2, formid); err != nil {
				logger.Panic(err)
			}
		}
	}

	if err := writeCSV(config.ExportPath, formItems); err != nil {
		logger.Panic(err)
	}
	if err := writePersons(config.ExportPath, formItems); err != nil {
		logger.Panic(err)
	}

	if doWait {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Transcode videos now")
		reader.ReadString('\n')
	}

	if err := writeData(logger, config.Full, config.ListTemplate, config.DetailTemplate, config.TableTemplate, config.ExportPath, ms, items, false); err != nil {
		logger.Panic(err)
	}

	if doCollage {
		if err := collage(logger, config.ExportPath, ms, items); err != nil {
			logger.Panic(err)
		}
	}

	// salon digital
	var cnts = []*sdmlcontent.Content{}
	for _, item := range items {
		cnt := &sdmlcontent.Content{Persons: []sdmlcontent.Person{}, Medias: map[string][]sdmlcontent.Media{}}
		cnt.Title = item.GetTitle()
		cnt.Year = item.GetDate()
		for _, p := range item.GetPersons() {
			cnt.Persons = append(cnt.Persons, sdmlcontent.Person{Name: p.Name, Role: p.Role})
			if _, ok := persons[p.Role]; !ok {
				persons[p.Role] = []string{}
			}
			persons[p.Role] = append(persons[p.Role], p.Name)
		}
		derivatePath := filepath.ToSlash(filepath.Join("werke", item.SignatureOriginal, "derivate"))
		for t, mss := range item.GetMedia() {
			cnt.Medias[t] = []sdmlcontent.Media{}
			for _, m := range mss {
				var thumb, image string
				switch m.Type {
				case "video":
					thumb, err = mediaUrl(
						logger,
						config.ExportPath,
						ms,
						derivatePath,
						"jpg",
						m.Uri+"$$timeshot$$3/resize/formatjpeg/size240x240")
					image, err = mediaUrl(
						logger,
						config.ExportPath,
						ms,
						derivatePath,
						"jpg",
						m.Uri+"$$timeshot$$3/resize/formatjpeg/size1024x768")
				case "audio":
					thumb, err = mediaUrl(
						logger,
						config.ExportPath,
						ms,
						derivatePath,
						"jpg",
						m.Uri+"$$poster/resize/formatjpeg/size240x240")
					image, err = mediaUrl(
						logger,
						config.ExportPath,
						ms,
						derivatePath,
						"jpg",
						m.Uri+"$$poster/resize/formatjpeg/size1024x768")
				case "pdf":
					thumb, err = mediaUrl(
						logger,
						config.ExportPath,
						ms,
						derivatePath,
						"jpg",
						m.Uri+"$$poster/resize/formatjpeg/size240x240")
					image, err = mediaUrl(
						logger,
						config.ExportPath,
						ms,
						derivatePath,
						"jpg",
						m.Uri+"$$poster/resize/formatjpeg/size1024x768")
				case "image":
					thumb, err = mediaUrl(
						logger,
						config.ExportPath,
						ms,
						derivatePath,
						"jpg",
						m.Uri+"/resize/autorotate/formatjpeg/size240x240")
					image, err = mediaUrl(
						logger,
						config.ExportPath,
						ms,
						derivatePath,
						"jpg",
						m.Uri+"/resize/autorotate/formatjpeg/size1024x768")
				}
				if err != nil {
					logger.Panic(err)
				}
				cnt.Medias[t] = append(cnt.Medias[t], sdmlcontent.Media{
					Type:      m.Type,
					Thumbnail: filepath.ToSlash(filepath.Join(derivatePath, thumb)),
					Media:     filepath.ToSlash(filepath.Join(derivatePath, image)),
				})
			}
		}
		cnts = append(cnts, cnt)

	}
	jfile, err := os.OpenFile(filepath.Join(config.ExportPath, "salon.json"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		logger.Panic(err)
	}
	jenc := json.NewEncoder(jfile)
	jenc.SetIndent("", "  ")
	jenc.Encode(cnts)
	jfile.Close()

	doStats(items)
}
