package search

import (
	"fmt"
	isoduration "github.com/channelmeter/iso8601duration"
	"github.com/vanng822/go-solr/solr"
	"strings"
	"time"
)

type SourceStatistic struct {
	MediaMimeType map[string][]string `json:"mediaMimeType"`
	MediaWidth    map[string][]int64  `json:"mediaWidth"`
	MediaHeight   map[string][]int64  `json:"mediaHeight"`
	MediaDuration map[string][]int64  `json:"mediaDuration"`
	MediaUri      map[string][]string `json:"mediaUri"`
	MediaType     []string            `json:"mediaType"`
	MediaCount    map[string]int64    `json:"mediaCount"`
	Person        []string            `json:"person"`
	Role          []string            `json:"role"`
}

type SourceData struct {
	Signature         string               `json:"signature"`
	SignatureOriginal string               `json:"signatureoriginal"`
	Source            string               `json:"source"`
	Title             string               `json:"title"`
	Series            string               `json:"series"`
	Place             string               `json:"place"`
	Date              string               `json:"date"`
	CollectionTitle   string               `json:"collectiontitle"`
	Persons           []Person             `json:"persons"`
	ACL               map[string][]string  `json:"acl"`
	Catalog           []string             `json:"catalog"`
	Category          []string             `json:"category"`
	Tags              []string             `json:"tags"`
	Media             map[string]MediaList `json:"media"`
	Poster            *Media               `json:"poster"`
	Notes             []Note               `json:"notes"`
	Url               string               `json:"url"`
	Abstract          string               `json:"abstract"`
	References        []Reference          `json:"references"`
	Meta              *Metalist            `json:"meta,omitempty"`
	Extra             *Metalist            `json:"extra,omitempty"`
	Vars              *Varlist             `json:"vars,omitempty"`
	Type              string               `json:"type"`
	Queries           []Query              `json:"queries,omitempty"`
	ContentStr        string               `json:"-"`
	ContentMime       string               `json:"-"`
	HasMedia          bool                 `json:"hasmedia"`
	Mediatype         []string             `json:"mediatype"`
	DateAdded         time.Time            `json:"dateadded"`
	Timestamp         time.Time            `json:"timestamp"`
	Publlisher        string               `json:"publisher"`
	Rights            string               `json:"rights"`
	License           string               `json:"license"`
	Statistics        *SourceStatistic     `json:"statistics,omitempty"`
}

func NewSourceData(src Source) (*SourceData, error) {
	sd := &SourceData{
		Signature:         src.GetSignature(),
		SignatureOriginal: src.GetSignatureOriginal(),
		Rights:            src.GetRights(),
		License:           src.GetLicense(),
		Source:            src.GetSource(),
		Title:             src.GetTitle(),
		Series:            src.GetSeries(),
		Place:             src.GetPlace(),
		Date:              src.GetDate(),
		CollectionTitle:   src.GetCollectionTitle(),
		Persons:           src.GetPersons(),
		ACL:               src.GetACL(),
		Catalog:           src.GetCatalogs(),
		Category:          src.GetCategories(),
		Tags:              src.GetTags(),
		Media:             src.GetMedia(),
		Poster:            src.GetPoster(),
		Notes:             src.GetNotes(),
		Url:               src.GetUrl(),
		Abstract:          src.GetAbstract(),
		References:        src.GetReferences(),
		Meta:              src.GetMeta(),
		Extra:             src.GetExtra(),
		Vars:              src.GetVars(),
		Type:              src.GetContentType(),
		Queries:           src.GetQueries(),
		ContentStr:        src.GetContentString(),
		ContentMime:       src.GetContentMime(),
		DateAdded:         src.GetDateAdded(),
		Publlisher:        src.GetPublisher(),
		Mediatype:         []string{},
		Timestamp:         time.Now(),
	}
	sd.HasMedia = len(sd.Media) > 0
	for mt, _ := range sd.Media {
		sd.Mediatype = append(sd.Mediatype, mt)
	}
	return sd, nil
}

