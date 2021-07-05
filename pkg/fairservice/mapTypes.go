package fairservice

import (
	"github.com/je4/FairService/v2/pkg/model/myfair"
	"github.com/je4/FairService/v2/pkg/model/zsearch"
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
