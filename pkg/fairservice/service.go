package fairservice

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/je4/FairService/v2/pkg/model/myfair"
	myfairService "github.com/je4/FairService/v2/pkg/service"
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

func (fs *FairService) Create(item *search.Item) (string, error) {
	createData := myfairService.CreateData{
		Source:    item.Name(),
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
		},
		Set:     item.GetCatalogs(),
		Catalog: item.GetCatalogs(),
		Public:  false,
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
