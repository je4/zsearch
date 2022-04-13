package fairservice

import (
	"github.com/je4/FairService/v2/pkg/fair"
	"github.com/je4/FairService/v2/pkg/model/myfair"
	"github.com/je4/FairService/v2/pkg/model/zsearch"
	"github.com/je4/zsearch/v2/pkg/search"
)

var zsearch2myfair = map[zsearch.ItemType]myfair.ResourceType{
	zsearch.ItemTypeBook:                myfair.ResourceTypeBook,
	zsearch.ItemTypeBookSection:         myfair.ResourceTypeBookSection,
	zsearch.ItemTypeThesis:              myfair.ResourceTypeThesis,
	zsearch.ItemTypeJournalArticle:      myfair.ResourceTypeJournalArticle,
	zsearch.ItemTypeMagazineArticle:     myfair.ResourceTypeMagazineArticle,
	zsearch.ItemTypeReport:              myfair.ResourceTypeReport,
	zsearch.ItemTypeWebpage:             myfair.ResourceTypeWebpage,
	zsearch.ItemTypeConferencePaper:     myfair.ResourceTypeConferencePaper,
	zsearch.ItemTypePatent:              myfair.ResourceTypePatent,
	zsearch.ItemTypeNote:                myfair.ResourceTypeNote,
	zsearch.ItemTypePerformance:         myfair.ResourceTypeArtisticPerformance,
	zsearch.ItemTypePresentation:        myfair.ResourceTypePresentation,
	zsearch.ItemTypeComputerProgram:     myfair.ResourceTypeComputerProgram,
	zsearch.ItemTypeOther:               myfair.ResourceTypeOther,
	zsearch.ItemTypeArtwork:             myfair.ResourceTypeArtwork,
	zsearch.ItemTypeAttachment:          myfair.ResourceTypeAttachment,
	zsearch.ItemTypeAudioRecording:      myfair.ResourceTypeAudioRecording,
	zsearch.ItemTypeBill:                myfair.ResourceTypeOther,
	zsearch.ItemTypeBlogPost:            myfair.ResourceTypeOther,
	zsearch.ItemTypeCase:                myfair.ResourceTypeOther,
	zsearch.ItemTypeDictionaryEntry:     myfair.ResourceTypeOther,
	zsearch.ItemTypeDocument:            myfair.ResourceTypeDocument,
	zsearch.ItemTypeEmail:               myfair.ResourceTypeEmail,
	zsearch.ItemTypeEncyclopediaArticle: myfair.ResourceTypeEncyclopediaArticle,
	zsearch.ItemTypeFilm:                myfair.ResourceTypeFilm,
	zsearch.ItemTypeForumPost:           myfair.ResourceTypeOther,
	zsearch.ItemTypeHearing:             myfair.ResourceTypeOther,
	zsearch.ItemTypeInstantMessage:      myfair.ResourceTypeInstantMessage,
	zsearch.ItemTypeInterview:           myfair.ResourceTypeInterview,
	zsearch.ItemTypeLetter:              myfair.ResourceTypeLetter,
	zsearch.ItemTypeManuscript:          myfair.ResourceTypeManuscript,
	zsearch.ItemTypeMap:                 myfair.ResourceTypeMap,
	zsearch.ItemTypeNewspaperArticle:    myfair.ResourceTypeNewspaperArticle,
	zsearch.ItemTypePodcast:             myfair.ResourceTypePodcast,
	zsearch.ItemTypeRadioBroadcast:      myfair.ResourceTypeRadioBroadcast,
	zsearch.ItemTypeStatute:             myfair.ResourceTypeOther,
	zsearch.ItemTypeTvBroadcast:         myfair.ResourceTypeTvBroadcast,
	zsearch.ItemTypeVideoRecording:      myfair.ResourceTypeVideoRecording,
}

func mapZsearch2Myfair(str string) myfair.ResourceType {
	zsType, ok := zsearch.ItemTypeReverse[str]
	if !ok {
		return myfair.ResourceTypeOther
	}
	mfType, ok := zsearch2myfair[zsType]
	if !ok {
		return myfair.ResourceTypeOther
	}
	return mfType
}

func SourceToFairItem(src search.Source) *fair.ItemData {
	item := &fair.ItemData{
		Source:    src.GetSource(),
		Signature: src.GetSignature(),
		Metadata: myfair.Core{
			Identifier: []myfair.Identifier{{
				Value:          src.GetSignatureOriginal(),
				IdentifierType: "zotero",
			}},
			Person: []myfair.Person{},
			Title: []myfair.Title{
				{
					Data: src.GetTitle(),
					Type: "",
				},
			},
			Publisher:       src.GetPublisher(),
			PublicationYear: src.GetDate(),
			ResourceType:    mapZsearch2Myfair(src.GetContentType()),
			Media:           []*myfair.Media{},
			Rights:          src.GetRights(),
			License:         src.GetLicense(),
		},
		Set:     src.GetCatalogs(),
		Catalog: src.GetCatalogs(),
		Access:  fair.DataAccessClosed,
	}
	medias := src.GetMedia()
	poster := src.GetPoster()
	if poster != nil {
		item.Metadata.Poster = &myfair.Media{
			Name:        poster.Name,
			Mimetype:    poster.Mimetype,
			Type:        poster.Type,
			Uri:         poster.Uri,
			Width:       poster.Width,
			Height:      poster.Height,
			Orientation: poster.Orientation,
			Duration:    poster.Duration,
			Fulltext:    poster.Fulltext,
		}
	}
	for _, medias := range medias {
		for _, media := range medias {
			item.Metadata.Media = append(item.Metadata.Media, &myfair.Media{
				Name:        media.Name,
				Mimetype:    media.Mimetype,
				Type:        media.Type,
				Uri:         media.Uri,
				Width:       media.Width,
				Height:      media.Height,
				Orientation: media.Orientation,
				Duration:    media.Duration,
				Fulltext:    media.Fulltext,
			})
		}
	}

	acls := src.GetACL()
	var contentOK, metaOK bool
	for name, acl := range acls {
		ok := false
		for _, grp := range acl {
			if grp == "global/guest" {
				ok = true
			}
			switch name {
			case "meta":
				metaOK = ok
			case "content":
				contentOK = ok
			}
		}
	}
	if contentOK && metaOK {
		item.Access = fair.DataAccessPublic
	}
	if metaOK && !contentOK {
		item.Access = fair.DataAccessClosedData
	}
	if !metaOK {
		item.Access = fair.DataAccessClosed
	}

	for _, p := range src.GetPersons() {
		item.Metadata.Person = append(item.Metadata.Person, myfair.Person{
			PersonType:     myfair.PersonTypeAuthor,
			PersonName:     myfair.Name{Value: p.Name},
			GivenName:      "",
			FamilyName:     "",
			Affiliation:    "",
			NameIdentifier: myfair.NameIdentifier{},
		})
	}
	return item
}
