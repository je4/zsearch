package search

type tElastidHighlightField struct {
	HiglightQuery *tElasticQuery `json:"highlight_query,omitempty"`
}

func elasticHighlightField() *tElastidHighlightField {
	return &tElastidHighlightField{}
}

type tElasticHighlight struct {
	Fields   map[string]*tElastidHighlightField `json:"fields"`
	PreTags  []string                           `json:"pre_tags,omitempty"`
	PostTags []string                           `json:"post_tags,omitempty"`
}

func (eh *tElasticHighlight) withTags(pre, post []string) *tElasticHighlight {
	eh.PreTags = pre
	eh.PostTags = post
	return eh
}
func (eh *tElasticHighlight) withField(name string, ehf *tElastidHighlightField) *tElasticHighlight {
	eh.Fields[name] = ehf
	return eh
}
func elasticHighlight() *tElasticHighlight {
	return &tElasticHighlight{
		Fields: make(map[string]*tElastidHighlightField),
	}
}
