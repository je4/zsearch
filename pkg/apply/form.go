package apply

import (
	"fmt"
	"github.com/gosimple/slug"
	"github.com/je4/zsearch/v2/pkg/search"
	"github.com/vanng822/go-solr/solr"
	"golang.org/x/exp/slices"
	"html/template"
	"path"
	"regexp"
	"strings"
	"time"
)

type FormFile struct {
	Id       int64
	Mimetype string
	Name     string
	Filename string
	Size     int64
}

type FormData struct {
	Key   string
	Value string
}

type Form struct {
	Id           int64
	Link         string
	Changed      time.Time
	Files        []*FormFile
	Data         map[string]string
	Project      string
	ProjectTitel string
	apply        *Apply
	Errors       []string
}

func (form *Form) Name() string {
	return form.Project
}

func (form *Form) GetSource() string {
	return form.Project
}

func (form *Form) GetSignature() string {
	return fmt.Sprintf("%s-%04d", form.Name(), form.Id)
}

func (form *Form) GetSignatureOriginal() string {
	return fmt.Sprintf("%v", form.Id)
}

func (form *Form) GetTitle() string {
	title := strings.TrimSpace(form.Data["titel"])
	if title == "" {
		title = "ohne Titel"
	}
	return title
}

func (form *Form) GetSeries() string {
	return form.ProjectTitel
}

func (form *Form) GetUrl() string {
	web := strings.TrimSpace(form.Data["web"])
	if !strings.HasPrefix("http", strings.ToLower(web)) && web != "" {
		web = "https://" + web
	}
	return web
}

func (form *Form) GetPlace() string {
	return form.Data["eventplace"]
}

func (form *Form) GetDate() string {
	return form.Data["year"]
}

func (form *Form) GetDateAdded() time.Time {
	return time.Now()
}

func (form *Form) GetCollectionTitle() string {
	return form.ProjectTitel
}

func (form *Form) GetPublisher() string {
	return "Mediathek HGK"
}

var pRoleRegex = regexp.MustCompile("([^(]+)\\(([^)]+)\\)")
var bracketRegexp = regexp.MustCompile("\\(([^\\)]+)\\)")

func extractPerson(pString, defaultRole string) []search.Person {
	ret := []search.Person{}
	if strings.TrimSpace(pString) == "" {
		return ret
	}
	pString2 := bracketRegexp.ReplaceAllStringFunc(pString, func(s string) string {
		return strings.Replace(s, ";", ",", -1)
	})
	ps := strings.Split(pString2, ";")
	for _, p := range ps {
		elems := pRoleRegex.FindStringSubmatch(p)
		if elems != nil {
			name := strings.TrimSpace(elems[1])
			es := strings.Split(elems[2], ";")
			for _, e := range es {
				ret = append(ret, search.Person{Name: name, Role: fmt.Sprintf("%s:%s", defaultRole, strings.TrimSpace(e))})
			}
		} else {
			ret = append(ret, search.Person{Name: strings.TrimSpace(p), Role: strings.TrimSpace(defaultRole)})
		}
	}
	return ret
}
func (form *Form) GetPersons() []search.Person {
	var persons = []search.Person{}
	persons = append(persons, extractPerson(form.Data["artists"], "artist")...)
	persons = append(persons, extractPerson(form.Data["performers"], "performer")...)
	persons = append(persons, extractPerson(form.Data["eventcurator"], "eventcurator")...)
	persons = append(persons, extractPerson(form.Data["camera"], "camera")...)
	/*
		persons = append(persons, search.Person{
			Name: fmt.Sprintf("%s, %s", form.Data["nachname"], form.Data["vorname"]),
			Role: "contact",
		})
	*/
	return persons
}

func (form *Form) GetACL() map[string][]string {

	var acls = map[string][]string{}
	acls["meta"] = []string{"global/admin", "global/guest"}
	if form.Data["rechtemediathek"] != "ok" {
		acls["content"] = []string{"global/admin"}
	} else {
		acls["content"] = []string{"global/admin", "global/guest"}
		/*
			if strings.TrimSpace(form.Data["visibility"]) == "1" {
				acls["content"] = []string{"global/admin", "global/guest"}
			} else {
				acls["content"] = []string{"global/admin", "hgk/bangbang", "hgk/mediathek"}
			}
		*/
	}
	return acls
}

