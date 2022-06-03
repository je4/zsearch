package main

import (
	"encoding/csv"
	"fmt"
	"github.com/Masterminds/sprig"
	"github.com/je4/zsearch/v2/pkg/apply"
	"github.com/je4/zsearch/v2/pkg/mediaserver"
	"github.com/je4/zsearch/v2/pkg/search"
	"github.com/op/go-logging"
	"github.com/pkg/errors"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var mediaserverRegexp = regexp.MustCompile("^mediaserver:([^/]+)/([^/]+)/(.+)$")

func mediaUrl(logger *logging.Logger, exportPath string, ms mediaserver.Mediaserver, folder, extension, mediaserverUrl string) (string, error) {
	logger.Infof("creating path for %s", mediaserverUrl)
	matches := mediaserverRegexp.FindStringSubmatch(mediaserverUrl)
	if matches == nil {
		logger.Errorf("invalid url format: %s", mediaserverUrl)
		return "", errors.New(fmt.Sprintf("invalid url: %s", mediaserverUrl))
	}
	collection := matches[1]
	signature := matches[2]
	function := matches[3]
	if extension == "" {
		ppos := strings.LastIndex(signature, ".")
		if ppos > 0 {
			extension = signature[ppos+1:]
		}
	}
	filename := strings.ToLower(fmt.Sprintf("%s_%s_%s.%s",
		collection,
		strings.ReplaceAll(signature, "$", "-"),
		strings.ReplaceAll(function, "/", "_"),
		strings.TrimPrefix(extension, ".")))
	if len(filename) > 203 {
		filename = fmt.Sprintf("%s-_-%s", filename[:100], filename[len(filename)-100:])
	}
	fullpath := filepath.Join(exportPath, folder, filename)
	if stat, err := os.Stat(fullpath); err == nil {
		if stat.Size() >= 2*1024 {
			logger.Infof("file already exists: %s", fullpath)
			return filename, nil
		}
		logger.Infof("removing small file: %s", fullpath)
		os.Remove(fullpath)
	}
	msUrl, err := ms.GetUrl(collection, signature, function)
	if err != nil {
		return "", err
	}
	logger.Infof("loading media: %s", msUrl)
	client := http.Client{
		Timeout: 3600 * time.Second,
	}
	resp, err := client.Get(msUrl)
	if err != nil {
		return "", errors.Wrapf(err, "cannot load url %s", msUrl)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		logger.Errorf("cannot get image: %v - %s", resp.StatusCode, resp.Status)
		return "", errors.New(fmt.Sprintf("cannot get image: %v - %s", resp.StatusCode, resp.Status))
	}
	file, err := os.OpenFile(fullpath, os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		return "", errors.Wrapf(err, "cannot open %s", fullpath)
	}
	defer file.Close()
	if _, err := io.Copy(file, resp.Body); err != nil {
		logger.Errorf("cannot read result data from url %s: %v", msUrl, err)
		return "", errors.Wrapf(err, "cannot read result data from url %s", msUrl)
	}
	return filename, nil
}

func writeData(logger *logging.Logger, full bool, listTemplatePath, detailTemplatePath, tableTemplatePath, exportPath string, ms mediaserver.Mediaserver, data []*search.SourceData, indexOnly bool) error {
	funcMap := sprig.FuncMap()
	funcMap["mediaUrl"] = func(mediaserverUrl, folder, extension string) (url string, err error) {
		return mediaUrl(logger, exportPath, ms, folder, extension, mediaserverUrl)
	}
	funcMap["correctWeb"] = func(u string) string {
		if strings.HasPrefix(strings.ToLower(u), "http") {
			return u
		}
		return "https://" + u
	}
	listTemplate := template.Must(template.New("list.gohtml").Funcs(funcMap).ParseFiles(listTemplatePath))
	detailTemplate := template.Must(template.New("detail.gohtml").Funcs(funcMap).ParseFiles(detailTemplatePath))
	tableTemplate := template.Must(template.New("table.gohtml").Funcs(funcMap).ParseFiles(tableTemplatePath))

	fullpath := filepath.Join(exportPath, "index.html")
	file, err := os.OpenFile(fullpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return errors.Wrapf(err, "cannot open %s", fullpath)
	}

	if err := listTemplate.Execute(file, struct {
		Items []*search.SourceData
	}{
		Items: data,
	}); err != nil {
		file.Close()
		return errors.Wrapf(err, "cannot execute template %s", listTemplatePath)
	}
	file.Close()

	fullpath = filepath.Join(exportPath, "table.html")
	file, err = os.OpenFile(fullpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return errors.Wrapf(err, "cannot open %s", fullpath)
	}

	if err := tableTemplate.Execute(file, struct {
		Items []*search.SourceData
	}{
		Items: data,
	}); err != nil {
		file.Close()
		return errors.Wrapf(err, "cannot execute template %s", tableTemplate)
	}
	file.Close()

	if !indexOnly {
		for _, item := range data {
			folder := filepath.Join(exportPath, "werke", item.SignatureOriginal)
			for _, dir := range []string{"master", "derivate"} {
				f := filepath.Join(folder, dir)
				if err := os.MkdirAll(f, os.ModePerm); err != nil {
					return errors.Wrapf(err, "cannot create %s", f)
				}
			}
			fullpath := filepath.Join(folder, "index.html")

			file, err := os.OpenFile(fullpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
			if err != nil {
				return errors.Wrapf(err, "cannot open %s", fullpath)
			}
			defer file.Close()
			if err := detailTemplate.Execute(file, struct {
				Full bool
				Item *search.SourceData
			}{
				Full: full,
				Item: item,
			}); err != nil {
				return errors.Wrapf(err, "cannot execute template %s", detailTemplatePath)
			}

			/*
				for field, _ := range *item.Meta {
					found := false
					for _, fld := range fields {
						if fld == field {
							found = true
							break
						}
					}
					if !found {
						fields = append(fields, field)
					}
				}
			*/
		}
	}
	return nil
}

var pGroupRegex = regexp.MustCompile("([^[]+)\\[([^\\]]+)\\]")

func writePersons(exportPath string, data []*apply.Form) error {
	// csv generation
	var persons = map[string][]string{}
	var appendPerson = func(role string, names ...string) {
		if _, ok := persons[role]; !ok {
			persons[role] = []string{}
		}
		for _, name := range names {
			found := false
			for _, n := range persons[role] {
				if n == name {
					found = true
					break
				}
			}
			if !found {
				persons[role] = append(persons[role], name)
			}
		}
	}

	for _, item := range data {
		for _, p := range item.GetPersons() {
			appendPerson(p.Role, p.Name)
		}
	}
	fields := []string{"role", "subrole", "name"}

	fullpath := filepath.Join(exportPath, "bangbang_names.csv")
	file, err := os.OpenFile(fullpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return errors.Wrapf(err, "cannot open %s", fullpath)
	}

	cw := csv.NewWriter(file)
	defer func() {
		cw.Flush()
		file.Close()
	}()
	cw.Write(fields)
	for role, names := range persons {
		for _, name := range names {
			roles := strings.Split(role, ":")
			r := roles[0]
			subr := ""
			if len(roles) > 1 {
				subr = roles[1]
			}
			if err := cw.Write([]string{r, subr, name}); err != nil {
				return errors.Wrapf(err, "cannot write to %s", "bangbang_names.csv")
			}
		}
	}
	return nil
}
func writeCSV(exportPath string, data []*apply.Form) error {

	// csv generation
	fields := []string{"artists",
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
	fullpath := filepath.Join(exportPath, "bangbang.csv")
	file, err := os.OpenFile(fullpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return errors.Wrapf(err, "cannot open %s", fullpath)
	}

	cw := csv.NewWriter(file)
	defer func() {
		cw.Flush()
		file.Close()
	}()
	/*
			var artists, camera string
		for _, p := range i.GetPersons() {
			switch p.Role {
			case "castMember":
				camera += fmt.Sprintf(" %s;", p.Name)
			default:
				artists += fmt.Sprintf(" %s;", p.Name)
			}
		}
		artists = strings.Trim(artists, " ;")
		camera = strings.Trim(camera, " ;")
		(*i.Meta)["artists"] = artists
		(*i.Meta)["camera"] = camera
		(*i.Meta)["medium"] = "Video"

	*/

	cw.Write(append([]string{"id", "delete", "update"}, fields...))
	for _, item := range data {
		content := item.GetAllMeta()
		record := []string{fmt.Sprintf("%d", item.Id), "", ""}
		for _, fld := range fields {
			var value = (*content)[fld]
			/*
				switch fld {
				case "titel":
					value = item.GetTitle()
				case "artists":
					persons := item.GetPersons()
					for _, person := range persons {
						if person.Role != "castMember" {
							value += fmt.Sprintf(" %s;", person.Name)
						}
					}
					value = strings.Trim(value, " ;")
				case "camera":
					persons := item.GetPersons()
					for _, person := range persons {
						if person.Role == "castMember" {
							value += fmt.Sprintf(" %s;", person.Name)
						}
					}
					value = strings.Trim(value, " ;")
				case "descr":
					value = item.GetAbstract()
				case "year":
					value = item.GetDate()
				case "medium":
					value = "Video"
				default:
					value = (*content)[fld]
				}
			*/
			record = append(record, value)

		}
		cw.Write(record)
	}
	return nil
}
