package apply

import (
	"database/sql"
	"fmt"
	"github.com/je4/utils/v2/pkg/MySQLReprepareStmt"
	"github.com/je4/zsearch/v2/pkg/mediaserver"
	"github.com/op/go-logging"
	"github.com/pkg/errors"
	"strconv"
	"strings"
)

type Apply struct {
	db                    *sql.DB
	schema                string
	mediaserver           mediaserver.Mediaserver
	mediaserverCollection string
	FormStmt              *MySQLReprepareStmt.Stmt
	DataStmt              *MySQLReprepareStmt.Stmt
	FileStmt              *MySQLReprepareStmt.Stmt
	filePath              string
	logger                *logging.Logger
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

func NewApply(logger *logging.Logger, db *sql.DB, schema string, filePath string, ms mediaserver.Mediaserver, msCollection string) (*Apply, error) {
	apply := &Apply{
		logger:                logger,
		db:                    db,
		schema:                schema,
		filePath:              filePath,
		mediaserver:           ms,
		mediaserverCollection: msCollection,
	}
	return apply, apply.PrepareStmt()
}

func (apply *Apply) PrepareStmt() error {

	var err error
	if apply.FormStmt, err = MySQLReprepareStmt.Prepare(apply.db, fmt.Sprintf(
		"SELECT f.formid, f.link, p.name, p.title \n"+
			" FROM form f, project p\n"+
			" WHERE f.projectid=p.projectid\n"+
			" AND f.projectid=? AND f.status=?")); err != nil {
		return errors.Wrap(err, "cannot prepare statement")
	}

	if apply.DataStmt, err = MySQLReprepareStmt.Prepare(apply.db, fmt.Sprintf(
		"SELECT name, value "+
			" FROM formdata"+
			" WHERE formid=?")); err != nil {
		return errors.Wrap(err, "cannot prepare statement")
	}

	if apply.FileStmt, err = MySQLReprepareStmt.Prepare(apply.db, fmt.Sprintf(
		"SELECT fileid, name, filename, mimetype, size "+
			" FROM files"+
			" WHERE formid=?")); err != nil {
		return errors.Wrap(err, "cannot prepare statement")
	}

	return nil
}

func (apply *Apply) Close() error {
	return nil
}

func (apply *Apply) IterateFormsAll(f func(form *Form) error) error {
	var forms = []*Form{}
	rows, err := apply.FormStmt.Query(1, "upload")
	if err != nil {
		return errors.Wrapf(err, "cannot query forms")
	}
	for rows.Next() {
		var form = &Form{
			Files:  []*FormFile{},
			Data:   map[string]string{},
			apply:  apply,
			Errors: []string{},
		}
		if err := rows.Scan(&form.Id, &form.Link, &form.Project, &form.ProjectTitel); err != nil {
			rows.Close()
			return errors.Wrap(err, "cannot scan row")
		}
		forms = append(forms, form)
	}
	rows.Close()
	for _, form := range forms {
		rows, err := apply.DataStmt.Query(form.Id)
		if err != nil {
			return errors.Wrapf(err, "cannot query forms")
		}
		for rows.Next() {
			var data = &FormData{}
			if err := rows.Scan(&data.Key, &data.Value); err != nil {
				rows.Close()
				return errors.Wrap(err, "cannot scan row")
			}
			form.Data[data.Key] = data.Value
		}
		rows.Close()

		rows, err = apply.FileStmt.Query(form.Id)
		if err != nil {
			return errors.Wrapf(err, "cannot query forms")
		}
		for rows.Next() {
			var file = &FormFile{}
			// fileid, name, filename, mimetype, size
			if err := rows.Scan(&file.Id, &file.Name, &file.Filename, &file.Mimetype, &file.Size); err != nil {
				rows.Close()
				return errors.Wrap(err, "cannot scan row")
			}
			if file.Size > 0 {
				form.Files = append(form.Files, file)
				if !(strings.HasPrefix(file.Mimetype, "image/") ||
					strings.HasPrefix(file.Mimetype, "audio/") ||
					strings.HasPrefix(file.Mimetype, "video/") ||
					strings.HasPrefix(file.Mimetype, "application/pdf")) {
					form.Errors = append(form.Errors, fmt.Sprintf("file %s has unknown mimetype %s", file.Name, file.Mimetype))
				}
			} else {
				form.Errors = append(form.Errors, fmt.Sprintf("file %s has size 0", file.Name))
			}
		}
		rows.Close()
		if err := f(form); err != nil {
			return err
		}
	}
	return nil
}