func (sd *SourceData) SetStatistics() {
	stats := &SourceStatistic{
		MediaType:     []string{},
		MediaMimeType: map[string][]string{},
		MediaWidth:    map[string][]int64{},
		MediaHeight:   map[string][]int64{},
		MediaDuration: map[string][]int64{},
		MediaCount:    map[string]int64{},
		MediaUri:      map[string][]string{},
		Person:        []string{},
		Role:          []string{},
	}
	for _, person := range sd.Persons {
		stats.Person = append(stats.Person, person.Name)
		stats.Role = append(stats.Role, person.Role)
	}
	for mType, medias := range sd.Media {
		if _, ok := stats.MediaDuration[mType]; !ok {
			stats.MediaDuration[mType] = []int64{}
		}
		if _, ok := stats.MediaWidth[mType]; !ok {
			stats.MediaWidth[mType] = []int64{}
		}
		if _, ok := stats.MediaHeight[mType]; !ok {
			stats.MediaHeight[mType] = []int64{}
		}
		if _, ok := stats.MediaMimeType[mType]; !ok {
			stats.MediaMimeType[mType] = []string{}
		}
		if _, ok := stats.MediaUri[mType]; !ok {
			stats.MediaUri[mType] = []string{}
		}
		stats.MediaCount[mType] = int64(len(medias))
		for _, media := range medias {
			stats.MediaType = append(stats.MediaType, media.Type)
			stats.MediaDuration[mType] = append(stats.MediaDuration[mType], media.Duration)
			stats.MediaWidth[mType] = append(stats.MediaWidth[mType], media.Width)
			stats.MediaHeight[mType] = append(stats.MediaHeight[mType], media.Height)
			stats.MediaUri[mType] = append(stats.MediaUri[mType], media.Uri)
			stats.MediaMimeType[mType] = append(stats.MediaMimeType[mType], media.Mimetype)
		}
	}
	sd.Statistics = stats
}

func (sd *SourceData) AddIdentifiers(identifiers map[string]string) {
	for name, ident := range identifiers {
		(*sd.Extra)[name] = ident
	}
}
func (sd *SourceData) GetSource() string {
	return sd.Source
}

func (sd *SourceData) GetSignatureOriginal() string {
	return sd.SignatureOriginal
}

func (sd *SourceData) GetRights() string {
	return sd.Rights
}

func (sd *SourceData) GetLicense() string {
	return sd.License
}

func (sd *SourceData) GetSignature() string {
	return sd.Signature
}

func (sd *SourceData) Name() string {
	return sd.Name()
}

func (sd *SourceData) GetTitle() string {
	return sd.Title
}

func (sd *SourceData) GetSeries() string {
	return sd.Series
}

func (sd *SourceData) GetPlace() string {
	return sd.Place
}

func (sd *SourceData) GetDate() string {
	return sd.Date
}

func (sd *SourceData) GetCollectionTitle() string {
	return sd.CollectionTitle
}

func (sd *SourceData) GetPersons() []Person {
	return sd.Persons
}

func (sd *SourceData) GetACL() map[string][]string {
	return sd.ACL
}

func (sd *SourceData) GetCatalogs() []string {
	return sd.Catalog
}

func (sd *SourceData) GetCategories() []string {
	return sd.Category
}

func (sd *SourceData) GetTags() []string {
	return sd.Tags
}

func (sd *SourceData) GetMedia() map[string]MediaList {
	return sd.Media
}

func (sd *SourceData) GetPoster() *Media {
	return sd.Poster
}

func (sd *SourceData) GetNotes() []Note {
	return sd.Notes
}

func (sd *SourceData) GetUrl() string {
	return sd.Url
}

func (sd *SourceData) GetAbstract() string {
	return sd.Abstract
}

func (sd *SourceData) GetReferences() []Reference {
	return sd.References
}

func (sd *SourceData) GetMeta() *Metalist {
	return sd.Meta
}

func (sd *SourceData) GetExtra() *Metalist {
	return sd.Extra
}

func (sd *SourceData) GetVars() *Varlist {
	return sd.Vars
}

func (sd *SourceData) GetContentType() string {
	return sd.Type
}

func (sd *SourceData) GetQueries() []Query {
	return sd.Queries
}

func (sd *SourceData) GetSolrDoc() *solr.Document {
	return nil
}

func (sd *SourceData) GetContentString() string {
	return sd.ContentStr
}

func (sd *SourceData) GetContentMime() string {
	return sd.ContentMime
}

func (sd *SourceData) GetDateAdded() time.Time {
	return sd.DateAdded
}

