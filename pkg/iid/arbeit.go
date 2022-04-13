package iid

import (
	"database/sql"
	"fmt"
	"github.com/je4/zsearch/v2/pkg/search"
	"github.com/pkg/errors"
	"github.com/vanng822/go-solr/solr"
	"html/template"
	"os"
	"regexp"
	"strings"
	"time"
)

type Material struct {
	Name, Klassierung, Dateiname string
}

type Arbeit struct {
	ID                                        int64
	Modulkennziffer                           string
	ModuleID                                  int64
	Titel                                     string
	Kurzbeschreibung, Abstract, Beschreibung  string
	Zusammenarbeit, Recherche, Auszeichnungen string
	BemerkungenArchiv                         string
	ExternesMaterial                          string
	Website                                   string
	Kontakt                                   string
	Genre                                     []string
	Techniken                                 []string
	Klassierung                               []string
	Mittel                                    []string
	Studiensemester                           []string
	Status                                    string
	Pfad                                      string
	Bild1, Bild2, Bild3                       string
	AdditionalDokumentation                   []Material
	Module                                    *Module
	ModDate                                   time.Time

	Persons []search.Person

	iid *IID
}

func (a *Arbeit) Name() string {
	return "iid-Arbeit"
}

func (a *Arbeit) GetSource() string {
	return fmt.Sprintf("%s", a.Name())
}

func (a *Arbeit) GetSignature() string {
	return fmt.Sprintf("%s-%v", a.Name(), a.ID)
}

func (a *Arbeit) GetSignatureOriginal() string {
	return fmt.Sprintf("%v", a.ID)
}

func (a *Arbeit) GetTitle() string {
	title := a.Titel
	return title
}

func (a *Arbeit) GetSeries() string {
	return a.Module.GetTitle()
}

func (a *Arbeit) GetUrl() string {
	return a.Website
}

func (a *Arbeit) GetPlace() string {
	if a.Module.Uebergabe.Year() < 2014 {
		return "Aarau"
	}
	return "Basel"
}

func (a *Arbeit) GetDate() string {
	return fmt.Sprintf("%v", a.Module.Uebergabe.Year())
}

func (a *Arbeit) GetDateAdded() time.Time {
	return a.ModDate
}

func (a *Arbeit) GetCollectionTitle() string {
	return "Archiv Institut Industrial Design"
}

func (a *Arbeit) GetPublisher() string {
	return "Mediathek HGK"
}

func (a *Arbeit) GetPersons() []search.Person {
	return a.Persons
}

// Name:value
var zoteroTagACLVariable = regexp.MustCompile(`^acl_(meta|content|preview):(.+)$`)

func (a *Arbeit) GetACL() map[string][]string {
	var acls = map[string][]string{
		"meta":    {"global/admin"},
		"content": {"global/admin"},
	}
	online := false
	for _, k := range a.Klassierung {
		switch k {
		case "PR":
			online = true
		case "WEB":
			online = true
		}
	}
	if online {
		// todo: do the right rights
		//		acls["meta"] = append(acls["meta"], "global/guest")
		//		acls["content"] = append(acls["content"], "global/guest")
	}
	return acls
}

func (a *Arbeit) GetCatalogs() []string {
	var catalogs = []string{
		"mediathek",
		"iidarchiv",
	}
	return catalogs
}

func (a *Arbeit) GetCategories() []string {
	var categories = []string{}

	categories = append(categories, fmt.Sprintf("%s", a.Name()))
	categories = append(categories, fmt.Sprintf("%s!!arbeit", a.Name()))
	categories = append(categories, fmt.Sprintf("%s!!arbeit!!modul!!%s", a.Name(), a.Module.GetTitle()))
	for _, k := range a.Mittel {
		categories = append(categories, fmt.Sprintf("%s!!arbeit!!mittel!!%s", a.Name(), k))
	}
	for _, s := range a.Studiensemester {
		categories = append(categories, fmt.Sprintf("%s!!arbeit!!semester!!%s", a.Name(), s))
	}
	for _, t := range a.Techniken {
		categories = append(categories, fmt.Sprintf("%s!!arbeit!!technik!!%s", a.Name(), t))
	}

	return categories
}

