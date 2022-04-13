package iid

import (
	"database/sql"
	"fmt"
	"github.com/je4/zsearch/v2/pkg/search"
	"github.com/pkg/errors"
	"github.com/vanng822/go-solr/solr"
	"os"
	"strings"
	"time"
)

const IMAGEPATH = "/hgkstore/minio/iid-archive/public_html/cache"
const ARCHIVEPATH = "/hgkstore/minio/iid-archive/archive"
const FILEBASE = "file://ba14ns21403.fhnw.ch"
const MEDIASERVER_COLLECTION = "iidarchiv"

type Module struct {
	ID           int64
	Semester     string
	Modultyp     string
	Modulart     string
	Kennziffer   string
	ModuleName   string
	Titel        string
	Abstract     string
	Beschreibung string
	Bild         string
	Website      string
	Uebergabe    time.Time
	Von          time.Time
	Bis          time.Time
	Deleted      bool

	IID *IID
}

func (mod *Module) IsDeleted() bool {
	return mod.Deleted
}

func (mod *Module) Name() string {
	return "iid-Modul"
}

func (mod *Module) GetSource() string {
	return "iid"
}

func (mod *Module) GetSignature() string {
	return fmt.Sprintf("%s.%v", mod.Name(), mod.ID)
}

func (mod *Module) GetSignatureOriginal() string {
	return mod.Kennziffer
}

func (mod *Module) GetTitle() string {
	title := strings.TrimSpace(mod.Titel)
	if title == "" {
		title = mod.ModuleName
	}
	return title
}

func (mod *Module) GetSeries() string {
	return "iid-Module"
}

func (mod *Module) GetUrl() string {
	return mod.Website
}

func (mod *Module) GetPlace() string {
	if mod.Uebergabe.Year() < 2014 {
		return "Aarau"
	}
	return "Basel"

}

func (mod *Module) GetDate() string {
	return fmt.Sprintf("%v", mod.Uebergabe.Year())
}

func (mod *Module) GetDateAdded() time.Time {
	return time.Now()
}

func (mod *Module) GetCollectionTitle() string {
	return "Archiv Institut Industrial Design"
}

func (mod *Module) GetPublisher() string {
	return "Mediathek HGK"
}

func (mod *Module) GetPersons() []search.Person {
	var persons = []search.Person{}
	return persons
}

func (mod *Module) GetACL() map[string][]string {

	var acls = map[string][]string{
		"meta":    {"global/admin"},
		"content": {"global/admin"},
	}
	return acls
}

func (mod *Module) GetCatalogs() []string {
	var catalogs = []string{
		"mediathek",
		"iid",
	}
	return catalogs
}

func (mod *Module) GetCategories() []string {
	var categories = []string{}
	categories = append(categories, fmt.Sprintf("%s", mod.Name()))
	categories = append(categories, fmt.Sprintf("%s!!%v", mod.Name(), mod.Uebergabe.Year()))
	categories = append(categories, fmt.Sprintf("%s!!%v", mod.Name(), mod.Semester))
	categories = append(categories, fmt.Sprintf("%s!!%v", mod.Name(), mod.Modultyp))
	categories = append(categories, fmt.Sprintf("%s!!%v", mod.Name(), mod.Modulart))
	return categories
}

func (mod *Module) GetTags() []string {
	var tags = []string{}
	tags = search.AppendIfMissing(tags, fmt.Sprintf("%v", mod.Uebergabe.Year()))
	tags = search.AppendIfMissing(tags, "Modul")
	tags = search.AppendIfMissing(tags, mod.Modultyp)
	tags = search.AppendIfMissing(tags, mod.Modulart)
	return tags
}

// https://mediathek.hgk.fhnw.ch/indexer.ng/media.php?id=4.4419.2211214
// http://hdl.handle.net/20.500.11806/mediathek/inventory/B0000078520/4.4421.2227476

func (mod *Module) GetMedia() map[string]search.MediaList {
	if mod.IID == nil {
		return map[string]search.MediaList{}
	}
	medias := make(map[string]search.MediaList)

	if mod.Bild != "" {
		filename := fmt.Sprintf("%s/%s.jpg", IMAGEPATH, mod.Bild)
		if _, err := os.Stat(filename); err == nil {
		}
		if true {
			signature := fmt.Sprintf("module_%v_%s", mod.ID, mod.Bild)
			if err := mod.IID.mediaserver.CreateMasterUrl(MEDIASERVER_COLLECTION, signature, FILEBASE+filename); err != nil {

			}
			if metadata, err := mod.IID.mediaserver.GetMetadata(MEDIASERVER_COLLECTION, signature); err == nil {
				media := search.Media{
					Name:        fmt.Sprintf("#%s.%s", mod.Name(), mod.Kennziffer),
					Mimetype:    metadata.Mimetype,
					Type:        metadata.Type,
					Uri:         fmt.Sprintf("mediaserver:%s/%s", MEDIASERVER_COLLECTION, signature),
					Width:       metadata.Width,
					Height:      metadata.Height,
					Orientation: getOrientation(metadata),
					Duration:    metadata.Duration,
					Fulltext:    "",
				}

				if _, ok := medias[media.Type]; !ok {
					medias[media.Type] = []search.Media{}
				}

				medias[media.Type] = append(medias[media.Type], media)
			}
		}
	}
	return medias
}

