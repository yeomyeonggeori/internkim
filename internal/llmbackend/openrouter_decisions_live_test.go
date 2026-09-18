//go:build llmeval

package llmbackend

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"
)

func liveDecisionsState(t *testing.T) json.RawMessage {
	t.Helper()
	document, errorValue := json.Marshal(map[string]any{
		"assistant": map[string]string{"name": "김인턴", "handle": "@김인턴", "role": "회사의 업무 보조 봇"},
		"channel":   "#일반",
		"recentMessages": []map[string]string{
			{"sender": "이샘플", "text": "@김인턴 지난달 근태 집계해서 정리해줘"},
			{"sender": "김인턴", "text": "9월 1일부터 9월 30일까지 전 직원 근태를 집계해서 문서로 올리겠습니다. 진행할까요?"},
		},
		"newestMessage": map[string]string{"sender": "이샘플", "text": "응 진행해"},
		"pending": map[string]any{
			"confirmation": map[string]string{"question": "전 직원 근태를 집계해서 문서로 올리겠습니다. 진행할까요?", "askedBy": "김인턴"},
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return document
}

func liveDecisionsQuestions() map[string]DecisionQuestion {
	return map[string]DecisionQuestion{
		"target": ChoiceQuestion("Who is the newest message directed at? The assistant is a workplace bot named 김인턴.", map[string]string{
			"bot":     "directed at the assistant 김인턴, by mention, by reply, or by an unmistakable request to it",
			"human":   "directed at one specific named person other than the assistant",
			"anyone":  "directed at the room in general, a share or an announcement anyone may answer",
			"none":    "directed at nobody, a self-note, a reaction, or filler",
			"unclear": "genuinely impossible to tell who it is aimed at",
		}),
		"route": ChoiceQuestion("What should the assistant do about the newest message?", map[string]string{
			"consume":         "nothing; read it and stay silent",
			"answer_question": "answer a question in words, right now",
			"clarify":         "ask the sender one clarifying question before acting",
			"start_task":      "start a piece of work that takes tools and time",
			"approve":         "treat it as approval of a pending confirmation",
			"reject":          "treat it as a refusal of a pending confirmation",
		}),
		"deliverableKind": ChoiceQuestion("What artifact does the sender expect back?", map[string]string{
			"none":     "no artifact",
			"message":  "a written reply only",
			"document": "a document or notes",
			"deck":     "slides",
			"sheet":    "a spreadsheet or a table",
		}),
		"shouldRespond": NoulQuestion("Should the assistant 김인턴 write a reply to the newest message?", map[string]string{
			"true":  "a reply is wanted from the assistant",
			"false": "a share or chatter between other people wants no reply",
		}),
		"relatesToActiveTask": NoulQuestion("Does the newest message refer to work the assistant is already doing?", nil),
		"urgent":              NoulQuestion("Does the newest message need attention today?", nil),
	}
}

func TestOpenRouterLiveAlphaDecisionsRouteAnswersTypedQuestions(t *testing.T) {
	backend, _ := liveOpenRouterBackendFromEnv(t)
	backend.HTTPClient = httpClientWithTimeout(30 * time.Second)

	startedAt := time.Now()
	response, errorValue := backend.Decide(context.Background(), DecisionsRequest{
		State:     liveDecisionsState(t),
		Questions: liveDecisionsQuestions(),
	})
	latency := time.Since(startedAt)
	if errorValue != nil {
		t.Fatalf("the alpha decisions route did not answer: %v", errorValue)
	}

	if len(response.Answers) != len(liveDecisionsQuestions()) {
		t.Fatalf("expected one answer per question, got %d: %+v", len(response.Answers), response.Answers)
	}
	choice, isChoice := response.Answers["target"].AsChoice()
	if !isChoice {
		t.Fatalf("target came back as something other than a choice: %+v", response.Answers["target"])
	}
	probabilityMass := 0.0
	for _, probability := range choice.Probabilities {
		probabilityMass += probability
	}
	if math.Abs(probabilityMass-1) > 0.05 {
		t.Fatalf("the choice probabilities sum to %.3f, not to one: %+v", probabilityMass, choice.Probabilities)
	}
	noul, isNoul := response.Answers["shouldRespond"].AsNoul()
	if !isNoul || noul.Noul < 0 || noul.Noul > 1 {
		t.Fatalf("shouldRespond is not a probability: %+v", response.Answers["shouldRespond"])
	}
	if response.Usage.InputTokens <= 0 {
		t.Fatalf("the decisions response counted no input tokens: %+v", response.Usage)
	}

	t.Logf("jev decided %d questions in %d ms for $%.6f (model=%s provider=%s target=%s p(respond)=%.2f)",
		len(response.Answers), latency.Milliseconds(), response.Usage.CostUSD, response.Model, response.Provider, choice.Choice, noul.Noul)
}
