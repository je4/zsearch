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
	Logfile       string                          `toml:"logfile"`
	Loglevel      string                          `toml:"loglevel"`
	Mediaserver   configdata.MediaserverMySQL     `toml:"mediaserver"`
	DataPrefix    string                          `toml:"dataprefix"`
	ElasticSearch configdata.CfgElasticSearch     `toml:"elasticsearch"`
	Tunnel        map[string]configdata.SSHTunnel `toml:"tunnel"`
	Filters       map[string][]string             `toml:"filter"`
	ExportPath    string                          `toml:"exportpath"`
	Width         int                             `toml:"width"`
	Height        int                             `toml:"height"`
	CHeight       int                             `toml:"cheight"`
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
