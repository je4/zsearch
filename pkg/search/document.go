/*
Copyright 2020 Center for Digital Matter HGK FHNW, Basel.
Copyright 2020 info-age GmbH, Basel.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS-IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package search

type Document struct {
	Content *SourceData         `json:"content,omitempty"`
	ACL     map[string][]string `json:"acl,omitempty"`
	Id      string              `json:"id"`
	Source  string              `json:"source,omitempty"`
	Catalog []string            `json:"catalog,omitempty"`
	Tag     []string            `json:"tag,omitempty"`
	Error   string              `json:"error"`
}
