package translate

import (
	"encoding/json"
	"golang.org/x/text/language"
	"testing"
)

func TestMultilangString(t *testing.T) {
	jsonStr := `["bête ::t:fra","animal","Tier ::deu"]`
	var str = &MultiLangString{}
	if err := json.Unmarshal([]byte(jsonStr), &str); err != nil {
		t.Errorf("cannot unmarshal string: %v", err)
	}
	jsonStr2, err := json.Marshal(str)
	if err != nil {
		t.Errorf("cannot marshal string: %v", err)
	}
	if string(jsonStr2) != jsonStr {
		t.Errorf("unexpected result: %s", string(jsonStr2))
	}

	if str.String() != "animal" {
		t.Errorf("String() returned unexpected value: %s", str.String())
	}

	nl := str.GetNativeLanguages()
	if len(nl) != 2 {
		t.Errorf("GetNativeLanguages() returned %d languages, expected 2: %v", len(nl), nl)
	}

	for _, l := range nl {
		if l != language.German && l != language.Und {
			t.Errorf("GetNativeLanguages() returned unexpected language %s", l)
		}
	}
	nl = str.GetTranslatedLanguages()
	if len(nl) != 1 {
		t.Errorf("GetTranslatedLanguages() returned %d languages, expected 1: %v", len(nl), nl)
	}
	for _, l := range nl {
		if l != language.French {
			t.Errorf("GetTranslatedLanguages() returned unexpected language %s", l)
		}
	}
	str.Set("bestia", language.Spanish, false)
	if str.Get(language.Spanish) != "bestia" {
		t.Errorf("Set() did not set the value")
	}

	str.Set("el bicho", language.Spanish, false)
	if str.Get(language.Spanish) != "el bicho" {
		t.Errorf("Set() did not set the value: %s", str.Get(language.Spanish))
	}
	frStr := str.Get(language.French)
	if frStr != "bête" {
		t.Errorf("Get() returned unexpected value for French: %s", frStr)
	}
}

func TestMultilangString2(t *testing.T) {
	str := &MultiLangString{}
	str.Set("animal", language.Und, false)
	jsonStr, err := json.Marshal(str)
	if err != nil {
		t.Errorf("cannot marshal string: %v", err)
	}
	if string(jsonStr) != `"animal"` {
		t.Errorf("unexpected result: %s", string(jsonStr))
	}
}
