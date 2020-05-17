package source

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"github.com/goph/emperror"
	"io"
)

func GUnzip(data string) (string, error) {
	var src, dest bytes.Buffer

	bytedata, err := base64.StdEncoding.DecodeString(data)
	if _, err := src.Write(bytedata); err != nil {
		return "", emperror.Wrap(err, "cannot write data into buffer")
	}
	zr, err := gzip.NewReader(&src)
	if err != nil {
		return "", emperror.Wrap(err, "cannot create gzip reader")
	}
	if _, err := io.Copy(&dest, zr); err != nil {
		return "", emperror.Wrap(err, "uncompress data")
	}
	return dest.String(), nil
}

type Person struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

type Media struct {
	Name     string `json:"name"`
	Mimetype string `json:"mimetype"`
	Type     string `json:"type"`
	Uri      string `json:"uri"`
	Width    int64  `json:"width"`
	Height   int64  `json:"height"`
	Duration int64  `json:"duration"`
}

type MediaList []Media

func (ml MediaList) Len() int           { return len(ml) }
func (ml MediaList) Swap(i, j int)      { ml[i], ml[j] = ml[j], ml[i] }
func (ml MediaList) Less(i, j int) bool { return ml[i].Name < ml[j].Name }

type Source interface {
	Init(data string) error
	Name() string
	GetTitle() string
	GetCollectionTitle() string
	GetNames() []Person
	GetTags() []string
	GetMedia() map[string]MediaList
	GetNotes() []string
	GetAbstract() string
}

type SourceData struct {
	Source          string               `json:"source"`
	Title           string               `json:"title"`
	CollectionTitle string               `json:"collectiontitle"`
	Persons         []Person             `json:"persons"`
	Tags            []string             `json:"tags"`
	Media           map[string]MediaList `json:"media"`
	Notes           []string             `json:"notes"`
	Abstract        string               `json:"abstract"`
	HasMedia        bool                 `json:"hasmedia"`
}