func (a *Arbeit) GetTags() []string {
	var tags = []string{}

	for _, t := range a.Techniken {
		tags = append(tags, fmt.Sprintf("%s", t))
	}

	return tags
}

// https://mediathek.hgk.fhnw.ch/indexer.ng/media.php?id=4.4419.2211214
// http://hdl.handle.net/20.500.11806/mediathek/inventory/B0000078520/4.4421.2227476

func (a *Arbeit) GetMedia() map[string]search.MediaList {
	if a.iid == nil {
		return map[string]search.MediaList{}
	}
	medias := make(map[string]search.MediaList)

	for _, bild := range []string{a.Bild1, a.Bild2, a.Bild3} {
		if bild == "" {
			continue
		}
		filename := fmt.Sprintf("%s/%s/images/%s.tif", ARCHIVEPATH, a.Pfad, bild)
		if _, err := os.Stat(filename); err == nil {
		}
		if true {
			signature := fmt.Sprintf("arbeit_%v_%s", a.ID, bild)
			if err := a.iid.mediaserver.CreateMasterUrl(MEDIASERVER_COLLECTION, signature, FILEBASE+filename); err != nil {
				continue
			}
			metadata, err := a.iid.mediaserver.GetMetadata(MEDIASERVER_COLLECTION, signature)
			if err != nil {
				continue
			}
			media := search.Media{
				Name:        fmt.Sprintf("#%s.%v", a.Name(), a.ID),
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
	return medias
}

var mediaserverRegexp = regexp.MustCompile("^mediaserver:([^/]+)/(.+)$")

func (a *Arbeit) GetPoster() *search.Media {
	if a.iid == nil {
		return nil
	}
	medias := a.GetMedia()
	if _, ok := medias["video"]; ok {
		if len(medias["video"]) > 0 {
			vid := medias["video"][0]
			if matches := mediaserverRegexp.FindStringSubmatch(vid.Uri); matches != nil {
				collection := matches[1]
				signature := fmt.Sprintf("%s$$timeshot$$3", matches[2])
				metadata, err := a.iid.mediaserver.GetMetadata(collection, signature)
				if err == nil {
					return &search.Media{
						Name:        "poster",
						Mimetype:    metadata.Mimetype,
						Type:        metadata.Type,
						Uri:         fmt.Sprintf("mediaserver:%v/%v", collection, signature),
						Width:       metadata.Width,
						Height:      metadata.Height,
						Orientation: getOrientation(metadata),
						Duration:    metadata.Duration,
					}
				}
			}
		}
	}
	if _, ok := medias["image"]; ok {
		for _, media := range medias["image"] {
			if strings.ToLower(media.Name) == "poster" {
				return &media
			}
		}
		if len(medias["image"]) > 0 {
			return &(medias["image"][0])
		}
	}
	if _, ok := medias["audio"]; ok {
		if len(medias["audio"]) > 0 {
			aud := medias["audio"][0]
			if matches := mediaserverRegexp.FindStringSubmatch(aud.Uri); matches != nil {
				collection := matches[1]
				signature := fmt.Sprintf("%s$$poster", matches[2])
				metadata, err := a.iid.mediaserver.GetMetadata(collection, signature)
				if err == nil {
					return &search.Media{
						Name:     "poster",
						Mimetype: metadata.Mimetype,
						Type:     metadata.Type,
						Uri:      fmt.Sprintf("mediaserver:%v/%v", collection, signature),
						Width:    metadata.Width,
						Height:   metadata.Height,
						Duration: metadata.Duration,
					}
				}
			}
		}
	}
	return nil
}

func (a *Arbeit) GetNotes() []search.Note {
	notes := []search.Note{}
	if a.BemerkungenArchiv != "" {
		notes = append(notes, search.Note{
			Title: "Archiv",
			Note:  template.HTML(a.BemerkungenArchiv),
		})
	}
	if a.ExternesMaterial != "" {
		notes = append(notes, search.Note{
			Title: "ext. Material",
			Note:  template.HTML(a.ExternesMaterial),
		})
	}
	if a.Auszeichnungen != "" {
		notes = append(notes, search.Note{
			Title: "Auszeichnungen",
			Note:  template.HTML(a.Auszeichnungen),
		})
	}
	if a.Recherche != "" {
		notes = append(notes, search.Note{
			Title: "Recherche",
			Note:  template.HTML(a.Recherche),
		})
	}
	if a.Zusammenarbeit != "" {
		notes = append(notes, search.Note{
			Title: "Zusammenarbeit",
			Note:  template.HTML(a.Zusammenarbeit),
		})
	}
	return notes
}

func (a *Arbeit) GetAbstract() string {
	var beschreibung string

	if a.Kurzbeschreibung != "" {
		beschreibung += a.Kurzbeschreibung
	}

	if a.Abstract != "" {
		beschreibung += "\n\n" + a.Abstract
	}

	if a.Beschreibung != "" {
		beschreibung += "\n\n" + a.Beschreibung
	}
	return strings.TrimSpace(beschreibung)
}

func (a *Arbeit) GetRights() string {
	return ""
}

var urlRegex = regexp.MustCompile(`(https?:\/\/[^@\/\n]+[^: ;):\n]+[^: ;)?:\n.])`)

func (a *Arbeit) GetLicense() string {
	return "http://rightsstatements.org/vocab/InC/1.0/"
}

var zoterolinkregexp = regexp.MustCompile("^https?://zotero.org/groups/([^/]+)/items/([^/]+)$")

func (a *Arbeit) GetReferences() []search.Reference {
	var references = []search.Reference{
		{
			Type:      "Module",
			Title:     fmt.Sprintf("Modul: %s", a.Module.GetTitle()),
			Signature: a.Module.GetSignature(),
		},
	}
	return references
}

func (a *Arbeit) GetMeta() *search.Metalist {
	var result = search.Metalist{}
	result["status"] = a.Status
	result["semester"] = strings.Join(a.Studiensemester, " // ")
	for count, k := range a.Klassierung {
		result[fmt.Sprintf("klassierung.%v", count)] = k
	}
	return &result
}

func (a *Arbeit) GetExtra() *search.Metalist {
	return a.GetMeta()
}

func (a *Arbeit) GetVars() *search.Varlist {
	ml := search.Varlist{}

	ml["mittel"] = []string{}
	for _, k := range a.Mittel {
		ml["mittel"] = append(ml["mittel"], k)
	}
	ml["studiensemester"] = []string{}
	for _, s := range a.Studiensemester {
		ml["studiensemester"] = append(ml["studiensemester"], s)
	}
	ml["technik"] = []string{}
	for _, s := range a.Techniken {
		ml["technik"] = append(ml["technik"], s)
	}
	return ml.Unique()
}

func (a *Arbeit) GetContentType() string {
	return "Work"
}

func (a *Arbeit) GetQueries() []search.Query {
	queries := []search.Query{}

	appendQuery := func(qs []search.Query, newqueries ...search.Query) []search.Query {
		for _, newquery := range newqueries {
			found := false
			for _, q := range qs {
				if q.Search == newquery.Search {
					found = true
					break
				}
			}
			if !found {
				qs = append(qs, newquery)
			}
		}
		return qs
	}

	title := a.GetCollectionTitle()
	queries = appendQuery(queries,
		search.Query{
			Label:  fmt.Sprintf("%s - Modul - %s", title, a.Module.GetTitle()),
			Search: fmt.Sprintf(`cat:"%s!!arbeit!!modul!!%s"`, a.Name(), a.Module.GetTitle()),
		},
		search.Query{
			Label:  fmt.Sprintf("%s - Arbeiten", title),
			Search: fmt.Sprintf(`cat:"%s"`, a.Name()),
		},
		search.Query{
			Label:  fmt.Sprintf("%s - Module", title),
			Search: fmt.Sprintf(`cat:"iid-Modul"`),
		},
	)
	return queries
}

func (a *Arbeit) GetSolrDoc() *solr.Document {
	return nil
}

func (a *Arbeit) GetContentString() string {
	return ""

}

func (a *Arbeit) GetContentMime() string {
	return ""

}

func (a *Arbeit) Scan(rowX interface{}) error {
	var HauptReferent, Referent sql.NullString
	var ModDate string
	switch row := rowX.(type) {
	case *sql.Rows:
		if err := row.Scan(&a.ID, &HauptReferent, &Referent, &a.ModuleID, &a.Titel, &a.Kurzbeschreibung, &a.Abstract,
			&a.Beschreibung, &a.Zusammenarbeit, &a.Recherche, &a.Auszeichnungen, &a.BemerkungenArchiv, &a.Kontakt,
			&a.Pfad, &a.Bild1, &a.Bild2, &a.Bild3, &a.Website, &a.ExternesMaterial, &a.Modulkennziffer, &ModDate); err != nil {
			return errors.Wrap(err, "cannot scan row")
		}
	case *sql.Row:
		if err := row.Scan(&a.ID, &HauptReferent, &Referent, &a.ModuleID, &a.Titel, &a.Kurzbeschreibung, &a.Abstract,
			&a.Beschreibung, &a.Zusammenarbeit, &a.Recherche, &a.Auszeichnungen, &a.BemerkungenArchiv, &a.Kontakt,
			&a.Pfad, &a.Bild1, &a.Bild2, &a.Bild3, &a.Website, &a.ExternesMaterial, &a.Modulkennziffer, &ModDate); err != nil {
			return errors.Wrap(err, "cannot scan row")
		}
	default:
		return errors.New("invalid row type")
	}
	a.ModDate, _ = time.Parse("2006-02-01 15:04:05", ModDate)
	if HauptReferent.Valid {
		a.Persons = append(a.Persons, search.Person{
			Name: HauptReferent.String,
			Role: "Hauptreferent",
		})
	}
	if Referent.Valid {
		a.Persons = append(a.Persons, search.Person{
			Name: Referent.String,
			Role: "Referent",
		})
	}
	rows, err := a.iid.BenutzerStmt.Query(a.ID)
	if err != nil {
		return errors.Wrapf(err, "cannot query BenutzerStmt: %v", err)
	}
	for rows.Next() {
		p := search.Person{}
		if err := rows.Scan(&p.Role, &p.Name); err != nil {
			rows.Close()
			return errors.Wrap(err, "cannot scan values")
		}
		a.Persons = append(a.Persons, p)
	}
	rows.Close()
	rows, err = a.iid.TechnikenStmt.Query(a.ID)
	if err != nil {
		return errors.Wrapf(err, "cannot query TechnikenStmt: %v", err)
	}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			rows.Close()
			return errors.Wrap(err, "cannot scan values")
		}
		a.Techniken = append(a.Techniken, t)
	}
	rows.Close()
	rows, err = a.iid.KlassierungenStmt.Query(a.ID)
	if err != nil {
		return errors.Wrapf(err, "cannot execute KlassierungenStmt")
	}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			rows.Close()
			return errors.Wrap(err, "cannot scan values")
		}
		a.Klassierung = append(a.Klassierung, k)
	}
	rows.Close()
	rows, err = a.iid.StudiensemesterStmt.Query(a.ID)
	if err != nil {
		return errors.Wrapf(err, "cannot execute StudiensemesterStmt")
	}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			rows.Close()
			return errors.Wrap(err, "cannot scan values")
		}
		a.Studiensemester = append(a.Studiensemester, s)
	}
	rows, err = a.iid.MittelStmt.Query(a.ID)
	if err != nil {
		return errors.Wrapf(err, "cannot execute MittelStmt")
	}
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			rows.Close()
			return errors.Wrap(err, "cannot scan values")
		}
		a.Mittel = append(a.Mittel, m)
	}
	rows.Close()
	if err := a.iid.StatiStmt.QueryRow(a.ID).Scan(&a.Status); err != nil {
		if sql.ErrNoRows != err {
			return errors.Wrapf(err, "cannot execute StatiStmt")
		}
	}
	if a.Module, err = a.iid.LoadModule(a.ModuleID); err != nil {
		return errors.Wrapf(err, "cannot load module for arbeit %v", a.ID)
	}
	return nil
}
