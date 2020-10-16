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
	"fmt"
	"github.com/bluele/gcache"
	"github.com/goph/emperror"
	"github.com/je4/zsearch/pkg/amp"
	"github.com/pkg/errors"
	"html/template"
	"net/url"
	"time"
)

type User struct {
	Server    *Server   `json:"-"`
	Id        string    `json:"Id"`
	Groups    []string  `json:"groups"`
	Email     string    `json:"email"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	HomeOrg   string    `json:"homeOrg"`
	Exp       time.Time `json:"exp"`
	LoggedIn  bool      `json:"loggedIn"`
	LoggedOut bool      `json:"loggedOut"`
}

func (u User) inGroup(grp string) bool {
	for _, g := range u.Groups {
		if g == grp {
			return true
		}
	}
	return false
}

func (u User) LinkSignatureCache(signature string) string {
	urlstr := fmt.Sprintf("%s/%s/%s", u.Server.addrExt, u.Server.detailPrefix, signature)
	var err error
	if u.Server.ampCache != nil {
		urlstr, err = u.Server.ampCache.BuildUrl(urlstr, amp.PAGE)
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
	}
	return urlstr
}

func (u User) LinkSearch(query string, facets ...string) template.URL {
	urlstr := fmt.Sprintf("%s/%s?searchtext=%s", u.Server.addrExt, u.Server.searchPrefix, url.QueryEscape(query))
	for _, f := range facets {
		urlstr += fmt.Sprintf("&%s=true", url.QueryEscape(f))
	}
	if u.LoggedIn {
		jwt, err := NewJWT(
			u.Server.jwtKey,
			"search",
			"HS256",
			int64(u.Server.linkTokenExp.Seconds()),
			"catalogue",
			"mediathek",
			u.Id)
		if err != nil {
			return template.URL(fmt.Sprintf("ERROR: %v", err))
		}
		urlstr += fmt.Sprintf("&token=%s", jwt)
	} else {
		if u.Server.ampCache != nil {
			var err error
			urlstr, err = u.Server.ampCache.BuildUrl(urlstr, amp.PAGE)
			if err != nil {
				return template.URL(fmt.Sprintf("ERROR: %v", err))
			}
		}
	}
	return template.URL(urlstr)

}
func (u User) LinkSignature(signature string) string {
	/*
		proto := "http"
		if u.Server.srv.TLSConfig.Certificates != nil {
			proto = "https"
		}
		urlstr := fmt.Sprintf("%s://%s/%s/%s", proto, u.Server.srv.Addr, u.Server.detailPrefix, signature)
	*/
	urlstr := fmt.Sprintf("%s/%s/%s", u.Server.addrExt, u.Server.detailPrefix, signature)
	if u.LoggedIn {
		jwt, err := NewJWT(
			u.Server.jwtKey,
			fmt.Sprintf("detail:%s", signature),
			"HS256",
			int64(u.Server.linkTokenExp.Seconds()),
			"catalogue",
			"mediathek",
			u.Id)
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		urlstr = fmt.Sprintf("%s?token=%s", urlstr, jwt)
	} else {
		if u.Server.ampCache != nil {
			var err error
			urlstr, err = u.Server.ampCache.BuildUrl(urlstr, amp.PAGE)
			if err != nil {
				return fmt.Sprintf("ERROR: %v", err)
			}
		}
	}
	return urlstr
}

type UserCache struct {
	cache gcache.Cache
}

func NewGuestUser(s *Server) *User {
	return &User{
		Server:    s,
		Id:        "0",
		Groups:    []string{"global/guest"},
		Email:     "",
		FirstName: "",
		LastName:  "Guest",
		HomeOrg:   "",
		Exp:       time.Now().Add(time.Hour * 24),
		LoggedIn:  false,
		LoggedOut: false,
	}
}

func NewUserCache(idleTimeout time.Duration, initialSize int) (*UserCache, error) {
	uc := &UserCache{
		cache: gcache.New(initialSize).ARC().Expiration(idleTimeout).Build(),
	}
	return uc, nil
}

func (uc *UserCache) GetUser(id string) (*User, error) {
	u, err := uc.cache.Get(id)
	if err != nil {
		return nil, emperror.Wrapf(err, "user %s not in cache", id)
	}
	user, ok := u.(*User)
	if !ok {
		return nil, errors.New(fmt.Sprintf("invalid cache entry %+v", u))
	}
	return user, nil
}

func (uc *UserCache) SetUser(user *User, index string) error {
	return uc.cache.Set(index, user)
}
