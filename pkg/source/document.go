package source

type Document struct {
	Content *SourceData         `json:"content,omitempty"`
	ACL     map[string][]string `json:"acl,omitempty"`
	Id      string              `json:"id"`
	Source  string              `json:"source,omitempty"`
	Catalog []string            `json:"catalog,omitempty"`
	Tag     []string            `json:"tag,omitempty"`
	Error   string              `json:"error"`
}
