package forms2

import (
	"database/sql"
	"fmt"
	"github.com/je4/zsearch/v2/pkg/mediaserver"
	"github.com/pkg/errors"
	"strings"
)

type Forms2 struct {
	db          *sql.DB
	schema      string
	dataprefix  string
	mediaserver mediaserver.Mediaserver
}

func NewForms2(db *sql.DB, schema string, dataprefix string, ms mediaserver.Mediaserver) (*Forms2, error) {
	f2 := &Forms2{
		db:          db,
		schema:      schema,
		dataprefix:  dataprefix,
		mediaserver: ms,
	}
	return f2, nil
}

func (f2 *Forms2) GetGroups() ([]int64, error) {
	var result = []int64{}
	sqlstr := fmt.Sprintf("SELECT DISTINCT year FROM %s.source_diplomhgk WHERE done=1", f2.schema)
	rows, err := f2.db.Query(sqlstr)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot query %s", sqlstr)
	}
	defer rows.Close()
	for rows.Next() {
		var val int64
		if err := rows.Scan(&val); err != nil {
			return nil, errors.Wrapf(err, "cannot scan value")
		}
		result = append(result, val)
	}
	return result, nil
}

func (f2 *Forms2) IterateItemsAll(year int64, f func(item *Item) error) error {
	var sqlstr = fmt.Sprintf("SELECT year, IDPerson, Vornamen, Nachname, Anlassnummer, Anlassbezeichnung FROM %s.source_diplomhgk WHERE year=? AND done=1", f2.schema)
	rows, err := f2.db.Query(sqlstr, year)
	if err != nil {
		return errors.Wrapf(err, "cannot query %s", sqlstr)
	}
	defer rows.Close()
	for rows.Next() {
		var firstname, lastname string
		var item = &Item{
			ms:   f2.mediaserver,
			Data: map[string]string{},
			File: map[int64]*File{},
		}
		if err := rows.Scan(&item.Year, &item.IDPerson, &firstname, &lastname, &item.Anlassnummer, &item.Anlassbezeichnung); err != nil {
			return errors.Wrapf(err, "cannot scan value")
		}
		firstname = strings.TrimSpace(firstname)
		lastname = strings.TrimSpace(lastname)
		if firstname == "" {
			item.PersonName = lastname
		} else {
			item.PersonName = fmt.Sprintf("%s, %s", lastname, firstname)
		}
		sqlstr = fmt.Sprintf("SELECT name, value from %s.source_diplomhgk_data WHERE year=? and idperson=?", f2.schema)
		rows2, err := f2.db.Query(sqlstr, year, item.IDPerson)
		if err != nil {
			return errors.Wrapf(err, "cannot query %s - %v/%v", sqlstr, year, item.IDPerson)
		}
		for rows2.Next() {
			var field, value string
			if err := rows2.Scan(&field, &value); err != nil {
				rows2.Close()
				return errors.Wrapf(err, "cannot scan fields")
			}
			if field == "email" {
				continue
			}
			item.Data[field] = strings.TrimSpace(value)
		}
		rows2.Close()

		sqlstr = fmt.Sprintf("SELECT fileid, name, filename from %s.source_diplomhgk_files WHERE year=? and idperson=? ORDER BY fileid ASC", f2.schema)
		rows2, err = f2.db.Query(sqlstr, year, item.IDPerson)
		if err != nil {
			return errors.Wrapf(err, "cannot query %s - %v/%v", sqlstr, year, item.IDPerson)
		}
		for rows2.Next() {
			var fileid int64
			var name, filename string
			if err := rows2.Scan(&fileid, &name, &filename); err != nil {
				rows2.Close()
				return errors.Wrapf(err, "cannot scan fields")
			}
			urn := fmt.Sprintf("%s/diplomhgk%v/%s", f2.dataprefix, item.Year, filename)
			coll, sig, err := f2.mediaserver.FindByUrn(urn)
			if err != nil {
				continue
			}
			var file = &File{
				Name:     name,
				Filename: fmt.Sprintf("mediaserver:%s/%s", coll, sig),
			}
			item.File[fileid] = file
		}
		rows2.Close()
		if err := f(item); err != nil {
			return errors.Wrapf(err, "cannot handle item")
		}
	}
	return nil
}
