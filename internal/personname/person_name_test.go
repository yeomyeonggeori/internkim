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
