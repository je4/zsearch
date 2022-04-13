package zsearchclient

import (
	"bytes"
	"crypto/sha512"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/je4/utils/v2/pkg/JWTInterceptor"
	"github.com/je4/zsearch/v2/pkg/search"
	"github.com/op/go-logging"
	"github.com/pkg/errors"
	"io/ioutil"
	"math"
	"net/http"
	"time"
)

type ZSearchClient struct {
	service        string
	baseUrl        string
	jwtKey         string
	jwtAlg         string
	certSkipVerify bool
	log            *logging.Logger
}

func NewZSearchClient(service, baseUrl, jwtKey, jwtAlg string, certSkipVerify bool, jwtTimeout time.Duration, log *logging.Logger) (*ZSearchClient, error) {
	zsc := &ZSearchClient{
		service:        service,
		baseUrl:        baseUrl,
		jwtKey:         jwtKey,
		jwtAlg:         jwtAlg,
		certSkipVerify: certSkipVerify,
		log:            log,
	}
	// create transport with authorization bearer
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: zsc.certSkipVerify}

	return zsc, nil
}
func (zsc *ZSearchClient) SignatureCreate(data *search.SourceData) error {
	tr, err := JWTInterceptor.NewJWTTransport(zsc.service, "SignatureCreate", JWTInterceptor.Secure, nil, sha512.New(), zsc.jwtKey, zsc.jwtAlg, 30*time.Second)
	if err != nil {
		return errors.Wrapf(err, "cannot create jwt transport")
	}
	client := &http.Client{Transport: tr}

	qurl := fmt.Sprintf("%s/signatures", zsc.baseUrl)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return errors.Wrap(err, "cannot marshal sourcedata")
	}
	req, err := http.NewRequest("POST", qurl, bytes.NewBuffer(jsonData))
	if err != nil {
		return errors.Wrapf(err, "cannot create post request %s", qurl)
	}
	req.Header.Add("Content-Type", "application/json")
	response, err := client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "error creating signature - POST %s", qurl)
	}
	resultData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return errors.Wrap(err, "cannot read response body")
	}
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		return errors.Errorf("invalid return status: %v", response.Status)
	}
	result := &search.ApiResult{}
	if err := json.Unmarshal(resultData, result); err != nil {
		return errors.Wrap(err, "cannot decode result")
	}
	if result.Status != "ok" {
		return errors.Errorf("error creating signature: %s", result.Message)
	}
	return nil
}

func (zsc *ZSearchClient) SignaturesClear(prefix string) (int64, error) {
	tr, err := JWTInterceptor.NewJWTTransport(zsc.service, "SignaturesDelete", JWTInterceptor.Secure, nil, sha512.New(), zsc.jwtKey, zsc.jwtAlg, 30*time.Second)
	if err != nil {
		return 0, errors.Wrapf(err, "cannot create jwt transport")
	}
	client := &http.Client{Transport: tr}

	qurl := fmt.Sprintf("%s/signatures/%s", zsc.baseUrl, prefix)
	req, err := http.NewRequest("DELETE", qurl, nil)
	if err != nil {
		return 0, errors.Wrapf(err, "cannot create delete request %s", qurl)
	}

	zsc.log.Infof("calling %s:%s", req.Method, req.URL.String())
	response, err := client.Do(req)
	if err != nil {
		return 0, errors.Wrapf(err, "cannot query DELETE:%s", qurl)
	}
	defer response.Body.Close()

	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return 0, errors.Wrap(err, "cannot read response body")
	}

	result := &search.ApiResult{}

	if err := json.Unmarshal(bodyBytes, result); err != nil {
		return 0, errors.Wrapf(err, "cannot unmarshal result %s", string(bodyBytes))
	}

	num, ok := result.Result.(float64)
	if !ok {
		return 0, errors.New(fmt.Sprintf("result does not contain number of deletions: %s", string(bodyBytes)))
	}

	return int64(math.Round(num)), nil
}

