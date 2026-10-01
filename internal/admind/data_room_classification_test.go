package admind

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"gitlab.com/eastriver/internkim/internal/centralplane"
	"gitlab.com/eastriver/internkim/internal/llmbackend"
)

func defaultDataRoomCategories(t *testing.T) []centralplane.DataRoomCategory {
	t.Helper()
	document, errorValue := os.ReadFile("../../web/src/lib/data-room/template.json")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var template struct {
		Categories []centralplane.DataRoomCategory `json:"categories"`
	}
	if errorValue := json.Unmarshal(document, &template); errorValue != nil {
		t.Fatal(errorValue)
	}
	return template.Categories
}

func TestDataRoomClassificationOffersEveryFilingCategoryOnceWithinChoiceLimits(t *testing.T) {
	categories := defaultDataRoomCategories(t)
	questions := dataRoomClassificationQuestions(categories)
	if len(questions) != 2 {
		t.Fatalf("wanted two choices, got %d", len(questions))
	}
	if len(optionsOf(t, questions["group1part1"])) != 19 || len(optionsOf(t, questions["group2part1"])) != 20 {
		t.Fatal("the semantic groups must contain 18 and 19 categories plus other")
	}
	for _, parent := range categories {
		if parent.ChoiceGroup == nil {
			continue
		}
		for _, child := range categories {
			if child.Parent == nil || *child.Parent != parent.Code {
				continue
			}
			question := fmt.Sprintf("group%dpart1", *parent.ChoiceGroup)
			if optionsOf(t, questions[question])[child.Code] == "" {
				t.Fatalf("%s left its parent's semantic group", child.Code)
			}
		}
	}
	occurrences := map[string]int{}
	for name, question := range questions {
		options := optionsOf(t, question)
		if len(options) > 32 || options["other"] == "" {
			t.Fatalf("invalid choice %s: %v", name, options)
		}
		for code := range options {
			if code != "other" {
				occurrences[code]++
			}
		}
	}
	for _, category := range categories {
		if category.Parent == nil {
			continue
		}
		if occurrences[category.Code] != 1 {
			t.Fatalf("category %s offered %d times", category.Code, occurrences[category.Code])
		}
	}
	if occurrences["X"] != 0 {
		t.Fatal("X is the result of other, not a competing filing destination")
	}
}

func TestDataRoomClassificationSelectsTheHigherProbabilityAndKeepsTiesInTheInbox(t *testing.T) {
	questions := map[string]llmbackend.DecisionQuestion{
		"partition1": llmbackend.ChoiceQuestion("first", map[string]string{"FS": "Statements", "other": "Other"}),
		"partition2": llmbackend.ChoiceQuestion("second", map[string]string{"HA": "Appraisals", "other": "Other"}),
	}
	for _, sample := range []struct {
		first, second                       string
		firstProbability, secondProbability float64
		expected                            string
	}{
		{"FS", "other", 0.8, 0.9, "FS"},
		{"other", "HA", 0.9, 0.8, "HA"},
		{"other", "other", 0.9, 0.9, "X"},
		{"FS", "HA", 0.8, 0.6, "FS"},
		{"FS", "HA", 0.6, 0.8, "HA"},
		{"FS", "HA", 0.8, 0.8, "X"},
		{"invented", "HA", 0.9, 0.8, "X"},
		{"FS", "HA", -1, 0.8, "X"},
	} {
		answers := map[string]llmbackend.DecisionAnswer{
			"partition1": {Type: "choice", Choice: sample.first, Probabilities: map[string]float64{sample.first: sample.firstProbability}},
			"partition2": {Type: "choice", Choice: sample.second, Probabilities: map[string]float64{sample.second: sample.secondProbability}},
		}
		if category := dataRoomClassificationOf(answers, questions); category != sample.expected {
			t.Fatalf("%s/%s (%.2f/%.2f) became %s, wanted %s", sample.first, sample.second, sample.firstProbability, sample.secondProbability, category, sample.expected)
		}
	}
	if category := dataRoomClassificationOf(map[string]llmbackend.DecisionAnswer{}, questions); category != "X" {
		t.Fatalf("missing answers became %s", category)
	}
}
