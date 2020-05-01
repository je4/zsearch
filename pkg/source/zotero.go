package source

import (
	"encoding/json"
	"github.com/goph/emperror"
)

type Zotero struct {
	zdata ZoteroData
}

func NewZotero( data string ) (*Zotero, error) {
	zot := &Zotero{zdata: ZoteroData{}}
	return zot, zot.Init(data)
}

func (zot *Zotero) Init(data string) (error) {
	var zdata ZoteroData
	err := json.Unmarshal([]byte(data), &zdata)
	if err != nil {
		return emperror.Wrapf(err, "cannot unmarshal json\n%s", data)
	}
	return nil
}