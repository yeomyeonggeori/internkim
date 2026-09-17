package personname

import (
	"encoding/json"
	"os"
	"testing"
)

func TestEveryRecordedNameReadsAsTheSharedCasesSay(t *testing.T) {
	document, errorValue := os.ReadFile("../../web/src/lib/person-name-cases.json")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var cases struct {
		Cases []struct {
			Recorded string `json:"recorded"`
			Locale   string `json:"locale"`
			Rendered string `json:"rendered"`
		} `json:"cases"`
	}
	if errorValue := json.Unmarshal(document, &cases); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(cases.Cases) == 0 {
		t.Fatal("the shared cases are what keeps the two renderings the same")
	}
	for _, testCase := range cases.Cases {
		if rendered := Render(testCase.Recorded, testCase.Locale); rendered != testCase.Rendered {
			t.Errorf("Render(%q, %q) = %q, want %q", testCase.Recorded, testCase.Locale, rendered, testCase.Rendered)
		}
	}
}

func TestFirstNameReadsTheRecordedGivenName(t *testing.T) {
	for recorded, expected := range map[string]string{
		"샘플 이":               "샘플",
		"John Michael":       "John",
		"John Michael Smith": "John Michael",
		"  ":                 "",
	} {
		if firstName := FirstName(recorded); firstName != expected {
			t.Errorf("FirstName(%q) = %q, want %q", recorded, firstName, expected)
		}
	}
}

func TestDefaultCallMeReadsTheSharedCasesSay(t *testing.T) {
	document, errorValue := os.ReadFile("../../web/src/lib/person-call-me-cases.json")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var cases struct {
		Cases []struct {
			Recorded string `json:"recorded"`
			Locale   string `json:"locale"`
			CallMe   string `json:"callMe"`
		} `json:"cases"`
	}
	if errorValue := json.Unmarshal(document, &cases); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, testCase := range cases.Cases {
		if callMe := DefaultCallMe(testCase.Recorded, testCase.Locale); callMe != testCase.CallMe {
			t.Errorf("DefaultCallMe(%q, %q) = %q, want %q", testCase.Recorded, testCase.Locale, callMe, testCase.CallMe)
		}
	}
}

func TestMatchesReadsANameInAnyOrderAndWithoutTheMiddle(t *testing.T) {
	smith := "John Michael Smith"
	for hint, expected := range map[string]bool{
		"John Michael Smith": true,
		"smith john michael": true,
		"johnmichaelsmith":   true,
		"John Smith":         true,
		"Smith John":         true,
		"Smith":              true,
		"smi":                true,
		"John Smyth":         false,
		"Jane":               false,
		"  ":                 false,
	} {
		if isMatch := Matches(hint, smith); isMatch != expected {
			t.Errorf("Matches(%q, %q) = %v, want %v", hint, smith, isMatch, expected)
		}
	}
}

func TestMatchesReadsAKoreanNameInEitherOrder(t *testing.T) {
	lee := "샘플 이"
	for hint, expected := range map[string]bool{
		"이샘플":  true,
		"샘플이":  true,
		"샘플 이": true,
		"이 샘플": true,
		"샘플":   true,
		"이":    true,
		"박샘플":  false,
	} {
		if isMatch := Matches(hint, lee); isMatch != expected {
			t.Errorf("Matches(%q, %q) = %v, want %v", hint, lee, isMatch, expected)
		}
	}
}
