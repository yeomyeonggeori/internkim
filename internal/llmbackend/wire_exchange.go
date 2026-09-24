package llmbackend

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type WireExchange struct {
	Endpoint string `json:"endpoint"`
	Request  string `json:"request"`
	Response string `json:"response"`
}

func (snapshot ExchangeCaptureSnapshot) AnsweringExchange() (WireExchange, bool) {
	attempt, isAnswered := snapshot.answeringAttempt()
	if !isAnswered {
		return WireExchange{}, false
	}
	return WireExchange{
		Endpoint: attempt.URL,
		Request:  attempt.RequestBody,
		Response: answerDocument(attempt.ResponseBody),
	}, true
}

func (snapshot ExchangeCaptureSnapshot) answeringAttempt() (ExchangeCaptureAttempt, bool) {
	answering := ExchangeCaptureAttempt{}
	isAnswered := false
	for _, attempt := range snapshot.Attempts {
		if !attempt.hasAnswered() {
			continue
		}
		if !isAnswered || attempt.finishedAt().After(answering.finishedAt()) {
			answering = attempt
			isAnswered = true
		}
	}
	return answering, isAnswered
}

func (attempt ExchangeCaptureAttempt) hasAnswered() bool {
	isSuccessful := attempt.Status >= http.StatusOK && attempt.Status < http.StatusMultipleChoices
	return attempt.Method == http.MethodPost && isSuccessful && attempt.ErrorCategory == "" && attempt.ResponseBody != ""
}

func (attempt ExchangeCaptureAttempt) finishedAt() time.Time {
	return attempt.StartedAt.Add(time.Duration(attempt.DurationMS) * time.Millisecond)
}

func answerDocument(responseBody string) string {
	if !isEventStream(responseBody) {
		return responseBody
	}
	response, errorValue := assembleStream(strings.NewReader(responseBody), &streamProgress{})
	if errorValue != nil {
		return responseBody
	}
	document, errorValue := json.Marshal(response)
	if errorValue != nil {
		return responseBody
	}
	return string(document)
}

func isEventStream(responseBody string) bool {
	trimmed := strings.TrimSpace(responseBody)
	return strings.HasPrefix(trimmed, "data:") || strings.HasPrefix(trimmed, ":")
}
