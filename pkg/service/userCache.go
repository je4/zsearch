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
	LoggedIn  bool
	LoggedOut bool
}

type UserCache struct {
	cache gcache.Cache
}

func NewGuestUser() *User {
	return &User{
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

func (uc *UserCache) SetUser(user *User) error {
	return uc.cache.Set(user.Id, user)
}
