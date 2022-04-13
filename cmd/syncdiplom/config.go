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

type MediaserverMySQL struct {
	DB  Cfg_database `toml:"database"`
	Url string       `toml:"url"`
}

type Cfg_ElasticSearch struct {
	Endpoint []string `toml:"endpoint"`
	Index    string   `toml:"index"`
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

type Endpoint struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

type Forward struct {
	Local  Endpoint `toml:"local"`
	Remote Endpoint `toml:"remote"`
}

type SSHTunnel struct {
	User       string             `toml:"user"`
	PrivateKey string             `toml:"privatekey"`
	Endpoint   Endpoint           `toml:"endpoint"`
	Forward    map[string]Forward `toml:"forward"`
}

type Config struct {
	Logfile             string               `toml:"logfile"`
	Loglevel            string               `toml:"loglevel"`
	ElasticSearch       Cfg_ElasticSearch    `toml:"elasticsearch"`
	CacheDir            string               `toml:"cachedir"`
	StaticDir           string               `toml:"staticdir"`
	AddrExt             string               `toml:"addrext"`
	SitemapPrefix       string               `toml:"sitemapprefix"`
	ClearCacheOnStartup bool                 `toml:"clearcacheonstartup"`
	Sleep               duration             `toml:"sleep"`
	Mediaserver         MediaserverMySQL     `toml:"mediaserver"`
	FormsDB             Cfg_database         `toml:"formsdb"`
	DataPrefix          string               `toml:"dataprefix"`
	S3                  Cfg_S3               `toml:"s3"`
	Groups              []int64              `toml:"groups"`
	ClearBeforSync      []int64              `toml:"clearbeforesync"`
	Query               Query                `toml:"query"`
	Prefixes            map[string]string    `toml:"prefix"`
	Tunnel              map[string]SSHTunnel `toml:"tunnel"`
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
	conf.AddrExt = strings.TrimRight(conf.AddrExt, "/")

	return conf
}
