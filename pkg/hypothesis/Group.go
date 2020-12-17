package hypothesis

type Group struct {
	hypothesis   *Hypothesis
	Id           string `json:"id"`
	GroupID      string `json:"groupid,omitempty"`
	Name         string `json:"name"`
	Organization string `json:"organization,omitempty"`
	Public       bool   `json:"public"`
	Scoped       bool   `json:"scoped"`
	Type         string `json:"type"`
	Links        Links  `json:"links,omitempty"`
}

func (grp Group) GetAnnotations(callback func(ann Annotation) error) error {
	return grp.hypothesis.Search(map[string]string{"group": grp.Id}, callback)
}
