package translate

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"github.com/dgraph-io/badger/v4"
	"github.com/pkg/errors"
	"golang.org/x/text/language"
	"io"
	"net/http"
	"slices"
	"strings"
)

func NewDeeplTranslator(deeplApiKey, deeplUrl string, badger *badger.DB) Translator {
	return &DeeplTranslator{deeplUrl: deeplUrl, deeplApiKey: deeplApiKey, badger: badger}
}

type Translator interface {
	Translate(text *MultiLangString, targetLang []language.Tag) error
}

type DeeplTranslator struct {
	deeplApiKey string
	deeplUrl    string
	badger      *badger.DB
}

var languageHierarchy = []language.Tag{language.English, language.German, language.French, language.Italian, language.Und}

func (t *DeeplTranslator) Translate(text *MultiLangString, targetLang []language.Tag) error {
	var sourcetext string
	var sourcelang language.Tag = language.Und

	languages := text.GetLanguages()
	if len(languages) == 0 {
		return nil
	}
	if len(languages) == 1 {
		sourcetext = text.String()
	} else {
		nativeLanguages := text.GetNativeLanguages()
		for _, l := range languageHierarchy {
			if slices.Contains(nativeLanguages, l) {
				sourcetext = text.Get(l)
				sourcelang = l
				break
			}
		}
	}
	var key = fmt.Sprintf("language-%x", sha1.Sum([]byte(sourcetext)))
	t.badger.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return nil
		}
		if err := item.Value(func(val []byte) error {
			if err := json.Unmarshal(val, &text); err != nil {
				return errors.Wrapf(err, "cannot unmarshal json for key %s", string(key))
			}
			return nil
		}); err != nil {
			return errors.Wrapf(err, "cannot get value from item for key %s", string(key))
		}
		return nil
	})

	var modified = false
	for _, lang := range targetLang {
		languages = text.GetLanguages()
		if slices.Contains(languages, lang) {
			continue
		}
		if sourcetext == "" {
			text.Set("", lang, true)
			continue
		}
		base, _ := lang.Base()
		jsonStr, err := json.Marshal(struct {
			Text       []string `json:"text"`
			TargetLang string   `json:"target_lang"`
		}{
			Text:       []string{sourcetext},
			TargetLang: strings.ToUpper(base.String()),
		})
		if err != nil {
			return errors.Wrapf(err, "cannot marshal json for deepl request")
		}
		client := &http.Client{}
		req, err := http.NewRequest("POST", t.deeplUrl+"/v2/translate", bytes.NewBuffer(jsonStr))
		if err != nil {
			return errors.Wrapf(err, "cannot create request for deepl")
		}
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("Authorization", "DeepL-Auth-Key "+t.deeplApiKey)

		resp, err := client.Do(req)
		if err != nil {
			return errors.Wrapf(err, "cannot post request to deepl")
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return errors.Wrapf(err, "cannot read response from deepl")
		}
		resp.Body.Close()
		responseStruct := struct {
			Translations []struct {
				Text                   string `json:"text"`
				DetectedSourceLanguage string `json:"detected_source_language"`
			} `json:"translations"`
		}{}
		if err := json.Unmarshal(data, &responseStruct); err != nil {
			print(errors.Wrapf(err, "cannot unmarshal response from deepl: %s", string(data)))
			continue
		}
		if len(responseStruct.Translations) == 0 {
			continue
			//return errors.Errorf("no translation returned from deepl: %s", string(data))
		}
		translation := responseStruct.Translations[0].Text
		if responseStruct.Translations[0].DetectedSourceLanguage != "" && sourcelang == language.Und {
			srcLang, err := language.Parse(responseStruct.Translations[0].DetectedSourceLanguage)
			if err == nil {
				sourcelang = srcLang
				text.SetLang(sourcetext, srcLang, false)
			}
		}
		text.Set(translation, lang, true)
		modified = true
	}
	if modified {
		if err := t.badger.Update(func(txn *badger.Txn) error {
			jsonStr, err := json.Marshal(text)
			if err != nil {
				return errors.Wrapf(err, "cannot marshal json for key %s", string(key))
			}
			if err := txn.Set([]byte(key), jsonStr); err != nil {
				return errors.Wrapf(err, "cannot set value for key %s", string(key))
			}
			return nil
		}); err != nil {
			return errors.Wrapf(err, "cannot set value for key %s", string(key))
		}
	}
	return nil
}
