package main

import (
	"github.com/BurntSushi/toml"
	"log"
)

type Google struct {
	Apikey string
}

type Templates struct {
	Details string
}

type Solr struct {
	Url  string
	Core string
}

type Config struct {
	Google    Google    `toml:"google"`
	Templates Templates `toml:"templates"`
	Solr      Solr      `toml:"solr"`
}

func LoadConfig(filepath string) Config {
	var conf Config
	_, err := toml.DecodeFile(filepath, &conf)
	if err != nil {
		log.Fatalln("Error on loading config: ", err)
	}
	return conf
}
