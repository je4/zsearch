package fairservice

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/je4/FairService/v2/pkg/model/myfair"
	myfairService "github.com/je4/FairService/v2/pkg/service"
	"github.com/je4/zsearch/v2/pkg/mediaserver"
	"github.com/je4/zsearch/v2/pkg/search"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
)

type FairService struct {
	Address        string
	CertSkipVerify bool
	JwtKey         string
}

func NewFairService(address string, certSkipVerify bool, jwtKey string) (*FairService, error) {
	fs := &FairService{
		Address:        address,
		CertSkipVerify: certSkipVerify,
		JwtKey:         jwtKey,
	}
	return fs, nil
}

func (fs *FairService) Create(item *search.Item, ms mediaserver.Mediaserver) (string, error) {
	createData := myfairService.CreateData{
		Source:    item.GetSource(),
		Signature: item.GetSignature(),
		Metadata: myfair.Core{
			Identifier: []myfair.Identifier{{
				Value:          item.GetSignatureOriginal(),
				IdentifierType: "zotero",
			}},
			Person: []myfair.Person{},
			Title: []myfair.Title{
				{
					Data: item.GetTitle(),
					Type: "",
				},
			},
			Publisher:       item.GetPublisher(),
			PublicationYear: item.GetDate(),
			ResourceType:    mapZsearch2Myfair(item.GetContentType()),
			Media:           []*myfair.Media{},
		},
		Set:     item.GetCatalogs(),
		Catalog: item.GetCatalogs(),
		Access:  myfairService.DataAccessClosed,
	}
	medias := item.GetMedia(ms)
	poster := item.GetPoster(ms)
	if poster != nil {
		createData.Metadata.Poster = &myfair.Media{
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
			createData.Metadata.Media = append(createData.Metadata.Media, &myfair.Media{
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

	acls := item.GetACL()
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
		createData.Access = myfairService.DataAccessPublic
	}
	if metaOK && !contentOK {
		createData.Access = myfairService.DataAccessClosedData
	}
	if !metaOK {
		createData.Access = myfairService.DataAccessClosed
	}

	for _, p := range item.GetPersons() {
		createData.Metadata.Person = append(createData.Metadata.Person, myfair.Person{
			PersonType:     myfair.PersonTypeAuthor,
			PersonName:     myfair.Name{Value: p.Name},
			GivenName:      "",
			FamilyName:     "",
			Affiliation:    "",
			NameIdentifier: myfair.NameIdentifier{},
		})
	}

	data, err := json.Marshal(createData)
	if err != nil {
		return "", errors.Wrapf(err, "cannot marshal [%v]", createData)
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: fs.CertSkipVerify},
	}
	client := &http.Client{Transport: tr}
	response, err := client.Post(fs.Address, "text/json", bytes.NewBuffer(data))
	if err != nil {
		return "", errors.Wrapf(err, "cannot post to %s", fs.Address)
	}
	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", errors.Wrap(err, "cannot read response body")
	}
	result := myfairService.CreateResultStatus{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", errors.Wrapf(err, "cannot decode result %s", string(bodyBytes))
	}
	if result.Status != "ok" {
		return "", errors.New(fmt.Sprintf("error creating item: %s", result.Message))
	}
	return result.UUID, nil
}
