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
package amp

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"golang.org/x/net/idna"
	"net/http"
	"net/url"
	"strings"
	"time"
)

/*
{
caches: [
{
id: "google",
name: "Google AMP Cache",
docs: "https://developers.google.com/amp/cache/",
cacheDomain: "cdn.ampproject.org",
updateCacheApiDomainSuffix: "cdn.ampproject.org",
thirdPartyFrameDomainSuffix: "ampproject.net"
},
{
id: "bing",
name: "Bing AMP Cache",
docs: "https://www.bing.com/webmaster/help/bing-amp-cache-bc1c884c",
cacheDomain: "www.bing-amp.com",
updateCacheApiDomainSuffix: "www.bing-amp.com",
thirdPartyFrameDomainSuffix: "www.bing-amp.net"
}
]
}
*/

type URLType rune

const (
	PAGE   URLType = 'c'
	IMAGE  URLType = 'i'
	UPDATE URLType = 'u'
)

type Cache struct {
	Id                          string                        `json:"id"`
	Name                        string                        `json:"name"`
	Docs                        string                        `json:"docs"`
	CacheDomain                 string                        `json:"cacheDomain"`
	UpdateCacheApiDomainSuffix  string                        `json:"updateCacheApiDomainSuffix"`
	ThirdPartyFrameDomainSuffix string                        `json:"thirdPartyFrameDomainSuffix"`
	baseUrl                     map[URLType]map[string]string `json:"-"`
}

func (c *Cache) Init() {
	c.baseUrl = make(map[URLType]map[string]string)
	c.baseUrl[PAGE] = make(map[string]string)
	c.baseUrl[IMAGE] = make(map[string]string)
	c.baseUrl[UPDATE] = make(map[string]string)
	c.CacheDomain = strings.ReplaceAll(c.CacheDomain, "www.", "")
	c.UpdateCacheApiDomainSuffix = strings.ReplaceAll(c.UpdateCacheApiDomainSuffix, "www.", "")
}

// 1) Punycode Decode the publisher domain. See RFC 3492
// 2) Replace any "-" (hyphen) character in the output of step 1 with "--" (two hyphens).
// 3) Replace any "." (dot) character in the output of step 2 with "-" (hyphen).
// 4) If the output of step 3 has a "-" (hyphen) at both positions 3 and 4, then to the output of step 3, add a prefix of "0-" and add a suffix of "-0". See #26205 for background.
// 5) Punycode Encode the output of step 3. See RFC 3492
func encHost(host string) (string, error) {
	// Punycode Decode the publisher domain. See RFC 3492
	domain, err := idna.ToUnicode(host)
	if err != nil {
		return "", errors.Wrapf(err, "cannot decode domain %s", host)
	}
	// Replace any "-" (hyphen) character in the output of step 1 with "--" (two hyphens).
	domain = strings.ReplaceAll(domain, "-", "--")

	// Replace any "." (dot) character in the output of step 2 with "-" (hyphen).
	domain = strings.ReplaceAll(domain, ".", "-")

	// if the output has a "-" (hyphen) at both positions 3 and 4,
	// then to the output of step 3, add a prefix of "0-" and add a suffix of "-0"
	if []rune(domain)[2] == '-' && []rune(domain)[3] == '-' {
		domain = fmt.Sprintf("0-%s-0", domain)
	}
	return domain, nil
}

func (c *Cache) BuildRefreshRSA(host string) (string, error) {
	//https://example-com.<cache.updateCacheApiDomainSuffix>/r/s/example.com/.well-known/amphtml/apikey.pub
	domain, err := encHost(host)
	if err != nil {
		return "", errors.Wrapf(err, "cannot  encode host %s", host)
	}
	return fmt.Sprintf("https://%s.%s/r/s/%s/.well-known/amphtml/apikey.pub", domain, c.UpdateCacheApiDomainSuffix, host), nil
}

func (c *Cache) BuildUpdateUrl(u string, ampApiKey *rsa.PrivateKey) (string, error) {
	url, err := url.Parse(u)
	if err != nil {
		return "", errors.Wrapf(err, "cannot parse external address %s", u)
	}
	baseurl, ok := c.baseUrl[UPDATE][url.Host]
	if !ok {
		domain, err := encHost(url.Host)
		if err != nil {
			return "", errors.Wrapf(err, "cannot  encode host %s", url.Host)
		}
		baseurl = fmt.Sprintf("https://%s.%s", domain, c.UpdateCacheApiDomainSuffix)
		c.baseUrl[UPDATE][url.Host] = baseurl
	}
	// build the url
	updateUrl := fmt.Sprintf("/update-cache/c/s/%s?amp_action=flush&amp_ts=%d",
		strings.TrimRight(url.Host+"/"+strings.TrimLeft(url.Path, "/"), "/"),
		time.Now().Unix(),
	)

	// create sha256 for url
	msgHash := sha256.New()
	_, err = msgHash.Write([]byte(updateUrl))
	if err != nil {
		return "", errors.Wrapf(err, "cannot hash path %s", updateUrl)
	}
	msgHashSum := msgHash.Sum(nil)

	// sign the hash with rsa
	sign, err := rsa.SignPSS(rand.Reader, ampApiKey, crypto.SHA256, msgHashSum, nil)
	if err != nil {
		return "", errors.Wrapf(err, "cannog sign path %s", updateUrl)
	}

	// do base64 encoding of signature
	sEnc := base64.URLEncoding.EncodeToString(sign)

	return fmt.Sprintf("%s/%s&amp_url_signature=%s", baseurl, strings.TrimLeft(updateUrl, "/"), sEnc), nil
}

func (c *Cache) BuildUrl(u string, urltype URLType) (string, error) {
	if urltype != PAGE && urltype != IMAGE {
		return "", fmt.Errorf("invalid url type %v", urltype)
	}
	url, err := url.Parse(u)
	if err != nil {
		return "", errors.Wrapf(err, "cannot parse external address %s", u)
	}
	baseurl, ok := c.baseUrl[urltype][url.Host]
	if !ok {
		domain, err := encHost(url.Host)
		if err != nil {
			return "", errors.Wrapf(err, "cannot  encode host %s", url.Host)
		}
		baseurl = fmt.Sprintf("https://%s.%s/%s/s/%s", domain, c.CacheDomain, string(urltype), url.Host)
		c.baseUrl[urltype][url.Host] = baseurl
	}
	return strings.TrimRight(fmt.Sprintf("%s/%s", baseurl, strings.TrimLeft(url.Path, "/")), "/"), nil
}

func GetCaches() (map[string]*Cache, error) {
	resp, err := http.Get("https://cdn.ampproject.org/caches.json")
	if err != nil {
		return nil, errors.Wrapf(err, "cannot load json from https://cdn.ampproject.org/caches.json")
	}
	var cachesjson = make(map[string][]Cache)

	if err := json.NewDecoder(resp.Body).Decode(&cachesjson); err != nil {
		return nil, errors.Wrapf(err, "cannot decode json")
	}
	var result = make(map[string]*Cache)
	caches, ok := cachesjson["caches"]
	if !ok {
		return nil, errors.New("no caches in result")
	}

	for _, c := range caches {
		c.Init()
		result[c.Id] = &c
	}
	return result, nil
}
