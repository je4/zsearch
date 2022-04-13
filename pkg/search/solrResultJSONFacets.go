package search

import (
	"fmt"
	"github.com/pkg/errors"
)

type SolrResultJSONFacetBucket struct {
	Count int64  `json:"count"`
	Val   string `json:"val"`
}

func NewSolrResultJSONFacetBucket() (*SolrResultJSONFacetBucket, error) {
	return &SolrResultJSONFacetBucket{}, nil
}

func (fb *SolrResultJSONFacetBucket) Init(data interface{}) error {
	eIntList, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid bucket type: %T", data)
	}
	for name, v := range eIntList {
		switch name {
		case "count":
			cnt, ok := v.(float64)
			if !ok {
				return fmt.Errorf("wrong type for bucket field count: %T", v)
			}
			fb.Count = int64(cnt)
		case "val":
			str, ok := v.(string)
			if !ok {
				return fmt.Errorf("wrong type for bucket field val: %T", v)
			}
			fb.Val = str
		default:
			return fmt.Errorf("unknown bucket field: %s", name)
		}
	}
	return nil
}

type SolrResultJSONFacetElement struct {
	//Name    string                      `json:"Name"`
	Buckets []*SolrResultJSONFacetBucket `json:"buckets"`
}

func NewSolrResultJSONFacetElement() (*SolrResultJSONFacetElement, error) {
	return &SolrResultJSONFacetElement{}, nil
}

func (fe *SolrResultJSONFacetElement) Init(data interface{}) error {
	eIntList, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid type for data: %T", data)
	}
	for name, d := range eIntList {
		if name != "buckets" {
			return fmt.Errorf("cannot handle element field %s", name)
		}
		switch t := d.(type) {
		case []interface{}:
			for _, val := range t {
				bucket, err := NewSolrResultJSONFacetBucket()
				if err != nil {
					return errors.Wrap(err, "cannot create new bucket")
				}
				if err := bucket.Init(val); err != nil {
					return errors.Wrapf(err, "cannot initialize bucket with %v", val)
				}
				fe.Buckets = append(fe.Buckets, bucket)
			}
		default:
			return fmt.Errorf("invalid type %T for buckets", t)
		}
	}
	return nil
}

type SolrResultJSONFacets struct {
	Count    int64                                  `json:"count"`
	Elements map[string]*SolrResultJSONFacetElement `json:"elements"`
}

func NewSolrResultJSONFacets() (*SolrResultJSONFacets, error) {
	facets := &SolrResultJSONFacets{
		Count:    0,
		Elements: make(map[string]*SolrResultJSONFacetElement),
	}
	return facets, nil
}

func (fa *SolrResultJSONFacets) Init(data interface{}) error {
	eIntList, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid type for data: %T", data)
	}
	for name, val := range eIntList {
		switch t := val.(type) {
		case float64:
			if name != "count" {
				return fmt.Errorf("invalid Name for number type: %s", name)
			}
			fa.Count = int64(t)
		default:
			if name == "count" {
				return fmt.Errorf("invalid type for count: %T", t)
			}
			var err error
			fa.Elements[name], err = NewSolrResultJSONFacetElement()
			if err != nil {
				return errors.Wrapf(err, "cannot parse facet element of %s", name)
			}
			fa.Elements[name].Init(t)
		}
	}
	return nil
}
