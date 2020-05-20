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

type Google struct {
	Apikey string `toml:"apikey"`
}

type Template struct {
	Mediaserver    string   `toml:"mediaserver"`
	MediaserverKey string   `toml:"mediaserverkey"`
	Detail         []string `toml:"detail"`
	Error          []string `toml:"error"`
	Forbidden      []string `toml:"forbidden"`
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

type Config struct {
	Logfile       string      `toml:"logfile"`
	Loglevel      string      `toml:"loglevel"`
	AccessLog     string      `toml:"accesslog"`
	Addr          string      `toml:"addr"`
	CertPEM       string      `toml:"certpem"`
	KeyPEM        string      `toml:"keypem"`
	StaticDir     string      `toml:"staticdir"`
	StaticPrefix  string      `toml:"staticprefix"`
	PublicPrefix  string      `toml:"publicprefix"`
	PrivatePrefix string      `toml:"privateprefix"`
	JWTKey        string      `toml:"jwtkey"`
	JWTAlg        []string    `toml:"jwtalg"`
	LoginUrl      string      `toml:"loginurl"`
	LoginIssuer   string      `toml:"loginissuer"`
	IdleTimeout   duration    `toml:"idletimeout"`
	UserCacheSize int         `toml:"usercachesize"`
	Google        Google      `toml:"google"`
	Template      Template    `toml:"template"`
	Solr          Solr        `toml:"solr"`
	AccessGroup   AccessGroup `toml:"access"`
}

func LoadConfig(filepath string) Config {
	var conf Config
	_, err := toml.DecodeFile(filepath, &conf)
	if err != nil {
		log.Fatalln("Error on loading config: ", err)
	}
	// make sure, that medaiserver url ends with an /
	conf.Template.Mediaserver = strings.TrimRight(conf.Template.Mediaserver, "/")
	return conf
}
