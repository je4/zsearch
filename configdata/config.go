package configdata

import (
	"time"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

type CfgDatabase struct {
	ServerType string
	DSN        string
	ConnMax    int `toml:"connection_max"`
	Schema     string
}

type CfgZotero struct {
	Endpoint string      `toml:"endpoint"`
	Apikey   string      `toml:"apikey"`
	DB       CfgDatabase `toml:"database"`
}

type MediaserverMySQL struct {
	DB  CfgDatabase `toml:"database"`
	Url string      `toml:"url"`
}

type CfgElasticSearch struct {
	Endpoint []string `toml:"endpoint"`
	Index    string   `toml:"index"`
}

type CfgS3 struct {
	Endpoint        string `toml:"endpoint"`
	AccessKeyId     string `toml:"accessKeyId"`
	SecretAccessKey string `toml:"secretAccessKey"`
	UseSSL          bool   `toml:"useSSL"`
}

type CfgHypothesis struct {
	Apikey   string `toml:"apikey"`
	Endpoint string `toml:"endpoint"`
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

type CfgFairService struct {
	ServiceName    string `toml:"servicename"`
	Address        string `toml:"address"`
	CertSkipVerify bool   `toml:"certskipverify"`
	JwtKey         string `toml:"jwtkey"`
	JwtAlg         string `toml:"jwtalg"`
}

type CfgZSearchService struct {
	ServiceName    string `toml:"servicename"`
	Address        string `toml:"address"`
	CertSkipVerify bool   `toml:"certskipverify"`
	JwtKey         string `toml:"jwtkey"`
	JwtAlg         string `toml:"jwtalg"`
}
