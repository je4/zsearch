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

func TestMultilangString3(t *testing.T) {
	str := `
EN: One of Jill Scott’s favourite healing meditations during her illness 
of breast-cancer was to imagine that she could shrink her body down into 
a molecule and wander around inside her own body. From this perspective, 
she would travel around with a garden-hose to wash out all the cancer cells. 
It was this experience, which influenced the performance of Continental Drift. 
On the floors of the space, she had drawn an abstract shape of her own body. 
In place of the head, stood two monitors (facing the audience like 2 eyes) 
and she rested on a horizontal plank above. When each viewer entered the 
gallery space, he or she was asked to sit on one part of the body (organ). 
During the performance she was blindfolded, and she moved around to interact 
with each audience member (organ). The audience were treated like the cells 
in her meditation, and as a metaphor for the hose, she used a miniature security 
camera, which via a long cable supplied direct live feedback to one of the monitors. 
She looked like a patient with a hospital drip.  Meanwhile on the other monitor, a 
video played that symbolized the different eastern and western approaches to such 
an illness.
DE: Eine Heilmeditation von Jill Scott während ihrer Brustkrebserkrankung war die 
Vorstellung, in ihrem eigenen Körper als winziges Molekül herumzuwandern und aus 
dieser Perspektive alle Krebszellen wie mit einem Gartenschlauch heraus zu spülen. 
Für die Performance zeichnete Jill Scott auf den Boden des Zuschauerraums eine 
abstrakte Form ihres eigenen Körpers mit ihren verschiedenen Organen. Anstelle 
des Kopfes standen zwei Monitore (die das Publikum wie Augen anblickten). Zu 
Beginn lag Jill Scott auf einem horizontalen Brett über den Monitoren und jede/r 
Zuschauer/in wurde aufgefordert, sich auf einen Teil des gezeichneten Körpers 
(einem Organ) auf den Boden zu setzen. Während der Aufführung bewegte sich Jill 
Scott mit verbundenen Augen durch das Publikum, um mit einzelnen Zuschauer/innen 
(Organen) zu interagieren. Das Publikum wurde wie die Zellen in ihrer Meditation 
behandelt und als Metapher für den Schlauch benutzte Jill Scott eine 
Miniatur-Überwachungskamera, die über ein langes Kabel ein Live-Feedback auf 
einen der Monitore lieferte.  Jill Scott sah aus wie ein Patient am Tropf eines 
Krankenhauses. Auf dem anderen Monitor lief ein Video, das die unterschiedlichen 
philosophischen Ansätze östlicher und westlicher Medizin gegen Krankheit wie 
Krebssymbolisierte.
`
	str2 := &MultiLangString{}
	str2.Set(str, language.Und, false)
	langs := str2.GetLanguages()
	if len(langs) != 2 {
		t.Errorf("GetLanguages() returned %d languages, expected 2: %v", len(langs), langs)
	}
	if str2.Get(language.German) != `Eine Heilmeditation von Jill Scott während ihrer Brustkrebserkrankung war die 
Vorstellung, in ihrem eigenen Körper als winziges Molekül herumzuwandern und aus 
dieser Perspektive alle Krebszellen wie mit einem Gartenschlauch heraus zu spülen. 
Für die Performance zeichnete Jill Scott auf den Boden des Zuschauerraums eine 
abstrakte Form ihres eigenen Körpers mit ihren verschiedenen Organen. Anstelle 
des Kopfes standen zwei Monitore (die das Publikum wie Augen anblickten). Zu 
Beginn lag Jill Scott auf einem horizontalen Brett über den Monitoren und jede/r 
Zuschauer/in wurde aufgefordert, sich auf einen Teil des gezeichneten Körpers 
(einem Organ) auf den Boden zu setzen. Während der Aufführung bewegte sich Jill 
Scott mit verbundenen Augen durch das Publikum, um mit einzelnen Zuschauer/innen 
(Organen) zu interagieren. Das Publikum wurde wie die Zellen in ihrer Meditation 
behandelt und als Metapher für den Schlauch benutzte Jill Scott eine 
Miniatur-Überwachungskamera, die über ein langes Kabel ein Live-Feedback auf 
einen der Monitore lieferte.  Jill Scott sah aus wie ein Patient am Tropf eines 
Krankenhauses. Auf dem anderen Monitor lief ein Video, das die unterschiedlichen 
philosophischen Ansätze östlicher und westlicher Medizin gegen Krankheit wie 
Krebssymbolisierte.` {
		t.Errorf("Get() returned unexpected value for German: %s", str2.Get(language.German))
	}
	if str2.Get(language.English) != `One of Jill Scott’s favourite healing meditations during her illness 
of breast-cancer was to imagine that she could shrink her body down into 
a molecule and wander around inside her own body. From this perspective, 
she would travel around with a garden-hose to wash out all the cancer cells. 
It was this experience, which influenced the performance of Continental Drift. 
On the floors of the space, she had drawn an abstract shape of her own body. 
In place of the head, stood two monitors (facing the audience like 2 eyes) 
and she rested on a horizontal plank above. When each viewer entered the 
gallery space, he or she was asked to sit on one part of the body (organ). 
During the performance she was blindfolded, and she moved around to interact 
with each audience member (organ). The audience were treated like the cells 
in her meditation, and as a metaphor for the hose, she used a miniature security 
camera, which via a long cable supplied direct live feedback to one of the monitors. 
She looked like a patient with a hospital drip.  Meanwhile on the other monitor, a 
video played that symbolized the different eastern and western approaches to such 
an illness.` {
		t.Errorf("Get() returned unexpected value for English: %s", str2.Get(language.English))
	}
}
