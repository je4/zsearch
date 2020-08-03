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
	"net"
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

type network struct {
	net.IPNet
}

func (n *network) UnmarshalText(text []byte) error {
	_, net, err := net.ParseCIDR(string(text))
	if err != nil {
		return err
	}
	if net == nil {
		return fmt.Errorf("no network - %s", string(text))
	}
	n.IPNet = *net
	return nil
}

type Google struct {
	Apikey string `toml:"apikey"`
}

type Template struct {
	Detail    []string `toml:"detail"`
	Error     []string `toml:"error"`
	Forbidden []string `toml:"forbidden"`
	Search    []string `toml:"search"`
}

type Solr struct {
	Url             string   `toml:"url"`
	Core            string   `toml:"core"`
	CacheExpiration duration `toml:"cacheexpiration"`
	CacheSize       int      `toml:"cachesize"`
}

type AccessGroup struct {
	Admin string `toml:"admin"`
	Guest string `toml:"guest"`
}

type Facet struct {
	Name     string          `toml:"name"`
	Field    string          `toml:"field"`
	Query    string          `toml:"query"`
	Prefix   string          `toml:"prefix"`
	Restrict map[string]bool `toml:"restrict"`
	Type     string          `toml:"type"`
}

type Network struct {
	Group    string    `toml:"group"`
	Networks []network `toml:"networks"`
}

type Config struct {
	Logfile             string            `toml:"logfile"`
	Loglevel            string            `toml:"loglevel"`
	AccessLog           string            `toml:"accesslog"`
	Addr                string            `toml:"addr"`
	AddrExt             string            `toml:"addrext"`
	CertPEM             string            `toml:"certpem"`
	KeyPEM              string            `toml:"keypem"`
	StaticDir           string            `toml:"staticdir"`
	StaticPrefix        string            `toml:"staticprefix"`
	DetailPrefix        string            `toml:"detailprefix"`
	UpdatePrefix        string            `toml:"updateprefix"`
	SearchPrefix        string            `toml:"searchprefix"`
	ApiPrefix           string            `toml:"apiprefix"`
	JWTKey              string            `toml:"jwtkey"`
	JWTAlg              []string          `toml:"jwtalg"`
	LinkTokenExp        duration          `toml:"linktokenexp"`
	LoginUrl            string            `toml:"loginurl"`
	LoginIssuer         string            `toml:"loginissuer"`
	IdleTimeout         duration          `toml:"idletimeout"`
	UserCacheSize       int               `toml:"usercachesize"`
	Google              Google            `toml:"google"`
	Template            Template          `toml:"template"`
	Solr                Solr              `toml:"solr"`
	AccessGroup         AccessGroup       `toml:"access"`
	Mediaserver         string            `toml:"mediaserver"`
	MediaserverKey      string            `toml:"mediaserverkey"`
	MediaserverExp      duration          `toml:"mediaserverexp"`
	AmpCache            string            `toml:"ampcache"`
	AmpApiKey           string            `toml:"ampapikey"`
	CacheDir            string            `toml:"cachedir"`
	ClearCacheOnStartup bool              `toml:"clearcacheonstartup"`
	SearchFields        map[string]string `toml:"searchfields"`
	Facets              []Facet           `toml:"facets"`
	Locations            []Network         `toml:"locations"`
}

func LoadConfig(filepath string) Config {
	var conf Config
	_, err := toml.DecodeFile(filepath, &conf)
	if err != nil {
		log.Fatalln("Error on loading config: ", err)
	}
	// make sure, that medaiserver url ends with an /
	conf.Mediaserver = strings.TrimRight(conf.Mediaserver, "/")
	conf.AddrExt = strings.TrimRight(conf.AddrExt, "/")
	conf.StaticPrefix = strings.Trim(conf.StaticPrefix, "/")
	conf.DetailPrefix = strings.Trim(conf.DetailPrefix, "/")
	conf.UpdatePrefix = strings.Trim(conf.UpdatePrefix, "/")
	conf.SearchPrefix = strings.Trim(conf.SearchPrefix, "/")
	conf.ApiPrefix = strings.Trim(conf.ApiPrefix, "/")
	return conf
}
