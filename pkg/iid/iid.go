package iid

import (
	"database/sql"
	"fmt"
	"github.com/bluele/gcache"
	"github.com/je4/utils/v2/pkg/MySQLReprepareStmt"
	"github.com/je4/zsearch/v2/pkg/mediaserver"
	"github.com/je4/zsearch/v2/pkg/search"
	"github.com/pkg/errors"
	"strconv"
)

type IID struct {
	db                  *sql.DB
	schema              string
	dataprefix          string
	mediaserver         mediaserver.Mediaserver
	BenutzerStmt        *MySQLReprepareStmt.Stmt
	TechnikenStmt       *MySQLReprepareStmt.Stmt
	KlassierungenStmt   *MySQLReprepareStmt.Stmt
	StudiensemesterStmt *MySQLReprepareStmt.Stmt
	MittelStmt          *MySQLReprepareStmt.Stmt
	StatiStmt           *MySQLReprepareStmt.Stmt
	moduleCache         gcache.Cache
}

func getOrientation(metadata *mediaserver.Metadata) int64 {
	var orientation int64 = 1
	if metadata.Image != nil {
		if image, ok := metadata.Image.(map[string]interface{}); ok {
			if image["properties"] != nil {
				if props, ok := image["properties"].(map[string]interface{}); ok {
					if props["exif:Orientation"] != nil {
						if oStr, ok := props["exif:Orientation"].(string); ok {
							if oVal, err := strconv.ParseInt(oStr, 10, 64); err == nil {
								orientation = oVal
							}
						}
					}
				}

			}
		}
	}
	return orientation
}

func NewIID(db *sql.DB, schema string, dataprefix string, ms mediaserver.Mediaserver) (*IID, error) {
	iid := &IID{
		db:          db,
		schema:      schema,
		dataprefix:  dataprefix,
		mediaserver: ms,
		moduleCache: gcache.New(50).ARC().Build(),
	}
	return iid, iid.PrepareStmt()
}

func (iid *IID) PrepareStmt() error {

	var err error
	if iid.BenutzerStmt, err = MySQLReprepareStmt.Prepare(iid.db, fmt.Sprintf("SELECT `Rolle`, `Name` FROM `ArbeitenBenutzer` WHERE Arbeiten_idArbeiten=?")); err != nil {
		return errors.Wrap(err, "cannot prepare statement")
	}
	if iid.TechnikenStmt, err = MySQLReprepareStmt.Prepare(iid.db, fmt.Sprintf("SELECT `Name` FROM `ArbeitenTechniken` WHERE Arbeiten_idArbeiten=?")); err != nil {
		return errors.Wrap(err, "cannot prepare statement")
	}
	if iid.KlassierungenStmt, err = MySQLReprepareStmt.Prepare(iid.db, fmt.Sprintf("SELECT `Name` FROM `ArbeitenKlassierungen` WHERE Arbeiten_idArbeiten=?")); err != nil {
		return errors.Wrap(err, "cannot prepare statement")
	}
	if iid.StudiensemesterStmt, err = MySQLReprepareStmt.Prepare(iid.db, fmt.Sprintf("SELECT `Name` FROM `ArbeitenStudiensemester` WHERE Arbeiten_idArbeiten=?")); err != nil {
		return errors.Wrap(err, "cannot prepare statement")
	}
	if iid.MittelStmt, err = MySQLReprepareStmt.Prepare(iid.db, fmt.Sprintf("SELECT `Name` FROM `ArbeitenMittel` WHERE Arbeiten_idArbeiten=?")); err != nil {
		return errors.Wrap(err, "cannot prepare statement")
	}
	if iid.StatiStmt, err = MySQLReprepareStmt.Prepare(iid.db, fmt.Sprintf("SELECT `Name` FROM `ArbeitenStati` WHERE Arbeiten_idArbeiten=? ORDER BY ModDate DESC")); err != nil {
		return errors.Wrap(err, "cannot prepare statement")
	}

	return nil
}

func (iid *IID) Close() error {
	return nil
}

func (iid *IID) LoadModule(id int64) (*Module, error) {
	if module, err := iid.moduleCache.Get(id); err == nil {
		return module.(*Module), nil
	}
	var sqlstr = fmt.Sprintf("SELECT `idModule`, `Semester`, Von, Bis, `ModulTypText`, `ModulArtName`, " +
		"`Kennziffer`, `Name`, `Titel`, `Abstract`, `Beschreibung`, `Bild`, `Uebergabe`, `Website`, deleted " +
		"FROM `FullModule` WHERE idModule=?")
	row := iid.db.QueryRow(sqlstr, id)
	module := &Module{IID: iid}
	if err := module.Scan(row); err != nil {
		return nil, errors.Wrapf(err, "cannot scan module %v", id)
	}
	iid.moduleCache.Set(id, module)
	return module, nil
}

