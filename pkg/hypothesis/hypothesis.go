package hypothesis

import (
	"emperror.dev/emperror"
	"encoding/base64"
	"fmt"
	"github.com/op/go-logging"
)

const PAGESIZE = 50

type Hypothesis struct {
	log    *logging.Logger
	client *resty.Client
}

type Links map[string]string

type Permission struct {
	Read   []string `json:"read,omitempty"`
	Admin  []string `json:"admin,omitempty"`
	Update []string `json:"update,omitempty"`
	Delete []string `json:"delete,omitempty"`
}

func NewHypothesis(endpoint, apikey string, log *logging.Logger) (*Hypothesis, error) {
	hypothesis := &Hypothesis{
		client: resty.New().SetAuthToken(apikey).SetHostURL(endpoint),
		log:    log,
	}
	return hypothesis, nil
}

func (hy *Hypothesis) GetGroup(id string) (Group, error) {
	hy.log.Debugf("%s/groups/%s", hy.client.HostURL, id)
	resp, err := hy.client.R().SetResult(&Group{}).Get(fmt.Sprintf("groups/%s", id))
	if err != nil {
		return Group{hypothesis: hy}, emperror.Wrap(err, "cannot get groups")
	}
	if resp.IsError() {
		return Group{hypothesis: hy}, fmt.Errorf("error loading group %s - %v", id, resp.Error())
	}
	group, ok := resp.Result().(*Group)
	if !ok {
		return Group{hypothesis: hy}, fmt.Errorf("invalid result type - %v", resp.String())
	}
	if group == nil {
		return Group{hypothesis: hy}, nil
	}
	group.hypothesis = hy
	return *group, nil
}

func (hy *Hypothesis) GetUser(id string) (User, error) {
	hy.log.Debugf("%s/users/%s", hy.client.HostURL, base64.URLEncoding.EncodeToString([]byte("acct:"+id)))
	resp, err := hy.client.R().SetResult(&User{}).Get(fmt.Sprintf("users/acct:%s", id))
	if err != nil {
		return User{hypothesis: hy}, emperror.Wrap(err, "cannot get user")
	}
	if resp.IsError() {
		return User{hypothesis: hy}, fmt.Errorf("error loading user %s - %v", id, resp.Error())
	}
	user, ok := resp.Result().(*User)
	if !ok {
		return User{hypothesis: hy}, fmt.Errorf("invalid result type - %v", resp.String())
	}
	if user == nil {
		return User{hypothesis: hy}, nil
	}
	user.hypothesis = hy
	return *user, nil
}

func (hy *Hypothesis) GetGroups(callback func(grp Group) error) error {
	hy.log.Debugf("%s/groups", hy.client.HostURL)
	resp, err := hy.client.R().SetResult(&[]Group{}).Get("groups")
	if err != nil {
		return emperror.Wrap(err, "cannot get groups")
	}
	groups, ok := resp.Result().(*[]Group)
	if !ok {
		return fmt.Errorf("invalid result type - %v", resp.String())
	}
	if groups == nil {
		return fmt.Errorf("no result")
	}

	for _, grp := range *groups {
		grp.hypothesis = hy
		if err := callback(grp); err != nil {
			return emperror.Wrapf(err, "cannot callback für group #%v - %s", grp.Id, grp.Name)
		}
	}

	return nil
}

func (hy *Hypothesis) Search(params map[string]string, callback func(ann Annotation) error) error {
	var offset int64 = 0
	paramstring := ""
	for key, val := range params {
		if len(paramstring) > 0 {
			paramstring += "&"
		}
		paramstring += fmt.Sprintf("%s=%s", key, base64.URLEncoding.EncodeToString([]byte(val)))
	}
	hy.log.Debugf(fmt.Sprintf("%s/search?%s", hy.client.HostURL, paramstring))
	for {
		params["limit"] = fmt.Sprintf("%v", PAGESIZE)
		params["offset"] = fmt.Sprintf("%v", offset)
		resp, err := hy.client.R().
			SetQueryParams(params).
			SetResult(&AnnotationList{}).
			Get("search")
		if err != nil {
			return emperror.Wrapf(err, "cannot get annotations of %s/search?%s", hy.client.HostURL, paramstring)
		}
		if resp.IsError() {
			return fmt.Errorf("cannot get group annotations of %s/search?%s", hy.client.HostURL, paramstring)
		}
		result, ok := resp.Result().(*AnnotationList)
		if !ok {
			return emperror.Wrapf(err, "invalid result format - %s", resp.String())
		}
		for _, ann := range (*result).Rows {
			ann.hypothesis = hy
			if err := callback(ann); err != nil {
				return emperror.Wrapf(err, "error in callback for annotation %s", ann.Id)
			}
		}
		offset += PAGESIZE
		if offset > result.Total {
			break
		}
	}
	return nil
}
