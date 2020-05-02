package main

import (
	"github.com/BurntSushi/toml"
	"log"
	"strings"
)

type Google struct {
	Apikey string
}

type Template struct {
	Mediaserver string `toml:"mediaserver"`
	Detail      string `toml:"detail"`
	Error       string `toml:"error"`
}

type Solr struct {
	Url  string
	Core string
}

type Config struct {
	Logfile       string   `toml:"logfile"`
	Loglevel      string   `toml:"loglevel"`
	AccessLog     string   `toml:"accesslog"`
	Addr          string   `toml:"addr"`
	CertPEM       string   `toml:"certpem"`
	KeyPEM        string   `toml:"keypem"`
	StaticDir     string   `toml:"staticdir"`
	StaticPrefix  string   `toml:"staticprefix"`
	PublicPrefix  string   `toml:"publicprefix"`
	PrivatePrefix string   `toml:"privateprefix"`
	Google        Google   `toml:"google"`
	Template      Template `toml:"template"`
	Solr          Solr     `toml:"solr"`
}

func LoadConfig(filepath string) Config {
	var conf Config
	_, err := toml.DecodeFile(filepath, &conf)
	if err != nil {
		log.Fatalln("Error on loading config: ", err)
	}
	// make sure, that medaiserver url ends with an /
	conf.Template.Mediaserver = strings.TrimRight(conf.Template.Mediaserver, "/") + "/"
	return conf
}
