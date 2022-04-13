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
	sdmlcontent "github.com/je4/salon-digital/v2/pkg/content"
	"github.com/je4/utils/v2/pkg/ssh"
	"github.com/je4/zsearch/v2/pkg/apply"
	"github.com/je4/zsearch/v2/pkg/mediaserver"
	"github.com/je4/zsearch/v2/pkg/search"
	"github.com/je4/zsearch/v2/pkg/zsearchclient"
	"github.com/je4/zsync/pkg/filesystem"
	"github.com/je4/zsync/pkg/zotero"
	"github.com/pkg/errors"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const OnlyPCB = false

func main() {
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

	zsClient, err := zsearchclient.NewZSearchClient(
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
	/*
		if err := correction(applicationDB, "./cmd/syncbang/bangbang 26_3_gemacht_.csv"); err != nil {
			fmt.Sprintf("%v", err)
		}

		return
	*/

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

	app, err := apply.NewApply(logger, applicationDB, config.ApplicationDB.Schema, config.FilePath, ms, "bangbang")
	if err != nil {
		logger.Panic(err)
		return
	}
	defer app.Close()

	var counter int64 = 0

	/*
		if _, err := zsClient.SignaturesClear("bangbang"); err != nil {
			logger.Panicf("cannot clear signatures with prefix 'bangbang': %v", err)
		}
	*/

	//since := time.Date(1970, 01, 01, 0, 0, 0, 0, time.Local)
	// starting update

	var items = []*search.SourceData{}

	if !OnlyPCB {

		if err := app.IterateFormsAll(func(form *apply.Form) error {
			if form.Id > 30 {
				//return nil
			}
			// todo: use fair service
			src, err := search.NewSourceData(form)
			if err != nil {
				return errors.Wrap(err, "cannot create sourcedata from iid item")
			}
			logger.Infof("creating work %v", src.GetSignature())
			if err := zsClient.SignatureCreate(src); err != nil {
				return errors.Wrapf(err, "cannot create work entity")
			}
			counter++
			items = append(items, src)
			return nil
		},
		); err != nil {
			logger.Panicf("error iterating works: %v", err)
		}

		if counter > 0 {
			zsClient.ClearCache()
			if err := zsClient.BuildSitemap(); err != nil {
				logger.Panic(err)
			}
		}
	}

	group, err := zot.LoadGroupLocal(1624911)
	if err != nil {
		logger.Panicf("cannot load groups: %v", err)
	}

	since := time.Date(1970, 01, 01, 0, 0, 0, 0, time.Local)
	// starting update
	if err := group.IterateItemsAllLocal(
		&since,
		func(item *zotero.Item) error {
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

			counter++
			return nil
		},
	); err != nil {
		logger.Errorf("error getting items: %v", err)
	}

	var cnts = []*sdmlcontent.Content{}
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
		if locked == 0 && formid > 0 {
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
		cnt := &sdmlcontent.Content{Persons: []sdmlcontent.Person{}, Medias: map[string]sdmlcontent.Media{}}
		cnt.Title = item.GetTitle()
		cnt.Year = item.GetDate()
		for _, p := range item.GetPersons() {
			cnt.Persons = append(cnt.Persons, sdmlcontent.Person{Name: p.Name, Role: p.Role})
			if _, ok := persons[p.Role]; !ok {
				persons[p.Role] = []string{}
			}
			persons[p.Role] = append(persons[p.Role], p.Name)
		}
		/*
			for t, ms := range item.GetMedia() {
				cnt.Medias[t] =
				for _, m := range ms {
					cnt.Medias[]
				}
			}
		*/
		cnts = append(cnts, cnt)
	}
	jfile, err := os.OpenFile("c:/temp/salon.json", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		logger.Panic(err)
	}
	jenc := json.NewEncoder(jfile)
	jenc.SetIndent("", "  ")
	jenc.Encode(cnts)
	jfile.Close()

	if err := writeCSV("c:/temp", items); err != nil {
		logger.Panic(err)
	}
	if err := writePersons("c:/temp", items); err != nil {
		logger.Panic(err)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Transcode videos now")
	reader.ReadString('\n')

	if err := writeData(logger, config.ListTemplate, config.DetailTemplate, config.TableTemplate, config.ExportPath, ms, items, false); err != nil {
		logger.Panic(err)
	}

}
