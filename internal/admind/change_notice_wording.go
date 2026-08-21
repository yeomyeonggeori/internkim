package admind

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

const changeNoticeWordingTimeout = 20 * time.Second

func (service *Service) changeNoticeWording(ctx context.Context, factLines []string, detailBlock string) string {
	facts := strings.TrimSpace(strings.Join(nonEmptyLines(factLines), "\n"))
	if facts == "" {
		return detailBlock
	}
	sentence := service.generateChangeNoticeSentence(ctx, facts)
	if sentence == "" {
		return detailBlock
	}
	return strings.TrimSpace(sentence + "\n\n" + detailBlock)
}

func (service *Service) generateChangeNoticeSentence(ctx context.Context, facts string) string {
	requestDocument, errorValue := json.Marshal(changeNoticeWordingRequest(facts))
	if errorValue != nil {
		return ""
	}
	wordingContext, cancel := context.WithTimeout(ctx, changeNoticeWordingTimeout)
	defer cancel()
	responseDocument, errorValue := service.callCapabilityStructuredLLM(wordingContext, requestDocument)
	if errorValue != nil {
		return ""
	}
	var response capabilityLLMResponse
	if errorValue := json.Unmarshal(responseDocument, &response); errorValue != nil {
		return ""
	}
	var wording struct {
		Sentence string `json:"sentence"`
	}
	if errorValue := json.Unmarshal([]byte(response.Content), &wording); errorValue != nil {
		return ""
	}
	return strings.TrimSpace(wording.Sentence)
}

func changeNoticeWordingRequest(facts string) map[string]any {
	return map[string]any{
		"executionMode": "remote",
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are an office assistant writing a one-sentence Korean heads-up about a change to someone's schedule or task. Write only what the facts state. Keep it to one plain sentence under 60 characters, no greeting, no sign-off, no emoji, no markdown. Never invent a reason, a person, or a time the facts do not contain.",
			},
			{
				"role":    "user",
				"content": "Facts:\n" + facts,
			},
		},
		"structuredOutputSchema": map[string]any{
			"name":               "change_notice_wording",
			"isStrictlyEnforced": true,
			"document": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"sentence"},
				"properties": map[string]any{
					"sentence": map[string]any{"type": "string"},
				},
			},
		},
	}
}

func nonEmptyLines(lines []string) []string {
	kept := []string{}
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			kept = append(kept, trimmed)
		}
	}
	return kept
}