func (iid *IID) IterateModulesAll(f func(module *Module) error) error {
	var sqlstr = fmt.Sprintf("SELECT `idModule`, "+
		"`Semester`, "+
		"Von, "+
		"Bis, "+
		"`ModulTypText`, "+
		"`ModulArtName`, "+
		"IFNULL(`Kennziffer`, \"\"), "+
		"IFNULL(`Name`, \"\"), "+
		"IFNULL(`Titel`, \"\"), "+
		"IFNULL(`Abstract`, \"\"), "+
		"IFNULL(`Beschreibung`, \"\"), "+
		"IFNULL(`Bild`, \"\"), "+
		"`Uebergabe`, "+
		"IFNULL(`Website`, \"\"), "+
		"deleted "+
		"FROM %s.`FullModule` "+
		"ORDER BY idModule DESC", iid.schema)
	rows, err := iid.db.Query(sqlstr)
	if err != nil {
		return errors.Wrapf(err, "cannot query %s", sqlstr)
	}
	defer rows.Close()
	for rows.Next() {
		var mod = &Module{
			IID: iid,
		}
		if err := mod.Scan(rows); err != nil {
			return errors.Wrap(err, "cannot scan row")
		}
		if mod.IsDeleted() {
			continue
		}
		if err := f(mod); err != nil {
			return errors.Wrap(err, "cannot handle item")
		}
	}
	return nil
}

/*

SELECT a.`idArbeiten`,
	a.`Benutzer_idHauptreferent`,
	CONCAT(hauptReferent.Nachname, ", ", hauptReferent.Vorname) AS Hauptreferent_Name,
	hauptReferent.Email AS Hauptreferent_Email,
	CONCAT(referent.Nachname, ", ", referent.Vorname) AS Referent_Name,
	referent.Email AS Referent_Email,
	a.`Benutzer_idReferent`,
	a.`Module_idModule`,
	a.`Titel`,
	a.`Kurzbeschreibung`,
	a.`Abstract`,
	a.`TextZurArbeit`,
	a.`Zusammenarbeit`,
	a.`Recherche`,
	a.`Auszeichnungen`,
	a.`Bemerkungen`,
	a.`Kontakt`,
	a.`Pfad`,
	a.`Bild1`,
	a.`Bild2`,
	a.`Bild3`,
	a.`Website`,
	a.`ExterneDoku`,
	m.Kennziffer AS Modulkennziffer
FROM Module m, `Arbeiten` a
LEFT JOIN Benutzer hauptReferent ON (a.Benutzer_idHauptreferent=hauptReferent.idBenutzer)
LEFT JOIN Benutzer referent ON (a.Benutzer_idReferent=referent.idBenutzer)
WHERE a.Module_idModule=m.idModule AND a.`deleted` IS NULL

*/

func (iid *IID) IterateArbeitenAll(f func(item *Arbeit) error) error {
	sqlstr := fmt.Sprintf("SELECT " +
		"idArbeiten, " +
		"`Hauptreferent_Name`, " +
		"`Referent_Name`, " +
		"IFNULL(`Module_idModule`,\"\"), " +
		"IFNULL(`Titel`,\"\"), " +
		"IFNULL(`Kurzbeschreibung`,\"\"), " +
		"IFNULL(`Abstract`,\"\"), " +
		"IFNULL(`TextZurArbeit`,\"\"), " +
		"IFNULL(`Zusammenarbeit`,\"\"), " +
		"IFNULL(`Recherche`,\"\"), " +
		"IFNULL(`Auszeichnungen`,\"\"), " +
		"IFNULL(`Bemerkungen`,\"\"), " +
		"IFNULL(`Kontakt`,\"\"), " +
		"IFNULL(`Pfad`,\"\"), " +
		"IFNULL(`Bild1`,\"\"), " +
		"IFNULL(`Bild2`,\"\"), " +
		"IFNULL(`Bild3`,\"\"), " +
		"IFNULL(`Website`,\"\"), " +
		"IFNULL(`ExterneDoku`,\"\"), " +
		"IFNULL(`Modulkennziffer`,\"\"), " +
		"IFNULL(`ModDate`,\"\") " +
		"FROM `FullArbeiten` " +
		//		"WHERE idArbeiten <= 2360 " +
		"ORDER BY idArbeiten DESC")

	rows, err := iid.db.Query(sqlstr)
	if err != nil {
		return errors.Wrapf(err, "cannot execute %s", sqlstr)
	}
	defer rows.Close()
	for rows.Next() {
		a := &Arbeit{
			iid:             iid,
			Persons:         []search.Person{},
			Techniken:       []string{},
			Genre:           []string{},
			Klassierung:     []string{},
			Studiensemester: []string{},
		}
		if err := a.Scan(rows); err != nil {
			return errors.Wrapf(err, "cannot scan arbeit")
		}
		if err := f(a); err != nil {
			return err
		}
	}

	return nil
}
