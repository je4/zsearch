package mediaserver

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt"
	"github.com/op/go-logging"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Collection struct {
	CollectionId,
	EstateId int64
	Name,
	Description string
	StorageId int64
	JWTKey    string
}

type Metadata struct {
	Mimetype string
	Type     string
	Ext      string
	Sha256   string
	Width    int64
	Height   int64
	Duration int64
	Filesize int64
	Image    interface{}
	Exif     interface{}
	Video    interface{}
}

type MediaserverMySQL struct {
	db                *sql.DB
	dbSchema          string
	logger            *logging.Logger
	base              *url.URL
	mediaserverRegexp *regexp.Regexp
	collections       []*Collection
}

func NewJWT(secret string, subject string, valid int64) (tokenString string, err error) {
	exp := time.Now().Unix() + valid
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": strings.ToLower(subject),
		"exp": exp,
	})
	//log.Println("NewJWT( ", secret, ", ", subject, ", ", exp)
	tokenString, err = token.SignedString([]byte(secret))
	return tokenString, err
}

func NewMediaserverMySQL(mediaserverbase string, db *sql.DB, dbSchema string, logger *logging.Logger) (*MediaserverMySQL, error) {
	url, err := url.Parse(mediaserverbase)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot parse url %s", mediaserverbase)
	}
	rs := fmt.Sprintf("%s/([^/]+)/([^/]+)/.*", url.String())
	regexp, err := regexp.Compile(rs)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot compile regexp %s", rs)
	}
	ms := &MediaserverMySQL{
		db:                db,
		dbSchema:          dbSchema,
		logger:            logger,
		base:              url,
		mediaserverRegexp: regexp,
	}
	ms.Init()
	return ms, nil
}

func (ms *MediaserverMySQL) Init() error {
	ms.collections = []*Collection{}
	sqlstr := fmt.Sprintf("SELECT c.collectionid, c.estateid, c.name, c.description, c.storageid, s.jwtkey"+
		" FROM %s.collection c, %s.storage s"+
		" WHERE c.storageid=s.storageid", ms.dbSchema, ms.dbSchema)
	rows, err := ms.db.Query(sqlstr)
	if err != nil {
		return errors.Wrapf(err, "cannot load collections - %s", sqlstr)
	}
	defer rows.Close()
	var jwtkey sql.NullString
	for rows.Next() {
		coll := &Collection{}
		if err := rows.Scan(
			&coll.CollectionId,
			&coll.EstateId,
			&coll.Name,
			&coll.Description,
			&coll.StorageId,
			&jwtkey); err != nil {
			return errors.Wrap(err, "cannot scan values of collections")
		}
		coll.JWTKey = jwtkey.String
		ms.collections = append(ms.collections, coll)
	}
	return nil
}

var regexpOldMedia = regexp.MustCompile("^http://media/([^/]+)/([^/]+)$")
var regexpMetaMedia = regexp.MustCompile("^mediaserver:([^/]+)/([^/]+)$")

func (ms *MediaserverMySQL) IsMediaserverURL(url string) (string, string, bool) {
	if matches := ms.mediaserverRegexp.FindStringSubmatch(url); matches != nil {
		return matches[1], matches[2], true
	}
	if matches := regexpOldMedia.FindStringSubmatch(url); matches != nil {
		return matches[1], matches[2], true
	}
	if matches := regexpMetaMedia.FindStringSubmatch(url); matches != nil {
		return matches[1], matches[2], true
	}
	return "", "", false
}

func (ms *MediaserverMySQL) GetCollectionByName(name string) (*Collection, error) {
	for _, coll := range ms.collections {
		if coll.Name == name {
			return coll, nil
		}
	}
	return nil, fmt.Errorf("cannot find collection %s", name)
}

func (ms *MediaserverMySQL) GetCollectionById(id int64) (*Collection, error) {
	for _, coll := range ms.collections {
		if coll.CollectionId == id {
			return coll, nil
		}
	}
	return nil, fmt.Errorf("cannot find collection #%v", id)
}

func (ms *MediaserverMySQL) GetOriginalUrn(collection, signature string) (string, error) {
	coll, err := ms.GetCollectionByName(collection)
	if err != nil {
		return "", errors.Wrap(err, "cannot get collection")
	}
	var urn sql.NullString
	sqlstr := fmt.Sprintf("SELECT urn FROM %s.master WHERE collectionid=? AND signature=?", ms.dbSchema)
	params := []interface{}{coll.CollectionId, signature}
	if err := ms.db.QueryRow(sqlstr, params...).Scan(&urn); err != nil {
		return "", errors.Wrapf(err, "cannot query database %s [%v]", sqlstr, params)
	}
	return urn.String, nil
}

