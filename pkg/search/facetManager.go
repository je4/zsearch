package search

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"net/http"
	"net/url"
	"regexp"
)

type FacetManager struct {
	DefaultFacets SolrFacetList
	Facets        SolrFacetList
	facetsOut     map[string]map[string]int64
}

func NewFacetManager(defaults SolrFacetList) *FacetManager {
	return &FacetManager{
		DefaultFacets: defaults,
		Facets:        SolrFacetList{},
		facetsOut:     map[string]map[string]int64{},
	}
}

var facetFormRegexp = regexp.MustCompile(`^facet_([^_]+)_(.+)$`)

func (fm *FacetManager) fromRequest(req http.Request) error {
	fm.Facets = fm.DefaultFacets

	jsonFacet := ""
	if jVal, ok := req.URL.Query()["jsonFacet"]; ok {
		if len(jVal) < 1 {
			return fmt.Errorf("no data in jsonFacet")
		}
		jsonFacet = jVal[0]
	}
	if jVal, ok := req.Form["jsonFacet"]; ok {
		if len(jVal) < 1 {
			return fmt.Errorf("no data in jsonFacet")
		}
		jsonFacet = jVal[0]
	}
	if jsonFacet != "" {
		var facets []SolrFacet
		if err := json.Unmarshal([]byte(jsonFacet), &facets); err != nil {
			return errors.Wrapf(err, "cannut unmarshal jsonFacet %s", jsonFacet)
		}
		for _, facet := range facets {
			name := facet.Name
			if name == "" {
				name = facet.Field
				facet.Name = name
			}
			if _, ok := fm.Facets[name]; !ok {
				fm.Facets[name] = facet
			} else {
				_facet := fm.Facets[name]
				_facet.Name = name
				_facet.Field = facet.Field
				_facet.Label = facet.Label
				_facet.Prefix = facet.Prefix
				_facet.Restrict = facet.Restrict
			}
		}
	}

	paramList := url.Values{}
	// initialize from form value
	if req.Form != nil {
		paramList = req.Form
	}
	for name, vals := range req.URL.Query() {
		paramList[name] = vals
	}

	for paramName, states := range paramList {
		matches := facetFormRegexp.FindStringSubmatch(paramName)
		if matches != nil {
			name := matches[1]
			val := matches[2]
			//fmt.Sprintf("%v, %v", name, val)
			for _, state := range states {
				f, ok := fm.Facets[name]
				if !ok {
					f = SolrFacet{
						Label:    name,
						Name:     name,
						Field:    name,
						Prefix:   "",
						Restrict: map[string]bool{},
					}
				}
				f.Restrict[val] = fmt.Sprintf("%v", state) == "true"
				fm.Facets[name] = f
			}
		}
	}
	return nil
}
