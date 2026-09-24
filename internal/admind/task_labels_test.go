package admind

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
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
		"business": {Choice: "Invented business"},
		"type":     {Choice: noRegisteredTaskType},
		"size":     {Choice: "M"},
	}, questions)

	if labels != (taskLabels{Size: "M"}) {
		t.Fatalf("labels = %+v, want only the offered size", labels)
	}
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

func TestTaskLabelsRefuseAReadOnlyCaller(t *testing.T) {
	service := NewService(Configuration{})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/task/labels", strings.NewReader(`{"title":"Sample work"}`))
	recorder := httptest.NewRecorder()

	service.answerTaskLabels(recorder, request, publicToolGatewayActor{Actor: capabilities.ActorContext{Scopes: []string{publicAPIPermissionRead}}})

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want a read-only caller refused", recorder.Code)
	}
}
