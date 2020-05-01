package source

import (
	"bytes"
	"compress/gzip"
	"github.com/goph/emperror"
	"io"
)

func GUnzip(data string) (string, error) {
	var src, dest bytes.Buffer
	if _, err := src.WriteString(data); err != nil {
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

type Source interface {
	Init( data string ) error
}
