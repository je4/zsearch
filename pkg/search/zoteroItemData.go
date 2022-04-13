/*
Copyright 2020 Center for Digital Matter HGK FHNW, Basel.
Copyright 2020 info-age GmbH, Basel.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS-IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package search

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"
)

// Relations are empty array or string map
type RelationList map[string][]string

func (rl *RelationList) UnmarshalJSON(data []byte) error {
	var i interface{}
	if err := json.Unmarshal(data, &i); err != nil {
		return err
	}
	switch d := i.(type) {
	case map[string]interface{}:
		*rl = RelationList{}
		for key, val := range d {
			switch value := val.(type) {
			case []interface{}:
				(*rl)[key] = []string{}
				for _, val2 := range value {
					switch value2 := val2.(type) {
					case string:
						(*rl)[key] = append((*rl)[key], value2)
					}
				}
			case string:
				(*rl)[key] = []string{value}
			}
		}
	case []interface{}:
		if len(d) > 0 {
			return errors.New(fmt.Sprintf("invalid object list for type RelationList - %s", string(data)))
		}
	}
	return nil
}

type ZoteroCollectionData struct {
	Key              string       `json:"Key"`
	Name             string       `json:"Name"`
	Version          int64        `json:"version"`
	Relations        RelationList `json:"relations"`
	ParentCollection Parent       `json:"parentCollection,omitempty"`
}

type ZoteroCollectionMeta struct {
	NumCollections int64 `json:"numCollections"`
	NumItems       int64 `json:"numItems"`
}

type ZoteroCollection struct {
	Key     string               `json:"Key"`
	Version int64                `json:"version"`
	Library ZoteroLibrary        `json:"library,omitempty"`
	Links   interface{}          `json:"links,omitempty"`
	Meta    ZoteroCollectionMeta `json:"meta,omitempty"`
	Data    ZoteroCollectionData `json:"data,omitempty"`
	Parent  *ZoteroCollection    `json:"parent,omitempty"`
}

func (zc *ZoteroCollection) GetParents() []string {
	if zc.Parent == nil {
		return []string{zc.Data.Name}
	}
	result := zc.Parent.GetParents()
	return append(result, zc.Data.Name)
}

type ItemTag struct {
	Tag  string `json:"Tag"`
	Type int64  `json:"type,omitempty"`
}

// zotero returns single item lists as string
type ZoteroStringList []string

func (irl *ZoteroStringList) UnmarshalJSON(data []byte) error {
	var i interface{}
	if err := json.Unmarshal(data, &i); err != nil {
		return err
	}
	switch i.(type) {
	case string:
		*irl = ZoteroStringList{i.(string)}
	case []interface{}:
		*irl = ZoteroStringList{}
		for _, i2 := range i.([]interface{}) {
			str, ok := i2.(string)
			if !ok {
				return errors.New(fmt.Sprintf("invalid type %v for %v", reflect.TypeOf(i2), i2))
			}
			*irl = append(*irl, str)
		}
	default:
		return errors.New(fmt.Sprintf("invalid type %v for %v", reflect.TypeOf(i), string(data)))
	}
	return nil
}

// zotero treats empty strings as false in ParentCollection
type Parent string

func (pc *Parent) UnmarshalJSON(data []byte) error {
	var i interface{}
	if err := json.Unmarshal(data, &i); err != nil {
		return err
	}
	switch i.(type) {
	case bool:
		*pc = ""
	case string:
		*pc = Parent(i.(string))
	default:
		return errors.New(fmt.Sprintf("invalid no string for %v", string(data)))
	}
	return nil
}

type ZoteroLibrary struct {
	Type  string      `json:"type"`
	Id    int64       `json:"id"`
	Name  string      `json:"Name"`
	Links interface{} `json:"links"`
}

type ZoteroGroupMeta struct {
	Created      time.Time `json:"created"`
	LastModified time.Time `json:"lastModified"`
	NumItems     int64     `json:"numItems"`
}

type ZoteroGroupData struct {
	Id             int64   `json:"id"`
	Version        int64   `json:"version"`
	Name           string  `json:"Name"`
	Owner          int64   `json:"owner"`
	Type           string  `json:"type"`
	Description    string  `json:"description"`
	Url            string  `json:"url"`
	HasImage       int64   `json:"hasImage"`
	LibraryEditing string  `json:"libraryEditing"`
	LibraryReading string  `json:"libraryReading"`
	FileEditing    string  `json:"fileEditing"`
	Admins         []int64 `json:"admins"`
}

type ZoteroGroup struct {
	Id      int64           `json:"id"`
	Version int64           `json:"version"`
	Links   interface{}     `json:"links,omitempty"`
	Meta    ZoteroGroupMeta `json:"meta"`
	Data    ZoteroGroupData `json:"data"`
}

type ZoteroData struct {
	Key         string             `json:"Key"`
	Version     int                `json:"version"`
	Library     ZoteroLibrary      `json:"library"`
	Links       interface{}        `json:"links"`
	Meta        interface{}        `json:"meta"`
	Data        ItemGeneric        `json:"data"`
	Group       ZoteroGroup        `json:"group"`
	Children    []ZoteroData       `json:"children,omitempty"`
	Collections []ZoteroCollection `json:"collections,omitempty"`
}
type ItemDataBase struct {
	Key          string           `json:"Key,omitempty"`
	Version      int64            `json:"version"`
	ItemType     string           `json:"itemType"`
	Tags         []ItemTag        `json:"tags"`
	Relations    RelationList     `json:"relations"`
	ParentItem   Parent           `json:"parentItem,omitempty"`
	Collections  []string         `json:"collections"`
	DateAdded    string           `json:"dateAdded,omitempty"`
	DateModified string           `json:"dateModified,omitempty"`
	Creators     []ItemDataPerson `json:"creators"`
	Media        ItemMedia        `json:"media,omitempty"`
}

type ItemDataPerson struct {
	CreatorType string `json:"creatorType"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
}

type ItemMedia struct {
	Metadata ItemMediaMeta `json:"metadata,omitempty"`
}

type ItemMediaMeta struct {
	Mimetype string      `json:"mimetype"`
	Type     string      `json:"type"`
	Ext      string      `json:"ext,omitempty"`
	Sha256   string      `json:"sha256,omitempty"`
	Duration float64     `json:"duration,omitempty"`
	Width    float64     `json:"width,omitempty"`
	Height   float64     `json:"height,omitempty"`
	Video    interface{} `json:"video,omitempty"`
	Audio    interface{} `json:"audio,omitempty"`
	Image    interface{} `json:"image,omitempty"`
}

func (imm *ItemMediaMeta) UnmarshalJSON(data []byte) error {
	type ItemMediaMeta2 ItemMediaMeta
	i := ItemMediaMeta2{}
	if err := json.Unmarshal(data, &i); err != nil {
		return nil
	}
	*imm = ItemMediaMeta(i)

	return nil
}

type ItemGeneric struct {
	ItemDataBase
	NumPages             string  `json:"numPages,omitempty"`             // # of Pages
	NumberOfVolumes      string  `json:"numberOfVolumes,omitempty"`      // # of Volumes
	AbstractNote         string  `json:"abstractNote,omitempty"`         // Abstract
	AccessDate           string  `json:"accessDate,omitempty"`           // Accessed
	ApplicationNumber    string  `json:"applicationNumber,omitempty"`    // Application Number
	Archive              string  `json:"archive,omitempty"`              // Archive
	ArtworkSize          string  `json:"artworkSize,omitempty"`          // Artwork Size
	Assignee             string  `json:"assignee,omitempty"`             // Assignee
	BillNumber           string  `json:"billNumber,omitempty"`           // Bill Number
	BlogTitle            string  `json:"blogTitle,omitempty"`            // Blog Title
	BookTitle            string  `json:"bookTitle,omitempty"`            // Book Title
	CallNumber           string  `json:"callNumber,omitempty"`           // Call Number
	CaseName             string  `json:"caseName,omitempty"`             // Case Name
	Code                 string  `json:"code,omitempty"`                 // Code
	CodeNumber           string  `json:"codeNumber,omitempty"`           // Code Number
	CodePages            string  `json:"codePages,omitempty"`            // Code Pages
	CodeVolume           string  `json:"codeVolume,omitempty"`           // Code Volume
	Committee            string  `json:"committee,omitempty"`            // Committee
	Company              string  `json:"company,omitempty"`              // Company
	ConferenceName       string  `json:"conferenceName,omitempty"`       // Conference Name
	Country              string  `json:"country,omitempty"`              // Country
	Court                string  `json:"court,omitempty"`                // Court
	DOI                  string  `json:"DOI,omitempty"`                  // DOI
	Date                 string  `json:"date,omitempty"`                 // Date
	DateDecided          string  `json:"dateDecided,omitempty"`          // Date Decided
	DateEnacted          string  `json:"dateEnacted,omitempty"`          // Date Enacted
	DictionaryTitle      string  `json:"dictionaryTitle,omitempty"`      // Dictionary Title
	Distributor          string  `json:"distributor,omitempty"`          // Distributor
	DocketNumber         string  `json:"docketNumber,omitempty"`         // Docket Number
	DocumentNumber       string  `json:"documentNumber,omitempty"`       // Document Number
	Edition              string  `json:"edition,omitempty"`              // Edition
	EncyclopediaTitle    string  `json:"encyclopediaTitle,omitempty"`    // Encyclopedia Title
	EpisodeNumber        string  `json:"episodeNumber,omitempty"`        // Episode Number
	Extra                string  `json:"extra,omitempty"`                // Extra
	AudioFileType        string  `json:"audioFileType,omitempty"`        // File Type
	FilingDate           string  `json:"filingDate,omitempty"`           // Filing Date
	FirstPage            string  `json:"firstPage,omitempty"`            // First Page
	AudioRecordingFormat string  `json:"audioRecordingFormat,omitempty"` // Format
	VideoRecordingFormat string  `json:"videoRecordingFormat,omitempty"` // Format
	ForumTitle           string  `json:"forumTitle,omitempty"`           // Forum/Listserv Title
	Genre                string  `json:"genre,omitempty"`                // Genre
	History              string  `json:"history,omitempty"`              // History
	ISBN                 string  `json:"ISBN,omitempty"`                 // ISBN
	ISSN                 string  `json:"ISSN,omitempty"`                 // ISSN
	Institution          string  `json:"institution,omitempty"`          // Institution
	Issue                string  `json:"issue,omitempty"`                // Issue
	IssueDate            string  `json:"issueDate,omitempty"`            // Issue Date
	IssuingAuthority     string  `json:"issuingAuthority,omitempty"`     // Issuing Authority
	JournalAbbreviation  string  `json:"journalAbbreviation,omitempty"`  // Journal Abbr
	Label                string  `json:"label,omitempty"`                // Label
	Language             string  `json:"language,omitempty"`             // Language
	ProgrammingLanguage  string  `json:"programmingLanguage,omitempty"`  // Language
	LegalStatus          string  `json:"legalStatus,omitempty"`          // Legal Status
	LegislativeBody      string  `json:"legislativeBody,omitempty"`      // Legislative Body
	LibraryCatalog       string  `json:"libraryCatalog,omitempty"`       // ZoteroLibrary Catalog
	ArchiveLocation      string  `json:"archiveLocation,omitempty"`      // Loc. in Archive
	InterviewMedium      string  `json:"interviewMedium,omitempty"`      // Medium
	ArtworkMedium        string  `json:"artworkMedium,omitempty"`        // Medium
	MeetingName          string  `json:"meetingName,omitempty"`          // Meeting Name
	NameOfAct            string  `json:"nameOfAct,omitempty"`            // Name of Act
	Network              string  `json:"network,omitempty"`              // Network
	Pages                float64 `json:"pages,omitempty"`                // Pages
	PatentNumber         string  `json:"patentNumber,omitempty"`         // Patent Number
	Place                string  `json:"place,omitempty"`                // Place
	PostType             string  `json:"postType,omitempty"`             // Post Type
	PriorityNumbers      string  `json:"priorityNumbers,omitempty"`      // Priority Numbers
	ProceedingsTitle     string  `json:"proceedingsTitle,omitempty"`     // Proceedings Title
	ProgramTitle         string  `json:"programTitle,omitempty"`         // Program Title
	PublicLawNumber      string  `json:"publicLawNumber,omitempty"`      // Public Law Number
	PublicationTitle     string  `json:"publicationTitle,omitempty"`     // Publication
	Publisher            string  `json:"publisher,omitempty"`            // Publisher
	References           string  `json:"references,omitempty"`           // References
	ReportNumber         string  `json:"reportNumber,omitempty"`         // Report Number
	ReportType           string  `json:"reportType,omitempty"`           // Report Type
	Reporter             string  `json:"reporter,omitempty"`             // Reporter
	ReporterVolume       string  `json:"reporterVolume,omitempty"`       // Reporter Volume
	Rights               string  `json:"rights,omitempty"`               // Rights
	RunningTime          string  `json:"runningTime,omitempty"`          // Running Time
	Scale                string  `json:"scale,omitempty"`                // Scale
	Section              string  `json:"section,omitempty"`              // Section
	Series               string  `json:"series,omitempty"`               // Series
	SeriesNumber         string  `json:"seriesNumber,omitempty"`         // Series Number
	SeriesText           string  `json:"seriesText,omitempty"`           // Series Text
	SeriesTitle          string  `json:"seriesTitle,omitempty"`          // Series Title
	Session              string  `json:"session,omitempty"`              // Session
	ShortTitle           string  `json:"shortTitle,omitempty"`           // Short Title
	Studio               string  `json:"studio,omitempty"`               // Studio
	Subject              string  `json:"subject,omitempty"`              // Subject
	System               string  `json:"system,omitempty"`               // System
	Title                string  `json:"title,omitempty"`                // Title
	ThesisType           string  `json:"thesisType,omitempty"`           // Type
	PresentationType     string  `json:"presentationType,omitempty"`     // Type
	MapType              string  `json:"mapType,omitempty"`              // Type
	ManuscriptType       string  `json:"manuscriptType,omitempty"`       // Type
	LetterType           string  `json:"letterType,omitempty"`           // Type
	Url                  string  `json:"url,omitempty"`                  // URL
	University           string  `json:"university,omitempty"`           // University
	VersionNumber        string  `json:"versionNumber,omitempty"`        // Version
	Volume               string  `json:"volume,omitempty"`               // Volume
	WebsiteTitle         string  `json:"websiteTitle,omitempty"`         // Website Title
	WebsiteType          string  `json:"websiteType,omitempty"`          // Website Type

	// Attachment
	LinkMode    string `json:"linkMode,omitempty"`
	Note        string `json:"note,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	Charset     string `json:"charset,omitempty"`
	Filename    string `json:"filename,omitempty"`
	MD5         string `json:"md5,omitempty"`
	MTime       int64  `json:"mtime,omitempty"`
}
