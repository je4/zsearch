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
	"github.com/BurntSushi/toml"
	"github.com/je4/zsearch/v2/configdata"
	"log"
	"strings"
)

type Config struct {
	Logfile             string                          `toml:"logfile"`
	Loglevel            string                          `toml:"loglevel"`
	CacheDir            string                          `toml:"cachedir"`
	StaticDir           string                          `toml:"staticdir"`
	AddrExt             string                          `toml:"addrext"`
	Prefixes            map[string]string               `toml:"prefix"`
	SitemapPrefix       string                          `toml:"sitemapprefix"`
	ClearCacheOnStartup bool                            `toml:"clearcacheonstartup"`
	Sleep               configdata.Duration             `toml:"sleep"`
	Mediaserver         configdata.MediaserverMySQL     `toml:"mediaserver"`
	Zotero              configdata.CfgZotero            `toml:"zotero"`
	S3                  configdata.CfgS3                `toml:"s3"`
	Groups              []int64                         `toml:"groups"`
	ClearBeforSync      []int64                         `toml:"clearbeforesync"`
	Query               configdata.Query                `toml:"query"`
	FairService         configdata.CfgFairService       `toml:"fairservice"`
	Tunnel              map[string]configdata.SSHTunnel `toml:"tunnel"`
}

var prefixNames = []string{
	"detail",
	//	"update",
	//	"search",
	//	"images",
	//	"collections",
	//	"cse",
	//	"cluster",
	//	"api",
	"static",
}

func LoadConfig(filepath string) Config {
	var conf Config
	_, err := toml.DecodeFile(filepath, &conf)
	if err != nil {
		log.Fatalln("Error on loading config: ", err)
	}
	//fmt.Sprintf("%v", m)
	// make sure, that medaiserver url ends with an /
	conf.Mediaserver.Url = strings.TrimRight(conf.Mediaserver.Url, "/")

	conf.AddrExt = strings.TrimRight(conf.AddrExt, "/")

	// clean prefixes
	// check existence to avoid error handling on future prefix access
	for _, name := range prefixNames {
		val, ok := conf.Prefixes[name]
		if !ok {
			log.Fatalf("could not find prefix.%s in config file", name)
		}
		conf.Prefixes[name] = strings.Trim(val, "/")
	}

	return conf
}
