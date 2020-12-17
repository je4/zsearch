package hypothesis

import "time"

type Selector struct {
	Type string `json:"type"`
}

type RangeSelector struct {
	Selector
	EndOffset      int64  `json:"endOffset"`
	StartOffset    int64  `json:"startOffset"`
	EndContainer   string `json:"endContainer,omitempty"`
	StartContainer string `json:"startContainer,omitempty"`
}

type TextPositionSelector struct {
	Selector
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

type TextQuoteSelector struct {
	Selector
	Exact  string `json:"exact"`
	Prefix string `json:"prefix"`
	Suffix string `json:"suffix"`
}

type Target struct {
	Source   string                   `json:"source"`
	Selector []map[string]interface{} `json:"selector"`
}

type Document struct {
	Title []string `toml:"title"`
}

type Annotation struct {
	hypothesis  *Hypothesis
	Id          string            `json:"id"`
	Created     time.Time         `json:"created"`
	Updated     time.Time         `json:"updated"`
	User        string            `json:"user"`
	Uri         string            `json:"uri"`
	Text        string            `json:"text,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Group       string            `json:"group,omitempty"`
	Permissions Permission        `json:"permissions,omitempty"`
	Target      []Target          `json:"target,omitempty"`
	Document    Document          `json:"document,omitempty"`
	Links       Links             `json:"links"`
	Flagged     bool              `json:"flagged"`
	Hidden      bool              `json:"hidden"`
	UserInfo    map[string]string `json:"user_info,omitempty"`
}

type AnnotationList struct {
	Total int64        `toml:"total"`
	Rows  []Annotation `toml:"rows"`
}
