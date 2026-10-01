//go:build llmeval

package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/llmbackend"
)

func TestDataRoomClassificationLiveSemanticGroups(t *testing.T) {
	if os.Getenv("INTERNKIM_LIVE_LLM_TEST") != "1" {
		t.Skip("requires explicit live evaluation")
	}
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		t.Fatal("OPENROUTER_API_KEY is required")
	}
	keyPath := filepath.Join(t.TempDir(), "key")
	if errorValue := os.WriteFile(keyPath, []byte(key), 0600); errorValue != nil {
		t.Fatal(errorValue)
	}
	backend := llmbackend.OpenRouterBackend{KeyPath: keyPath, BaseURL: "https://openrouter.ai/api/v1/chat/completions", HTTPClient: &http.Client{Timeout: 45 * time.Second}}
	categories := defaultDataRoomCategories(t)
	questions := dataRoomClassificationQuestions(categories)
	for _, sample := range []struct{ name, title, text, expected string }{
		{"statements", "Annual financial statements", "Audited consolidated balance sheet, income statement and cash flow statement for the fiscal year.", "FS"},
		{"payroll", "Employee payroll register", "Monthly gross pay, employee withholding, deductions and net salary paid to each employee.", "FP"},
		{"supplier", "Equipment supplier agreement", "The company purchases machine components from a supplier. Includes delivery obligations, purchase prices and warranty terms.", "OS"},
		{"unknown", "Untitled attachment", "No readable content or contextual information is available.", "X"},
	} {
		t.Run(sample.name, func(t *testing.T) {
			state, errorValue := dataRoomClassificationState(dataRoomDocumentDraft{Title: sample.title, Text: sample.text}, categories)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			request := llmbackend.DecisionsRequest{State: state, Questions: questions}
			started := time.Now()
			answer, errorValue := backend.Decide(context.Background(), request)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			saveDataRoomEvaluation(t, sample.name, request, answer, time.Since(started))
			category := dataRoomClassificationOf(answer.Answers, questions)
			t.Logf("category=%s model=%s cost=$%.6f latency=%s", category, answer.Model, answer.Usage.CostUSD, time.Since(started))
			if category != sample.expected {
				t.Fatalf("wanted %s, got %s; answers=%+v", sample.expected, category, answer.Answers)
			}
		})
	}
}

func saveDataRoomEvaluation(t *testing.T, name string, request llmbackend.DecisionsRequest, answer llmbackend.DecisionsResponse, elapsed time.Duration) {
	t.Helper()
	path := filepath.Join("../../.artifacts/llmeval/data-room", time.Now().UTC().Format("20060102T150405")+"-"+name+".json")
	if errorValue := os.MkdirAll(filepath.Dir(path), 0700); errorValue != nil {
		t.Fatal(errorValue)
	}
	document, errorValue := json.MarshalIndent(struct {
		Request             llmbackend.DecisionsRequest  `json:"request"`
		Answer              llmbackend.DecisionsResponse `json:"answer"`
		ElapsedMilliseconds int64                        `json:"elapsedMilliseconds"`
	}{request, answer, elapsed.Milliseconds()}, "", "  ")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(path, document, 0600); errorValue != nil {
		t.Fatal(errorValue)
	}
}
