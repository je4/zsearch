package hypothesis

type Identity struct {
	Provider         string `json:"provider,omitempty"`
	ProviderUniqueId string `json:"provider_unique_id,omitempty"`
}

type User struct {
	hypothesis  *Hypothesis
	Authority   string     `json:"authority"`
	Username    string     `json:"username"`
	Email       string     `json:"email,omitempty"`
	DisplayName string     `json:"display_name"`
	Identities  []Identity `json:"identities,omitempty"`
	UserId      string     `json:"userid"`
}

func (user User) GetAnnotations(callback func(ann Annotation) error) error {
	return user.hypothesis.Search(map[string]string{"user": "acct:" + user.UserId}, callback)
}
