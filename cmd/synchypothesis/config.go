/*
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
	"fmt"
	"github.com/BurntSushi/toml"
	"log"
	"strings"
	"time"
)

type duration struct {
	time.Duration
}

func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

type Cfg_ElasticSearch struct {
	Endpoint []string `toml:"endpoint"`
	Index    string   `toml:"index"`
}

type Hypothesis struct {
	Apikey   string `toml:"apikey"`
	Endpoint string `toml:"endpoint"`
}

type Config struct {
	Logfile             string            `toml:"logfile"`
	Loglevel            string            `toml:"loglevel"`
	CacheDir            string            `toml:"cachedir"`
	ElasticSearch       Cfg_ElasticSearch `toml:"elasticsearch"`
	Hypothesis          Hypothesis        `toml:"hypothesis"`
	AddrExt             string            `toml:"addrext"`
	Prefixes            map[string]string `toml:"prefix"`
	ClearCacheOnStartup bool              `toml:"clearcacheonstartup"`
	Sleep               duration          `toml:"sleep"`
	Groups              []int64           `toml:"groups"`
	ClearBeforSync      []int64           `toml:"clearbeforesync"`
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
	m, err := toml.DecodeFile(filepath, &conf)
	if err != nil {
		log.Fatalln("Error on loading config: ", err)
	}
	fmt.Sprintf("%v", m)
	// make sure, that medaiserver url ends with an /

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
