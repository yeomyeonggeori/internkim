package admind

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/llmbackend"
)

func TestTaskLabelQuestionsOfferEverySizeAndOnlyRegisteredLabels(t *testing.T) {
	questions := taskLabelQuestions(taskDefinitions{
		Categories: []string{"Sample business"},
		Types:      []string{"Development", " "},
		Sizes:      defaultTaskSizeDefinitions(),
	})

	if offered := optionsOf(t, questions["size"]); len(offered) != len(defaultTaskSizeDefinitions()) {
		t.Fatalf("size options = %v", offered)
	}
	if offered := optionsOf(t, questions["business"]); len(offered) != 1 || offered["Sample business"] == "" {
		t.Fatalf("business options = %v", offered)
	}
	if offered := optionsOf(t, questions["type"]); len(offered) != 2 || offered[noRegisteredTaskType] == "" {
		t.Fatalf("type options = %v, want Development and none", offered)
	}
}

func TestTaskLabelQuestionsSkipALabelTheCompanyHasNone(t *testing.T) {
	questions := taskLabelQuestions(taskDefinitions{Sizes: defaultTaskSizeDefinitions()})

	if _, asked := questions["business"]; asked {
		t.Fatal("asked for a business the company has not registered")
	}
	if _, asked := questions["type"]; asked {
		t.Fatal("asked for a type the company has not registered")
	}
}

func TestTaskLabelsKeepOnlyOfferedChoices(t *testing.T) {
	questions := taskLabelQuestions(taskDefinitions{
		Categories: []string{"Sample business"},
		Types:      []string{"Development"},
		Sizes:      defaultTaskSizeDefinitions(),
	})

	labels := taskLabelsOfAnswers(map[string]llmbackend.DecisionAnswer{
		"business": {Type: llmbackend.ChoiceQuestionType, Choice: "Invented business", Confidence: confidenceOf(0.9)},
		"type":     {Choice: noRegisteredTaskType},
		"size":     {Choice: "M"},
	}, questions)

	if labels != (taskLabels{Size: "M"}) {
		t.Fatalf("labels = %+v, want only the offered size", labels)
	}
}

func TestTaskLabelsLeaveABusinessEmptyWhenTheChoiceIsAGuess(t *testing.T) {
	questions := taskLabelQuestions(taskDefinitions{
		Categories: []string{"Sample business", "Other business"},
		Sizes:      defaultTaskSizeDefinitions(),
	})
	answersWithBusinessConfidence := func(confidence *float64) map[string]llmbackend.DecisionAnswer {
		return map[string]llmbackend.DecisionAnswer{
			"business": {Type: llmbackend.ChoiceQuestionType, Choice: "Sample business", Confidence: confidence},
			"size":     {Type: llmbackend.ChoiceQuestionType, Choice: "S"},
		}
	}

	if labels := taskLabelsOfAnswers(answersWithBusinessConfidence(confidenceOf(0.49)), questions); labels != (taskLabels{Size: "S"}) {
		t.Fatalf("a guessed business was kept: %+v", labels)
	}
	if labels := taskLabelsOfAnswers(answersWithBusinessConfidence(nil), questions); labels.Business != "" {
		t.Fatalf("a business with no confidence was kept: %+v", labels)
	}
	if labels := taskLabelsOfAnswers(answersWithBusinessConfidence(confidenceOf(0.6)), questions); labels.Business != "Sample business" {
		t.Fatalf("a confident business was dropped: %+v", labels)
	}
}

func confidenceOf(value float64) *float64 {
	return &value
}

func optionsOf(t *testing.T, question llmbackend.DecisionQuestion) map[string]string {
	t.Helper()
	options := map[string]string{}
	if len(question.Criteria) == 0 {
		return options
	}
	if errorValue := json.Unmarshal(question.Criteria, &options); errorValue != nil {
		t.Fatal(errorValue)
	}
	return options
}

func TestTaskLabelsAreDecidedOnlyForAMemberTheRequestNames(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	unnamed := httptest.NewRequest(http.MethodPost, taskLabelsPath, strings.NewReader(`{"title":"Sample work"}`))
	unnamed.RemoteAddr = "198.51.100.10:443"
	unnamedResponse := httptest.NewRecorder()

	service.router().ServeHTTP(unnamedResponse, unnamed)

	if unnamedResponse.Code != http.StatusForbidden {
		t.Fatalf("a call naming nobody = %d %s", unnamedResponse.Code, unnamedResponse.Body.String())
	}

	named := httptest.NewRequest(http.MethodPost, taskLabelsPath, strings.NewReader(`{"title":"Sample work"}`))
	named.Header.Set(requesterEmailHeader, "member@example.com")
	namedResponse := httptest.NewRecorder()
	service.router().ServeHTTP(namedResponse, arrivingOnTheRequesterSocket(named))

	if namedResponse.Code != http.StatusBadGateway || !strings.Contains(namedResponse.Body.String(), "member@example.com") {
		t.Fatalf("the definitions were not read as the member: %d %s", namedResponse.Code, namedResponse.Body.String())
	}
}