func (mod *Module) GetPoster() *search.Media {

	if mod.IID == nil {
		return nil
	}
	medias := mod.GetMedia()
	for _, mediaList := range medias {
		for _, media := range mediaList {
			return &media
		}
	}
	return nil
}

func (mod *Module) GetNotes() []search.Note {
	notes := []search.Note{}

	return notes
}

func (mod *Module) GetAbstract() string {
	return fmt.Sprintf("%s\n\n%s", mod.Beschreibung, mod.Abstract)
}

func (mod *Module) GetRights() string {
	return "http://rightsstatements.org/vocab/InC/1.0/"
}

func (mod *Module) GetLicense() string {
	return ""
}

func (mod *Module) GetReferences() []search.Reference {
	var references []search.Reference
	return references
}

func (mod *Module) GetMeta() *search.Metalist {
	var result = search.Metalist{}
	result["kennziffer"] = mod.Kennziffer
	return &result
}

func (mod *Module) GetExtra() *search.Metalist {
	return mod.GetMeta()
}

func (mod *Module) GetVars() *search.Varlist {
	ml := search.Varlist{}
	return ml.Unique()
}

func (mod *Module) GetContentType() string {
	return "Module"
}

func (mod *Module) GetQueries() []search.Query {
	title := mod.GetCollectionTitle()
	queries := []search.Query{
		{
			Label:  fmt.Sprintf("%s", mod.GetTitle()),
			Search: fmt.Sprintf(`cat:"iid-Arbeit!!arbeit!!modul!!%v"`, mod.GetTitle()),
		},
		{
			Label:  fmt.Sprintf("%s - Arbeiten", title),
			Search: fmt.Sprintf(`cat:"%s"`, "iid-Arbeit"),
		},
		{
			Label:  fmt.Sprintf("%s - Module", title),
			Search: fmt.Sprintf(`cat:"iid-Modul"`),
		},
	}
	return queries
}

func (mod *Module) GetSolrDoc() *solr.Document {
	return nil
}

func (mod *Module) GetContentString() string {
	return ""

}

func (mod *Module) GetContentMime() string {
	return ""

}

func (mod *Module) Scan(rowX interface{}) error {
	var Titel sql.NullString
	var Abstract sql.NullString
	var Beschreibung sql.NullString
	var Bild sql.NullString
	var Uebergabe sql.NullString
	var Website sql.NullString
	var Von, Bis string

	var err error

	switch row := rowX.(type) {
	case *sql.Rows:
		if err := row.Scan(
			&mod.ID,
			&mod.Semester,
			&Von,
			&Bis,
			&mod.Modultyp,
			&mod.Modulart,
			&mod.Kennziffer,
			&mod.ModuleName,
			&Titel,
			&Abstract,
			&Beschreibung,
			&Bild,
			&Uebergabe,
			&Website,
			&mod.Deleted,
		); err != nil {
			return errors.Wrapf(err, "cannot scan value")
		}
	case *sql.Row:
		if err := row.Scan(
			&mod.ID,
			&mod.Semester,
			&Von,
			&Bis,
			&mod.Modultyp,
			&mod.Modulart,
			&mod.Kennziffer,
			&mod.ModuleName,
			&Titel,
			&Abstract,
			&Beschreibung,
			&Bild,
			&Uebergabe,
			&Website,
			&mod.Deleted,
		); err != nil {
			return errors.Wrapf(err, "cannot scan value")
		}
	default:
		return errors.New("invalid type for scanning data")
	}
	Von = strings.Replace(Von, "-00-00", "-01-01", -1)
	if mod.Von, err = time.Parse("2006-01-02", Von); err != nil {
		return errors.Wrapf(err, "invalid von date %s", Von)
	}
	Bis = strings.Replace(Von, "-00-00", "-12-31", -1)
	if mod.Bis, err = time.Parse("2006-01-02", Bis); err != nil {
		return errors.Wrapf(err, "invalid bis date %s", Bis)
	}
	mod.Titel = Titel.String
	mod.Abstract = Abstract.String
	mod.Beschreibung = Beschreibung.String
	mod.Bild = Bild.String
	if Uebergabe.String != "" && Uebergabe.String != "0000-00-00" {
		mod.Uebergabe, err = time.Parse("2006-01-02", Uebergabe.String)
		if err != nil {
			return errors.Wrapf(err, "invalid Uebergabe time format %s", Uebergabe.String)
		}
	}
	mod.Website = Website.String

	return nil
}