func (zsc *ZSearchClient) ClearCache() error {
	tr, err := JWTInterceptor.NewJWTTransport(zsc.service, "ClearCache", JWTInterceptor.Secure, nil, sha512.New(), zsc.jwtKey, zsc.jwtAlg, 30*time.Second)
	if err != nil {
		return errors.Wrapf(err, "cannot create jwt transport")
	}
	client := &http.Client{Transport: tr}

	qurl := fmt.Sprintf("%s/clearcache", zsc.baseUrl)
	req, err := http.NewRequest("POST", qurl, nil)
	if err != nil {
		return errors.Wrapf(err, "cannot create post request %s", qurl)
	}

	zsc.log.Infof("calling %s:%s", req.Method, req.URL.String())
	response, err := client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "cannot query POST:%s", qurl)
	}
	defer response.Body.Close()

	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return errors.Wrap(err, "cannot read response body")
	}

	result := &search.ApiResult{}

	if err := json.Unmarshal(bodyBytes, result); err != nil {
		return errors.Wrapf(err, "cannot unmarshal result %s", string(bodyBytes))
	}
	if result.Status != "ok" {
		return errors.Errorf("cannot clear cache: %v", result.Message)
	}
	return nil
}

func (zsc *ZSearchClient) LastUpdate(prefix string) (time.Time, error) {
	tr, err := JWTInterceptor.NewJWTTransport(zsc.service, "LastUpdate", JWTInterceptor.Secure, nil, sha512.New(), zsc.jwtKey, zsc.jwtAlg, 30*time.Second)
	if err != nil {
		return time.Time{}, errors.Wrapf(err, "cannot create jwt transport")
	}
	client := &http.Client{Transport: tr}

	qurl := fmt.Sprintf("%s/signatures/%s/lastupdate", zsc.baseUrl, prefix)
	req, err := http.NewRequest("GET", qurl, nil)
	if err != nil {
		return time.Time{}, errors.Wrapf(err, "cannot create delete request %s", qurl)
	}

	zsc.log.Infof("calling %s:%s", req.Method, req.URL.String())
	response, err := client.Do(req)
	if err != nil {
		return time.Time{}, errors.Wrapf(err, "cannot query DELETE:%s", qurl)
	}
	defer response.Body.Close()

	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return time.Time{}, errors.Wrap(err, "cannot read response body")
	}

	result := &search.ApiResult{}

	if err := json.Unmarshal(bodyBytes, result); err != nil {
		return time.Time{}, errors.Wrapf(err, "cannot unmarshal result %s", string(bodyBytes))
	}

	lastString, ok := result.Result.(string)
	if !ok {
		return time.Time{}, errors.New(fmt.Sprintf("result does not last update: %s", string(bodyBytes)))
	}
	last, err := time.Parse(time.RFC3339, lastString)
	if err != nil {
		return time.Time{}, errors.Wrapf(err, "cannot parse time - %s", lastString)
	}

	return last, nil
}

func (zsc *ZSearchClient) BuildSitemap() error {
	tr, err := JWTInterceptor.NewJWTTransport(zsc.service, "BuildSitemap", JWTInterceptor.Secure, nil, sha512.New(), zsc.jwtKey, zsc.jwtAlg, 30*time.Second)
	if err != nil {
		return errors.Wrapf(err, "cannot create jwt transport")
	}
	client := &http.Client{Transport: tr}

	qurl := fmt.Sprintf("%s/buildsitemap", zsc.baseUrl)
	req, err := http.NewRequest("POST", qurl, nil)
	if err != nil {
		return errors.Wrapf(err, "cannot create post request %s", qurl)
	}

	zsc.log.Infof("calling %s:%s", req.Method, req.URL.String())
	response, err := client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "cannot query POST:%s", qurl)
	}
	defer response.Body.Close()

	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return errors.Wrap(err, "cannot read response body")
	}

	result := &search.ApiResult{}

	if err := json.Unmarshal(bodyBytes, result); err != nil {
		return errors.Wrapf(err, "cannot unmarshal result %s", string(bodyBytes))
	}

	if result.Status != "ok" {
		return errors.New(fmt.Sprintf("error building sitemap: %s", result.Message))
	}

	return nil
}

func (zsc *ZSearchClient) Ping() error {
	qurl := fmt.Sprintf("%s/ping", zsc.baseUrl)
	zsc.log.Infof("calling %s:%s", "GET", qurl)
	response, err := http.Get(qurl)
	if err != nil {
		return errors.Wrapf(err, "cannot query GET:%s", qurl)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("invalid result status %v - %s", response.StatusCode, response.Status))
	}
	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return errors.Wrap(err, "cannot read response body")
	}

	result := &search.ApiResult{}

	if err := json.Unmarshal(bodyBytes, result); err != nil {
		return errors.Wrapf(err, "cannot unmarshal result %s", string(bodyBytes))
	}

	if result.Status != "ok" {
		return errors.New(fmt.Sprintf("cannot ping: %v", result.Message))
	}
	return nil
}