func (form *Form) GetCatalogs() []string {
	var catalogs = []string{
		"mediathek",
		form.Project,
	}
	return catalogs
}

func (form *Form) GetCategories() []string {
	var categories = []string{}
	categories = append(categories, fmt.Sprintf("%s", form.Name()))
	categories = append(categories, fmt.Sprintf("%s!!%v", form.Name(), form.GetDate()))
	return categories
}

func (form *Form) GetTags() []string {
	var tags = []string{"vwg:declare"}
	tags = search.AppendIfMissing(tags, fmt.Sprintf("%v", form.GetDate()))
	tags = search.AppendIfMissing(tags, form.Data["medium"])
	if strings.HasPrefix(strings.ToLower(form.GetTitle()), "bangbang:") {
		tags = search.AppendIfMissing(tags, "BangBang Production")
	}
	return tags
}

// https://mediathek.hgk.fhnw.ch/indexer.ng/media.php?id=4.4419.2211214
// http://hdl.handle.net/20.500.11806/mediathek/inventory/B0000078520/4.4421.2227476

func (form *Form) GetMedia() map[string]search.MediaList {
	if form.apply == nil {
		return map[string]search.MediaList{}
	}

	medias := make(map[string]search.MediaList)

	public := false
	//	public := strings.TrimSpace(form.Data["visibility"]) == "1"

	if cACL, ok := form.GetACL()["content"]; ok {
		public = slices.Contains(cACL, "global/guest")
	}

	for _, file := range form.Files {
		if true {

			signature := fmt.Sprintf("%04d_%05d_%s%s", form.Id, file.Id,
				slug.Make(strings.TrimSuffix(path.Base(file.Name), path.Ext(file.Name))), path.Ext(file.Name))
			if err := form.apply.mediaserver.CreateMasterUrl(
				form.apply.mediaserverCollection,
				signature,
				fmt.Sprintf("https://mediathek.hgk.fhnw.ch/apply/bangbang/%s", strings.TrimPrefix(file.Filename, "/")),
				public); err != nil {

			}
			if metadata, err := form.apply.mediaserver.GetMetadata(form.apply.mediaserverCollection, signature); err != nil {
				form.apply.logger.Errorf("cannot get metadata for signature %s: %v", err)
			} else {
				fulltext, _ := form.apply.mediaserver.GetFulltext(form.apply.mediaserverCollection, signature)
				media := search.Media{
					Name:        fmt.Sprintf("#%v.%v", form.Id, file.Id),
					Mimetype:    metadata.Mimetype,
					Type:        metadata.Type,
					Uri:         fmt.Sprintf("mediaserver:%s/%s", form.apply.mediaserverCollection, signature),
					Width:       metadata.Width,
					Height:      metadata.Height,
					Orientation: getOrientation(metadata),
					Duration:    metadata.Duration,
					Fulltext:    fulltext,
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

var mediaserverRegexp = regexp.MustCompile("^mediaserver:([^/]+)/(.+)$")

func (form *Form) GetPoster() *search.Media {
	if form.apply == nil {
		return nil
	}
	medias := form.GetMedia()
	for _, mediaList := range medias {
		for _, media := range mediaList {
			switch strings.ToLower(media.Type) {
			case "image":
				return &media
			}
		}
	}
	for _, mediaList := range medias {
		for _, media := range mediaList {
			switch strings.ToLower(media.Type) {
			case "video":
				matches := mediaserverRegexp.FindStringSubmatch(media.Uri)
				if matches == nil {
					continue
				}
				metadata, err := form.apply.mediaserver.GetMetadata(matches[1], matches[2]+"$$timeshot$$3")
				if err != nil {
					continue
				}
				return &search.Media{
					Name:     matches[2],
					Mimetype: metadata.Mimetype,
					Type:     metadata.Type,
					Uri:      fmt.Sprintf("mediaserver:%s/%s", matches[1], matches[2]+"$$timeshot$$3"),
					Width:    metadata.Width,
					Height:   metadata.Height,
					Duration: metadata.Duration,
				}
			}
		}
	}
	for _, mediaList := range medias {
		for _, media := range mediaList {
			switch strings.ToLower(media.Type) {
			case "audio":
				matches := mediaserverRegexp.FindStringSubmatch(media.Uri)
				if matches == nil {
					continue
				}
				metadata, err := form.apply.mediaserver.GetMetadata(matches[1], matches[2]+"$$poster")
				if err != nil {
					continue
				}
				return &search.Media{
					Name:     matches[2],
					Mimetype: metadata.Mimetype,
					Type:     metadata.Type,
					Uri:      fmt.Sprintf("mediaserver:%s/%s", matches[1], matches[2]+"$$poster"),
					Width:    metadata.Width,
					Height:   metadata.Height,
					Duration: metadata.Duration,
				}
			}
		}
	}
	for _, mediaList := range medias {
		for _, media := range mediaList {
			switch strings.ToLower(media.Type) {
			case "pdf":
				matches := mediaserverRegexp.FindStringSubmatch(media.Uri)
				if matches == nil {
					continue
				}
				metadata, err := form.apply.mediaserver.GetMetadata(matches[1], matches[2]+"$$poster")
				if err != nil {
					continue
				}
				return &search.Media{
					Name:     matches[2],
					Mimetype: metadata.Mimetype,
					Type:     metadata.Type,
					Uri:      fmt.Sprintf("mediaserver:%s/%s", matches[1], matches[2]+"$$poster"),
					Width:    metadata.Width,
					Height:   metadata.Height,
					Duration: metadata.Duration,
				}
			}
		}
	}
	return nil
}

func (form *Form) GetNotes() []search.Note {
	notes := []search.Note{}
	if strings.TrimSpace(form.Data["additional"]) != "" {
		notes = append(notes, search.Note{Title: "Additional", Note: template.HTML(form.Data["additional"])})
	}
	if strings.TrimSpace(form.Data["remark"]) != "" {
		notes = append(notes, search.Note{Title: "Remark", Note: template.HTML(form.Data["remark"])})
	}
	for _, errStr := range form.Errors {
		notes = append(notes, search.Note{Note: template.HTML(errStr), Title: "Error"})
	}
	return notes
}

func (form *Form) GetAbstract() string {
	return form.Data["descr"]
}

func (form *Form) GetRights() string {
	return "http://rightsstatements.org/vocab/InC/1.0/"
}

func (form *Form) GetLicense() string {
	return ""
}

func (form *Form) GetReferences() []search.Reference {
	var references = []search.Reference{
		search.Reference{
			Type:      "url",
			Title:     "BANG BANG Page",
			Signature: fmt.Sprintf("https://mediathek.hgk.fhnw.ch/bangbang/detail/%s", form.GetSignature()),
		},
	}
	return references
}

func (form *Form) GetMeta() *search.Metalist {
	var extra = search.Metalist{}
	var meta = (*search.Metalist)(&form.Data)
	if meta != nil {
		for key, val := range *meta {
			switch key {
			case "descr":
			case "titel":
			case "artists":
			case "year":
			case "web":
			case "additional":
			case "adresse":
			case "email":
			case "rechtebangbang":
			case "rechtemediathek":
			case "tel":
			case "vorname":
			case "nachname":
			default:
				extra[key] = val
			}
		}
	}
	return &extra

}

func (form *Form) GetAllMeta() *search.Metalist {
	return (*search.Metalist)(&form.Data)
}

func (form *Form) GetExtra() *search.Metalist {
	var extra = search.Metalist{}
	var meta = form.GetMeta()
	if meta != nil {
		for key, val := range *meta {
			switch key {
			case "descr":
			case "titel":
			case "artists":
			case "year":
			case "web":
			case "additional":
			case "adresse":
			case "email":
			case "rechtebangbang":
			case "rechtemediathek":
			case "tel":
			case "vorname":
			case "nachname":
			default:
				extra[key] = val
			}
		}
	}
	return &extra
}

func (form *Form) GetVars() *search.Varlist {
	ml := search.Varlist{}
	return ml.Unique()
}

func (form *Form) GetContentType() string {
	return "Performance"
}

func (form *Form) GetQueries() []search.Query {
	queries := []search.Query{
		{
			Label:  fmt.Sprintf("Bang Bang"),
			Search: fmt.Sprintf(`cat:"%s"`, "bangbang"),
		},
	}
	return queries
}

func (form *Form) GetSolrDoc() *solr.Document {
	return nil
}

func (form *Form) GetContentString() string {
	return ""

}

func (form *Form) GetContentMime() string {
	return ""

}