func (ms *MediaserverMySQL) CreateMasterUrl(collection, signature, url string, public bool) error {
	coll, err := ms.GetCollectionByName(collection)
	if err != nil {
		return errors.Wrap(err, "cannot get collection")
	}
	sqlstr := fmt.Sprintf("SELECT masterid, public FROM %s.master WHERE collectionid=? AND signature=?", ms.dbSchema)
	params := []interface{}{coll.CollectionId, signature}
	var pub bool
	var masterid int64
	if err := ms.db.QueryRow(sqlstr, params...).Scan(&masterid, &pub); err != nil {
		sqlstr = fmt.Sprintf("INSERT INTO %s.master(collectionid,signature,urn, public) VALUES(?, ?, ?, ?)", ms.dbSchema)
		params = []interface{}{coll.CollectionId, signature, url, public}
		_, err = ms.db.Exec(sqlstr, params...)
		if err != nil {
			ms.logger.Errorf("master #%s/%s error on creation", collection, signature)
			return errors.Wrapf(err, "cannot create master: %s - %v", sqlstr, params)
		} else {
			ms.logger.Infof("master #%s/%s created", collection, signature)
		}
	} else {
		ms.logger.Infof("master #%s/%s already in database", collection, signature)
		if public != pub {
			ms.logger.Infof("update master access %v - #%s/%s", public, collection, signature)
			sqlstr = fmt.Sprintf("UPDATE %s.master SET public=? WHERE masterid=? OR parentid=?", ms.dbSchema)
			params = []interface{}{public, masterid, masterid}
			_, err = ms.db.Exec(sqlstr, params...)
			if err != nil {
				return errors.Wrapf(err, "update access: %s - %v", sqlstr, params)
			}
		}
	}

	return nil
}

func (ms *MediaserverMySQL) FindByUrn(urn string) (string, string, error) {
	var collection, signature string
	sqlstr := fmt.Sprintf("SELECT c.name, m.signature FROM %s.master m, %s.collection c"+
		" WHERE c.collectionid=m.collectionid AND m.urn=?", ms.dbSchema, ms.dbSchema)
	if err := ms.db.QueryRow(sqlstr, urn).Scan(&collection, &signature); err != nil {
		return "", "", errors.Wrapf(err, "cannot query %s - %s", sqlstr, urn)
	}
	return collection, signature, nil
}

func (ms *MediaserverMySQL) GetUrl(collection, signature, function string) (string, error) {
	coll, err := ms.GetCollectionByName(collection)
	if err != nil {
		return "", errors.Wrapf(err, "cannot find collection %s", collection)
	}

	function = strings.TrimPrefix(function, "/")
	url := fmt.Sprintf("%s/%s/%s/%s", ms.base, collection, signature, function)
	if coll.JWTKey != "" {
		// todo: test this
		sub := strings.ToLower(fmt.Sprintf("mediaserver:%s/%s/%s", collection, signature, function))
		key, err := NewJWT(coll.JWTKey, sub, 1799)
		if err != nil {
			return "", errors.Wrapf(err, "cannot build jwt token")
		}
		url += fmt.Sprintf("?token=%s", key)
	}
	return url, nil
}
func (ms *MediaserverMySQL) IsPublic(collection, signature string) (bool, error) {
	coll, err := ms.GetCollectionByName(collection)
	if err != nil {
		return false, errors.Wrapf(err, "cannot find collection %s", collection)
	}
	if coll.JWTKey == "" {
		return true, nil
	}

	sqlstr := fmt.Sprintf("SELECT `public` FROM %s.master WHERE collectionid=? AND signature=?", ms.dbSchema)
	params := []interface{}{coll.CollectionId, signature}
	var public int
	if err := ms.db.QueryRow(sqlstr, params...).Scan(&public); err != nil {
		return false, errors.Wrapf(err, "cannot query database %s [%v]", sqlstr, params)
	}
	return public == 1, nil
}
func (ms *MediaserverMySQL) GetMetadata(collection, signature string) (*Metadata, error) {

	url, err := ms.GetUrl(collection, signature, "metadata")
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build url %s", "metadata")
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot load url %s", url)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp.Body); err != nil {
		return nil, errors.Wrapf(err, "cannot read result data from url %s", url)
	}
	result := &Metadata{}
	if err := json.Unmarshal(buf.Bytes(), result); err != nil {
		return nil, errors.Wrapf(err, "cannot unmarshal metadata for %s - %s", url, string(buf.Bytes()))
	}
	if result.Type == "" {
		return nil, errors.Errorf("invalid result from metadata (no type)")
	}
	return result, nil
}

func (ms *MediaserverMySQL) GetFulltext(collection, signature string) (string, error) {
	url, err := ms.GetUrl(collection, signature, "fulltext")
	if err != nil {
		return "", errors.Wrapf(err, "cannot build url %s", "metadata")
	}

	resp, err := http.Get(url)
	if err != nil {
		return "", errors.Wrapf(err, "cannot load url %s", url)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp.Body); err != nil {
		return "", errors.Wrapf(err, "cannot read result data from url %s", url)
	}
	return buf.String(), nil
}
