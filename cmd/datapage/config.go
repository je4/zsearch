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

type Template struct {
	Detail      []string `toml:"detail"`
	Error       []string `toml:"error"`
	Forbidden   []string `toml:"forbidden"`
	Search      []string `toml:"search"`
	ImageSearch []string `toml:"imagesearch"`
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

type SubFilter struct {
	Name   string `toml:"name"`
	Label  string `toml:"label"`
	Filter string `toml:"filter"`
}

type Query struct {
	BaseFilter string      `toml:"basefilter"`
	SubFilter  []SubFilter `toml:"subfilter"`
}

type Cfg_ElasticSearch struct {
	Endpoint []string `toml:"endpoint"`
	Index    string   `toml:"index"`
}

type Cfg_Google struct {
	Apikey           string            `toml:"apikey"`
	CustomSearchKeys map[string]string `toml:"searchkeys"`
}

type Config struct {
	Logfile             string              `toml:"logfile"`
	Loglevel            string              `toml:"loglevel"`
	AccessLog           string              `toml:"accesslog"`
	Addr                string              `toml:"addr"`
	AddrExt             string              `toml:"addrext"`
	CertPEM             string              `toml:"certpem"`
	KeyPEM              string              `toml:"keypem"`
	StaticDir           string              `toml:"staticdir"`
	StaticPrefix        string              `toml:"staticprefix"`
	StaticCacheControl  string              `toml:"staticcachecontrol"`
	DetailPrefix        string              `toml:"detailprefix"`
	UpdatePrefix        string              `toml:"updateprefix"`
	SearchPrefix        string              `toml:"searchprefix"`
	CollectionsPrefix   string              `toml:"collectionsprefix"`
	ImageSearchPrefix   string              `toml:"imagesearchprefix"`
	ClusterSearchPrefix string              `toml:"clustersearchprefix"`
	GoogleSearchPrefix  string              `toml:"googlesearchprefix"`
	CollectionsCatalog  string              `toml:"collectionscatalog"`
	ClusterCatalog      string              `toml:"clustercatalog"`
	ApiPrefix           string              `toml:"apiprefix"`
	JWTKey              string              `toml:"jwtkey"`
	JWTAlg              []string            `toml:"jwtalg"`
	LinkTokenExp        duration            `toml:"linktokenexp"`
	LoginUrl            string              `toml:"loginurl"`
	LoginIssuer         string              `toml:"loginissuer"`
	IdleTimeout         duration            `toml:"idletimeout"`
	UserCacheSize       int                 `toml:"usercachesize"`
	Template            map[string][]string `toml:"template"`
	TemplateDev         bool                `toml:"templatedev"`
	Solr                Solr                `toml:"solr"`
	Query               Query               `toml:"query"`
	AccessGroup         AccessGroup         `toml:"access"`
	Mediaserver         string              `toml:"mediaserver"`
	MediaserverKey      string              `toml:"mediaserverkey"`
	MediaserverExp      duration            `toml:"mediaserverexp"`
	AmpCache            string              `toml:"ampcache"`
	AmpApiKey           string              `toml:"ampapikey"`
	CacheDir            string              `toml:"cachedir"`
	ClearCacheOnStartup bool                `toml:"clearcacheonstartup"`
	SearchFields        map[string]string   `toml:"searchfields"`
	Facets              []Facet             `toml:"facets"`
	Locations           []Network           `toml:"locations"`
	Icons               map[string]string   `toml:"icons"`
	ElasticSearch       Cfg_ElasticSearch   `toml:"elasticsearch"`
	Google              Cfg_Google          `toml:"google"`
}

func LoadConfig(filepath string) Config {
	var conf Config
	m, err := toml.DecodeFile(filepath, &conf)
	if err != nil {
		log.Fatalln("Error on loading config: ", err)
	}
	fmt.Sprintf("%v", m)
	// make sure, that medaiserver url ends with an /
	conf.Mediaserver = strings.TrimRight(conf.Mediaserver, "/")
	conf.AddrExt = strings.TrimRight(conf.AddrExt, "/")
	conf.StaticPrefix = strings.Trim(conf.StaticPrefix, "/")
	conf.DetailPrefix = strings.Trim(conf.DetailPrefix, "/")
	conf.UpdatePrefix = strings.Trim(conf.UpdatePrefix, "/")
	conf.SearchPrefix = strings.Trim(conf.SearchPrefix, "/")
	conf.CollectionsPrefix = strings.Trim(conf.CollectionsPrefix, "/")
	conf.ImageSearchPrefix = strings.Trim(conf.ImageSearchPrefix, "/")
	conf.ClusterSearchPrefix = strings.Trim(conf.ClusterSearchPrefix, "/")
	conf.GoogleSearchPrefix = strings.Trim(conf.GoogleSearchPrefix, "/")
	conf.ApiPrefix = strings.Trim(conf.ApiPrefix, "/")
	return conf
}