func (sd *SourceData) GetPublisher() string {
	return sd.Publlisher
}

func (sd *SourceData) GetJsonLD(self string, mediaserver func(uri string, params ...string) (string, error)) (result interface{}) {
	videos, videook := sd.Media["video"]
	audios, audiook := sd.Media["audio"]
	vData := make(JSONData)

	var description string
	if videook || audiook {
	}

	if videook {
		if len(videos) > 0 {
			video := videos[0]
			vData.set("@type", "VideoObject")
			vData.set("@context", "https://schema.org")

			if sd.Poster != nil {
				if imgUrl, err := mediaserver(sd.Poster.Uri, "resize", fmt.Sprintf("size%vx%v", sd.Poster.Width, sd.Poster.Height), "formatJPEG"); err == nil {
					vData.add("thumbnailUrl", imgUrl)
					thumb := make(JSONData)
					thumb.set("@type", "ImageObject")
					thumb.set("url", imgUrl)
					thumb.set("width", fmt.Sprintf("%v", sd.Poster.Width))
					thumb.set("height", fmt.Sprintf("%v", sd.Poster.Height))
					vData.add("thumbnail", thumb)
				}
				if imgUrl, err := mediaserver(sd.Poster.Uri, "resize", fmt.Sprintf("size%vx%v", 640, 480), "crop", "formatJPEG"); err == nil {
					vData.add("thumbnailUrl", imgUrl)
					thumb := make(JSONData)
					thumb.set("@type", "ImageObject")
					thumb.set("url", imgUrl)
					thumb.set("width", "640")
					thumb.set("height", "480")
					vData.add("thumbnail", thumb)
				}
				if imgUrl, err := mediaserver(sd.Poster.Uri, "resize", fmt.Sprintf("size%vx%v", 480, 480), "crop", "formatJPEG"); err == nil {
					vData.add("thumbnailUrl", imgUrl)
					thumb := make(JSONData)
					thumb.set("@type", "ImageObject")
					thumb.set("url", imgUrl)
					thumb.set("width", "480")
					thumb.set("height", "480")
					vData.add("thumbnail", thumb)
				}
			}
			// duration / width / height
			var isoDuration = isoduration.Duration{}
			isoDuration.Hours = int(video.Duration / 3600)
			isoDuration.Minutes = (int(video.Duration) % 3600) / 60
			isoDuration.Seconds = int(video.Duration) % 60
			vData.set("duration", fmt.Sprintf("%v", isoDuration.String()))
			vData.set("width", fmt.Sprintf("%v", video.Width))
			vData.set("height", fmt.Sprintf("%v", video.Height))
			if coll, sig, err := mediaserverUri2ColSig(video.Uri); err == nil {
				vData.set("embedurl", fmt.Sprintf("%s/embed/%s/%s", self, coll, sig))
			}
		}
	} else {
		if audiook {
			if len(audios) > 0 {
				audio := audios[0]
				vData.set("@type", "AudioObject")
				vData.set("@context", "https://schema.org")

				// duration / width / height
				var isoDuration = isoduration.Duration{}
				isoDuration.Hours = int(audio.Duration / 3600)
				isoDuration.Minutes = (int(audio.Duration) % 3600) / 60
				isoDuration.Seconds = int(audio.Duration) % 60
				vData.set("duration", fmt.Sprintf("%v", isoDuration.String()))
				/*
					if coll, sig, err := mediaserverUri2ColSig(audio.Uri); err == nil {
						vData.set("embedurl", fmt.Sprintf("%s/embed/%s/%s", self, coll, sig))
					}
				*/
			}
		}
	}
	if videook || audiook {
		vData.set("url", self)
		vData.set("contenturl", self)
		vData.set("name", sd.Title)
		vData.set("uploadDate", sd.DateAdded.Format("2006-01-02T15:04:05Z"))

		description = sd.Abstract
		// director / actor / ...
		for _, p := range sd.Persons {
			vData.add("author", p.Name)
			description += fmt.Sprintf("  %s: %s", strings.Title(p.Role), p.Name)
		}
		pd := ""
		if sd.Place != "" {
			pd = sd.Place
		}
		if sd.Date != "" {
			if pd != "" {
				pd += ", "
			}
			pd += sd.Date
		}
		if pd != "" {
			if description != "" {
				description += "\n\n"
			}
			description += pd
		}
		vData.set("description", strings.Trim(description, " \n"))

		if sd.Poster != nil {
			if imgUrl, err := mediaserver(sd.Poster.Uri, "resize", fmt.Sprintf("size%vx%v", sd.Poster.Width, sd.Poster.Height), "formatJPEG"); err == nil {
				vData.add("thumbnailUrl", imgUrl)
				thumb := make(JSONData)
				thumb.set("@type", "ImageObject")
				thumb.set("url", imgUrl)
				thumb.set("width", fmt.Sprintf("%v", sd.Poster.Width))
				thumb.set("height", fmt.Sprintf("%v", sd.Poster.Height))
				vData.add("thumbnail", thumb)
			}
			if imgUrl, err := mediaserver(sd.Poster.Uri, "resize", fmt.Sprintf("size%vx%v", 640, 480), "crop", "formatJPEG"); err == nil {
				vData.add("thumbnailUrl", imgUrl)
				thumb := make(JSONData)
				thumb.set("@type", "ImageObject")
				thumb.set("url", imgUrl)
				thumb.set("width", "640")
				thumb.set("height", "480")
				vData.add("thumbnail", thumb)
			}
			if imgUrl, err := mediaserver(sd.Poster.Uri, "resize", fmt.Sprintf("size%vx%v", 480, 480), "crop", "formatJPEG"); err == nil {
				vData.add("thumbnailUrl", imgUrl)
				thumb := make(JSONData)
				thumb.set("@type", "ImageObject")
				thumb.set("url", imgUrl)
				thumb.set("width", "480")
				thumb.set("height", "480")
				vData.add("thumbnail", thumb)
			}
		}
		return vData
	}
	return nil
}

