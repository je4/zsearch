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
	"github.com/je4/utils/v2/pkg/config"
	"github.com/je4/zsearch/v2/configdata"
	"log"
	"strings"
)

type Config struct {
	Logfile        string                          `toml:"logfile"`
	Loglevel       string                          `toml:"loglevel"`
	FairService    configdata.CfgFairService       `toml:"fairservice"`
	ZSearchService configdata.CfgZSearchService    `toml:"zsearchservice"`
	Mediaserver    configdata.MediaserverMySQL     `toml:"mediaserver"`
	Zotero         configdata.CfgZotero            `toml:"zotero"`
	ApplicationDB  configdata.CfgDatabase          `toml:"applicationdb"`
	DataPrefix     string                          `toml:"dataprefix"`
	S3             configdata.CfgS3                `toml:"s3"`
	Tunnel         map[string]configdata.SSHTunnel `toml:"tunnel"`
	FilePath       string                          `toml:"filepath"`
	ExportPath     string                          `toml:"exportpath"`
	ListTemplate   string                          `toml:"listtemplate"`
	DetailTemplate string                          `toml:"detailtemplate"`
	TableTemplate  string                          `toml:"tabletemplate"`
	Full           bool                            `toml:"fullexport"`
	DeeplApiKey    config.EnvString                `toml:"deeplapikey"`
	DeeplApiUrl    string                          `toml:"deeplapiurl"`
	TanslateDBPath string                          `toml:"translatedbpath"`
	OpenaiApiUrl   string                          `toml:"openaiapiurl"`
	OpenaiApiKey   config.EnvString                `toml:"openaiapikey"`
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
	conf.DataPrefix = strings.TrimRight(conf.DataPrefix, "/")

	return conf
}
