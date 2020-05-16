package service

import (
	"fmt"
	"github.com/bluele/gcache"
	"github.com/goph/emperror"
	"github.com/pkg/errors"
	"time"
)

type User struct {
	Id        string
	Groups    []string
	Email     string
	FirstName string
	LastName  string
	HomeOrg   string
	Exp       time.Time
}

type UserCache struct {
	cache gcache.Cache
}

func NewUserCache() *UserCache {
	return &UserCache{cache: gcache.New(20).ARC().Build()}
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

func (uc *UserCache) SetUser(user *User) error {
	return uc.cache.Set(user.Id, user)
}
