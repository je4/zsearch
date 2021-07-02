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
}

func NewFairService(address string, certSkipVerify bool) (*FairService, error) {
	fs := &FairService{
		Address:        address,
		CertSkipVerify: certSkipVerify,
	}
	return fs, nil
}

func (fs *FairService) Create(item *search.Item) (string, error) {
	var createData myfairService.CreateData

	createData.Source = item.Name()
	createData.SourceIdentifier = item.GetSignature()
	createData.Core = myfair.Core{}
	createData.Core.PublicationYear = item.GetDate()
	createData.Core.Identifier = []myfair.Identifier{myfair.Identifier{
		Value:          item.GetSignatureOriginal(),
		IdentifierType: "zotero",
	}}
	createData.Core.Title = []myfair.Title{
		myfair.Title{
			Data: item.GetTitle(),
			Type: "",
		},
	}
	createData.Core.Person = []myfair.Person{}
	for _, p := range item.GetPersons() {
		createData.Core.Person = append(createData.Core.Person, myfair.Person{
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
