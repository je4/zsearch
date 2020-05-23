package source

type Document struct {
	Content *SourceData
	ACL     map[string][]string
	Id      string
	Source  string
	Catalog []string
	Tag     []string
}
