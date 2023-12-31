package translate

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"github.com/andybalholm/brotli"
	"github.com/dgraph-io/badger/v4"
	"github.com/op/go-logging"
	"github.com/pkg/errors"
	"golang.org/x/text/language"
	"io"
	"net/http"
	"slices"
	"strings"
)

func NewDeeplTranslator(deeplApiKey, deeplUrl string, badger *badger.DB, logger *logging.Logger) Translator {
	return &DeeplTranslator{deeplUrl: deeplUrl, deeplApiKey: deeplApiKey, badger: badger, logger: logger}
}

type Translator interface {
	Translate(text *MultiLangString, targetLang []language.Tag) error
}

type DeeplTranslator struct {
	deeplApiKey string
	deeplUrl    string
	badger      *badger.DB
	logger      *logging.Logger
}

var languageHierarchy = []language.Tag{language.English, language.German, language.French, language.Italian, language.Und}

func (t *DeeplTranslator) Deepl(str string, targetLang language.Tag) (string, language.Tag, error) {
	base, _ := targetLang.Base()
	jsonStr, err := json.Marshal(struct {
		Text       []string `json:"text"`
		TargetLang string   `json:"target_lang"`
	}{
		Text:       []string{str},
		TargetLang: strings.ToUpper(base.String()),
	})
	if err != nil {
		return "", language.Und, errors.Wrapf(err, "cannot marshal json for deepl request")
	}
	client := &http.Client{}
	req, err := http.NewRequest("POST", t.deeplUrl+"/v2/translate", bytes.NewBuffer(jsonStr))
	if err != nil {
		return "", language.Und, errors.Wrapf(err, "cannot create request for deepl")
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "DeepL-Auth-Key "+t.deeplApiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", language.Und, errors.Wrapf(err, "cannot post request to deepl")
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", language.Und, errors.Wrapf(err, "cannot read response from deepl")
	}
	resp.Body.Close()
	responseStruct := struct {
		Message      string `json:"message,omitempty"`
		Translations []struct {
			Text                   string `json:"text,omitempty"`
			DetectedSourceLanguage string `json:"detected_source_language,omitempty"`
		} `json:"translations,omitempty"`
	}{}
	if err := json.Unmarshal(data, &responseStruct); err != nil {
		return "", language.Und, errors.Wrapf(err, "cannot unmarshal response from deepl: %s", string(data))
	}
	if responseStruct.Message != "" {
		return "", language.Und, errors.Errorf("error from deepl: %s", responseStruct.Message)
	}
	if len(responseStruct.Translations) == 0 {
		return "", language.Und, errors.Errorf("no translation returned from deepl: %s", string(data))
	}
	sourceLang, err := language.Parse(responseStruct.Translations[0].DetectedSourceLanguage)
	if err != nil {
		sourceLang = language.Und
	}
	return responseStruct.Translations[0].Text, sourceLang, nil
}

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
	var key = []byte(fmt.Sprintf("language-%x", sha1.Sum([]byte(sourcetext+sourcelang.String()))))
	t.badger.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			t.logger.Infof("cache miss value for key %s", string(key))
			return nil
		}
		if err := item.Value(func(val []byte) error {
			br := brotli.NewReader(bytes.NewReader(val))
			jsonBytes, err := io.ReadAll(br)
			if err != nil {
				return errors.Wrapf(err, "cannot read value from item for key %s", string(key))
			}
			if err := json.Unmarshal(jsonBytes, &text); err != nil {
				return errors.Wrapf(err, "cannot unmarshal json for key %s", string(key))
			}
			t.logger.Infof("cache hit value for key %s", string(key))
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

		translation, detectedLanguage, err := t.Deepl(sourcetext, lang)
		if err != nil {
			t.logger.Errorf("cannot translate %s: %v", sourcetext, err)
			continue
		}
		// set source language if undefined
		if detectedLanguage != language.Und && sourcelang == language.Und {
			sourcelang = detectedLanguage
			text.SetLang(sourcetext, sourcelang, false)
		}
		text.Set(translation, lang, true)
		t.logger.Infof("translated from %s to %s", sourcelang.String(), lang.String())
		modified = true
	}
	if modified {
		if err := t.badger.Update(func(txn *badger.Txn) error {
			jsonStr, err := json.Marshal(text)
			if err != nil {
				return errors.Wrapf(err, "cannot marshal json for key %s", string(key))
			}
			buf := &bytes.Buffer{}
			wr := brotli.NewWriter(buf)
			if _, err := wr.Write(jsonStr); err != nil {
				return errors.Wrapf(err, "cannot write value for key %s", string(key))
			}
			if err := wr.Close(); err != nil {
				return errors.Wrapf(err, "cannot close writer for key %s", string(key))
			}
			if err := txn.Set(key, buf.Bytes()); err != nil {
				return errors.Wrapf(err, "cannot set value for key %s", string(key))
			}
			return nil
		}); err != nil {
			return errors.Wrapf(err, "cannot set value for key %s", string(key))
		}
	}
	return nil
}
