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
	BaseCatalog []string    `toml:"basecatalog"`
	SubFilter   []SubFilter `toml:"subfilter"`
}

type Cfg_ElasticSearch struct {
	Endpoint []string `toml:"endpoint"`
	Index    string   `toml:"index"`
}

type Cfg_Google struct {
	Apikey           string `toml:"apikey"`
	CustomSearchKeys map[string]struct {
		Key  string
		Name string
	} `toml:"searchkeys"`
}

type Endpoint struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

type Elastic struct {
	Index       string `toml:"elasticindex"`       // elastic search index to fill
	IndexCreate string `toml:"elasticindexcreate"` //  deletes and creates elastic index with given schema
	Endpoint    string `toml:"elasticendpoint"`    // endpoint for elastic search
	V8          bool   `toml:"elasticv8"`          // use elastic search client version 8
	APIKey      string `toml:"elasticapikey"`      // Apikey for elastic
}

type SSHTunnel struct {
	User           string   `toml:"user"`
	PrivateKey     string   `toml:"privatekey"`
	LocalEndpoint  Endpoint `toml:"localendpoint"`
	ServerEndpoint Endpoint `toml:"serverendpoint"`
	RemoteEndpoint Endpoint `toml:"remoteendpoint"`
}

type Config struct {
	ServiceName         string              `toml:"servicename"`
	Logfile             string              `toml:"logfile"`
	Loglevel            string              `toml:"loglevel"`
	AccessLog           string              `toml:"accesslog"`
	Addr                string              `toml:"addr"`
	AddrExt             string              `toml:"addrext"`
	CertPEM             string              `toml:"certpem"`
	KeyPEM              string              `toml:"keypem"`
	Prefixes            map[string]string   `toml:"prefix"`
	StaticDir           string              `toml:"staticdir"`
	SitemapDir          string              `toml:"sitemapdir"`
	TemplateDir         string              `toml:"templatedir"`
	StaticCacheControl  string              `toml:"staticcachecontrol"`
	CollectionsCatalog  string              `toml:"collectionscatalog"`
	ClusterCatalog      string              `toml:"clustercatalog"`
	JWTKey              string              `toml:"jwtkey"`
	JWTAlg              []string            `toml:"jwtalg"`
	LinkTokenExp        duration            `toml:"linktokenexp"`
	LoginUrl            string              `toml:"loginurl"`
	LoginIssuer         string              `toml:"loginissuer"`
	IdleTimeout         duration            `toml:"idletimeout"`
	SessionTimeout      duration            `toml:"sessiontimeout"`
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
	CacheExpiry         duration            `toml:"cacheexpiry"`
	SearchFields        map[string]string   `toml:"searchfields"`
	Facets              []Facet             `toml:"facets"`
	Locations           []Network           `toml:"locations"`
	Icons               map[string]string   `toml:"icons"`
	FacebookAppId       string              `toml:"facebookappid"`
	ElasticSearch       Cfg_ElasticSearch   `toml:"elasticsearch"`
	Google              Cfg_Google          `toml:"google"`
	InstanceName        string              `toml:"instancename"`
	SSHTunnel           SSHTunnel           `toml:"sshtunnel"`
	Target              Elastic             `toml:"target"`
}

var prefixNames = []string{
	"detail",
	"update",
	"search",
	"images",
	"collections",
	"cse",
	"cluster",
	"api",
	"static",
	"sitemap",
}

func LoadConfig(filepath string) Config {
	var conf Config
	conf.ServiceName = "ZSearch"
	_, err := toml.DecodeFile(filepath, &conf)
	if err != nil {
		log.Fatalln("Error on loading config: ", err)
	}
	//fmt.Sprintf("%v", m)
	// make sure, that mediaserver url ends with an /
	conf.Mediaserver = strings.TrimRight(conf.Mediaserver, "/")
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
	if conf.CacheExpiry.Duration == 0 {
		conf.CacheExpiry.Duration = 3 * time.Hour
	}
	return conf
}
