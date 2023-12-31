package translate

import (
	"encoding/json"
	"github.com/pkg/errors"
	"golang.org/x/text/language"
	"regexp"
	"strings"
)

type multiLangString struct {
	lang       language.Tag
	str        string
	translated bool
}

type MultiLangString []multiLangString

const translatePostfix = "t:"

var langPrefixRegexp = regexp.MustCompile(`^(?s)(.*) ::(` + translatePostfix + `)?([a-z]{3})$`)

func (m *MultiLangString) String() string {
	if len(*m) == 0 {
		return ""
	}
	str := m.Get(language.Und)
	if str != "" {
		return str
	}
	nLangs := m.GetNativeLanguages()
	if len(nLangs) > 0 {
		return m.Get(nLangs[0])
	}
	return (*m)[0].str
}

func (m *MultiLangString) Get(lang language.Tag) string {
	for _, v := range *m {
		if v.lang == lang {
			return v.str
		}
	}
	return ""
}

func (m *MultiLangString) GetNativeLanguages() []language.Tag {
	var result []language.Tag
	for _, v := range *m {
		if v.translated {
			continue
		}
		result = append(result, v.lang)
	}
	return result
}

func (m *MultiLangString) GetLanguages() []language.Tag {
	var result []language.Tag
	for _, v := range *m {
		result = append(result, v.lang)
	}
	return result
}

func (m *MultiLangString) GetTranslatedLanguages() []language.Tag {
	var result []language.Tag
	for _, v := range *m {
		if !v.translated {
			continue
		}
		result = append(result, v.lang)
	}
	return result
}

func (m *MultiLangString) Remove(lang language.Tag) {
	var result = []multiLangString{}
	for _, v := range *m {
		if v.lang != lang {
			result = append(result, v)
		}
	}
	*m = result
}

var mlStringRegexpStart = regexp.MustCompile(`^(?is)[a-z]{2,3}: .*$`)
var mlStringRegexp = regexp.MustCompile(`(?is)([a-z]{2,3}): `)

func (m *MultiLangString) Set(str string, lang language.Tag, translated bool) {
	str = strings.TrimSpace(str)
	if mlStringRegexpStart.MatchString(str) {
		indexes := mlStringRegexp.FindAllStringSubmatchIndex(str, -1)
		if indexes != nil {
			num := len(indexes)
			for i, ind := range indexes {
				langStr := str[ind[2]:ind[3]]
				textStr := ""
				if i < num-1 {
					textStr = strings.TrimSpace(str[ind[1]:indexes[i+1][0]])
				} else {
					textStr = strings.TrimSpace(str[ind[1]:])
				}
				nLang, err := language.Parse(langStr)
				if err != nil {
					nLang = language.Und
				}
				m.Remove(nLang)
				*m = append(*m, multiLangString{lang: nLang, str: textStr, translated: false})
			}
			return
		}
	}
	m.Remove(lang)
	*m = append(*m, multiLangString{lang: lang, str: str, translated: translated})
}

func (m *MultiLangString) MarshalJSON() ([]byte, error) {
	strList := []string{}
	for _, v := range *m {
		base, _ := v.lang.Base()
		if v.translated {
			strList = append(strList, v.str+" ::"+translatePostfix+base.ISO3())
		} else {
			if v.lang == language.Und {
				strList = append(strList, v.str)
			} else {
				strList = append(strList, v.str+" ::"+base.ISO3())
			}
		}
	}
	if len(strList) == 0 {
		return []byte(""), nil
	}
	if len(strList) == 1 {
		return json.Marshal(strList[0])
	}
	return json.Marshal(strList)
}

func (m *MultiLangString) UnmarshalJSON(data []byte) error {
	var strList []string
	if err := json.Unmarshal(data, &strList); err != nil {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return errors.Wrapf(err, "cannot unmarshal %s", string(data))
		}
		strList = []string{str}
	}
	*m = (*m)[:0]
	for _, str := range strList {
		matches := langPrefixRegexp.FindStringSubmatch(str)
		if matches == nil {
			m.Set(str, language.Und, false)
			continue
		}
		lang, err := language.Parse(matches[3])
		if err != nil {
			return errors.Wrapf(err, "cannot parse language %s", matches[3])
		}
		m.Set(matches[1], lang, matches[2] == translatePostfix)
	}
	return nil
}

func (m *MultiLangString) SetLang(sourcetext string, lang language.Tag, b bool) {
	for i, _ := range *m {
		if (*m)[i].str == sourcetext {
			(*m)[i].lang = lang
			return
		}
	}
}
