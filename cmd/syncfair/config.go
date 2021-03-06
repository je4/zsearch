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

type Cfg_database struct {
	ServerType string
	DSN        string
	ConnMax    int `toml:"connection_max"`
	Schema     string
}

type Cfg_Zotero struct {
	Endpoint string       `toml:"endpoint"`
	Apikey   string       `toml:"apikey"`
	DB       Cfg_database `toml:"database"`
}

type MediaserverMySQL struct {
	DB  Cfg_database `toml:"database"`
	Url string       `toml:"url"`
}

type Cfg_S3 struct {
	Endpoint        string `toml:"endpoint"`
	AccessKeyId     string `toml:"accessKeyId"`
	SecretAccessKey string `toml:"secretAccessKey"`
	UseSSL          bool   `toml:"useSSL"`
}

type Query struct {
	BaseCatalog []string `toml:"basecatalog"`
	//SubFilter   []SubFilter `toml:"subfilter"`
}

type FairService struct {
	Address        string `toml:"address"`
	CertSkipVerify bool   `toml:"certskipverify"`
	jwtKey         string `toml:"jwtkey"`
}

type Endpoint struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

type Tunnel struct {
	Name           string   `toml:"name"`
	LocalEndpoint  Endpoint `toml:"localendpoint"`
	RemoteEndpoint Endpoint `toml:"remoteendpoint"`
}

type SSHTunnel struct {
	User           string   `toml:"user"`
	PrivateKey     string   `toml:"privatekey"`
	ServerEndpoint Endpoint `toml:"serverendpoint"`
	Tunnel         []Tunnel `toml:"tunnel"`
}

type Config struct {
	Logfile             string            `toml:"logfile"`
	Loglevel            string            `toml:"loglevel"`
	CacheDir            string            `toml:"cachedir"`
	StaticDir           string            `toml:"staticdir"`
	AddrExt             string            `toml:"addrext"`
	Prefixes            map[string]string `toml:"prefix"`
	SitemapPrefix       string            `toml:"sitemapprefix"`
	ClearCacheOnStartup bool              `toml:"clearcacheonstartup"`
	Sleep               duration          `toml:"sleep"`
	Mediaserver         MediaserverMySQL  `toml:"mediaserver"`
	Zotero              Cfg_Zotero        `toml:"zotero"`
	S3                  Cfg_S3            `toml:"s3"`
	Groups              []int64           `toml:"groups"`
	ClearBeforSync      []int64           `toml:"clearbeforesync"`
	Query               Query             `toml:"query"`
	FairService         FairService       `toml:"fairservice"`
	SSHTunnel           SSHTunnel         `toml:"sshtunnel"`
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