func (sd *SourceData) GetOpenGraph(app_id, self string, mediaserver func(uri string, params ...string) (string, error)) (namespace string, ogstr string) {
	var ogdata = make(OGData)

	namespace = "https://ogp.me/ns#"

	//ogdata.set("fb:app_id", app_id)
	ogdata.set("og:title", TrimLength(sd.Title, 60, "..."))
	ogdata.set("og:type", "website")
	ogdata.set("og:url", self)
	switch sd.Type {
	}
	if videos, ok := sd.Media["video"]; ok {
		if len(videos) > 0 {
			//namespace = "https://ogp.me/ns/video#"
			video := videos[0]
			// type
			ogdata.set("og:type", "video.other")

			// director / actor / ...
			for _, p := range sd.Persons {
				switch p.Role {
				case "director":
					ogdata.add("og:video:director", p.Name)
				case "artist":
					ogdata.add("og:video:director", p.Name)
				default:
					ogdata.add(fmt.Sprintf("og:video:actor:%s", p.Role), p.Name)
				}
			}
			// duration / width / height
			ogdata.set("og:video:duration", fmt.Sprintf("%v", video.Duration))
			ogdata.set("og:video:width", fmt.Sprintf("%v", Min(1200, video.Width)))
			ogdata.set("og:video:height", fmt.Sprintf("%v", Min(630, video.Height)))

			// release
			ogdata.set("og:video:release_data", sd.Date)

			// url
			ogdata.set("og:video:url", self)
			ogdata.set("og:video:secure_url", self)
		}
	}
	ogdata.set("og:description", TrimLength(sd.Abstract, 160, "..."))
	if sd.Poster != nil {
		if imgUrl, err := mediaserver(sd.Poster.Uri, "resize", fmt.Sprintf("size%vx%v/autorotate", Min(1200, sd.Poster.Width), Min(630, sd.Poster.Height)), "formatJPEG"); err == nil {
			ogdata.set("og:image", imgUrl)
			ogdata.set("og:image:url", imgUrl)
			ogdata.set("og:image:secure_url", imgUrl)
			ogdata.set("og:image:width", fmt.Sprintf("%v", Min(1200, sd.Poster.Width)))
			ogdata.set("og:image:height", fmt.Sprintf("%v", Min(630, sd.Poster.Height)))
			ogdata.set("og:image:type", "image/jpeg")
		}
	}
	for key, vals := range ogdata {
		ogstr += fmt.Sprintf(`   <meta property="%s" content="%s">`, key, strings.Join(vals, "; ")) + "\n"
	}
	return namespace, ogstr
}
