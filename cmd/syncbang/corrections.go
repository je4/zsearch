package main

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"github.com/pkg/errors"
	"os"
	"strconv"
	"strings"
)

var fieldNames = []string{
	"artists",
	"year",
	"titel",
	"doctype",
	"dauer",
	"performers",
	"festival",
	"eventcurator",
	"eventplace",
	"medium",
	"descr",
	"remark",
	"function",
	"camera",
	"anderesformat",
	"additional",
	"nachname",
	"vorname",
	"web",
	"email",
	"jahrgang",
	"adresse",
	"tel",
	"rechtebangbang",
	"rechtemediathek",
}

func correction(db *sql.DB, csvFile string) error {
	f, err := os.Open(csvFile)
	if err != nil {
		return errors.Wrapf(err, "cannot open %s", csvFile)
	}
	defer f.Close()
	csvReader := csv.NewReader(f)
	records, err := csvReader.ReadAll()
	if err != nil {
		return errors.Wrapf(err, "cannot read %s", csvFile)
	}
	fields := make(map[string]int)
	for key, val := range records[0] {
		fields[val] = key
	}

	sqlstr := "UPDATE formdata SET value=? WHERE formid=? AND name=?" //; UPDATE formdata SET value=\"\" WHERE formid=9 AND name=\"performers\"; REPLACE INTO locked VALUES(9);\n"
	sqlstr2 := "REPLACE INTO locked VALUES(?)"
	for key, record := range records {
		if key == 0 {
			continue
		}
		formid, err := strconv.ParseInt(record[fields["id"]], 10, 64)
		if err != nil {
			return errors.Wrapf(err, "cannot parse formid %s", record[fields["id"]])
		}
		fmt.Printf("form %d of %d: #%d\n", key, len(records), formid)
		for _, fldname := range fieldNames {
			value := strings.TrimSpace(record[fields[fldname]])
			if _, err := db.Exec(sqlstr, value, formid, fldname); err != nil {
				return errors.Wrapf(err, "cannot execute %s [%v]", sqlstr, []interface{}{value, formid, fldname})
			}
		}
		if _, err := db.Exec(sqlstr2, formid); err != nil {
			return errors.Wrapf(err, "cannot execute %s [%v]", sqlstr2, formid)
		}
	}
	return nil
}
