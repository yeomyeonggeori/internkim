package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

type fakeLearningAdminAPIClient struct {
	document string
	requests []string
}

func (client *fakeLearningAdminAPIClient) request(method string, path string, requestBody any, responseBody any) ([]byte, error) {
	client.requests = append(client.requests, method+" "+path)
	if responseBody != nil {
		if errorValue := json.Unmarshal([]byte(client.document), responseBody); errorValue != nil {
			return nil, errorValue
		}
	}
	return []byte(client.document), nil
}

func TestLearningShowsReviewsSkillsAndSoulRevisions(t *testing.T) {
	output := withCapturedTaskCommandOutput(t)
	client := &fakeLearningAdminAPIClient{document: `{
		"settings": {"enabled": true, "activeLimit": 20},
		"reviews": [{"id": "20260920T010203.000000000", "audience": "person:first", "decision": {"action": "create", "skillID": "weekly-report", "reason": "repeated report procedure"}}],
		"skills": [{"id": "weekly-report", "version": 1, "audience": "person:first", "instruction": "Sample steps", "status": "active"}],
		"soulHistory": [{"version": 2, "origin": "reflection", "document": {"schemaVersion": 1}, "reason": "general principle"}],
		"pendingCount": 3,
		"reviewedCount": 7
	}`}

	if errorValue := showLearningWithClient(nil, client); errorValue != nil {
		t.Fatal(errorValue)
	}

	if client.requests[0] != "GET /diagnostics/learning" {
		t.Fatalf("requests = %v", client.requests)
	}
	for _, expected := range []string{"enabled: true", "pending: 3", "reviewed: 7", "create", "weekly-report", "12B", "reflection"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("output lacks %q:\n%s", expected, output.String())
		}
	}
}
