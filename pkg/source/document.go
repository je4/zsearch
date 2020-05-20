package source

type Document struct {
	Source     *SourceData
	ACL        map[string][]string
	Id         string
}
