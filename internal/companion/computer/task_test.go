package computer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/llmbackend"
)

type fakeDriver struct {
	pages    []Observation
	pageTurn int
	calls    []string
	failing  string
	isClosed bool
}

type fakeOpener struct {
	driver *fakeDriver
}

func (opener fakeOpener) Open(context.Context, string) (Driver, error) {
	return opener.driver, nil
}

func (driver *fakeDriver) Close() error {
	driver.isClosed = true
	return nil
}

func (driver *fakeDriver) Call(_ context.Context, tool string, arguments map[string]any) (json.RawMessage, error) {
	driver.calls = append(driver.calls, tool+" "+describeArguments(arguments))
	if tool == driver.failing {
		return nil, errors.New("the driver refused")
	}
	switch tool {
	case "browser_prepare":
		return json.RawMessage(`{"status":"ok","prepared_pid":4242}`), nil
	case "list_windows":
		return json.RawMessage(`{"windows":[{"window_id":7,"is_on_screen":false,"bounds":{"width":900,"height":900}},{"window_id":9,"is_on_screen":true,"bounds":{"width":800,"height":600}}]}`), nil
	case "browser_navigate":
		return json.RawMessage(`{"status":"ok"}`), nil
	case "get_browser_state":
		if _, isBinding := arguments["pid"]; isBinding {
			return json.RawMessage(`{"status":"ok","target_id":"bt1","binding_quality":"exact","mutation_allowed":true,"tabs":[{"tab_id":"tab-a","active":false},{"tab_id":"tab-b","active":true}]}`), nil
		}
		return json.Marshal(driver.currentPage())
	}
	driver.pageTurn++
	return json.RawMessage(`{"status":"ok"}`), nil
}

func (driver *fakeDriver) currentPage() Observation {
	if driver.pageTurn >= len(driver.pages) {
		return driver.pages[len(driver.pages)-1]
	}
	return driver.pages[driver.pageTurn]
}

func describeArguments(arguments map[string]any) string {
	document, _ := json.Marshal(arguments)
	return string(document)
}

type scriptedChooser struct {
	answers  []DecisionResponse
	requests []DecisionRequest
}

func (chooser *scriptedChooser) Decide(_ context.Context, request DecisionRequest) (DecisionResponse, error) {
	chooser.requests = append(chooser.requests, request)
	index := len(chooser.requests) - 1
	if index >= len(chooser.answers) {
		return chooser.answers[len(chooser.answers)-1], nil
	}
	return chooser.answers[index], nil
}

func answer(candidateID string, confidence float64, goalReached float64) DecisionResponse {
	return DecisionResponse{Answers: map[string]llmbackend.DecisionAnswer{
		nextActionQuestion:  {Type: llmbackend.ChoiceQuestionType, Choice: candidateID, Confidence: &confidence},
		goalReachedQuestion: {Type: llmbackend.NoulQuestionType, Noul: &goalReached},
	}}
}

func searchPage() Observation {
	return Observation{
		Page:    Page{URL: "https://example.test/", Title: "Example"},
		Outline: "- textbox \"Search\"\n- button \"Go\"",
		Refs: []Reference{
			{Ref: "r1", Role: "textbox", Name: "Search", Actions: []string{"click", "type"}, Visibility: "in_viewport"},
			{Ref: "r2", Role: "button", Name: "Go", Actions: []string{"click", "pointer"}, Visibility: "in_viewport"},
			{Ref: "r3", Role: "link", Name: "Hidden", Actions: []string{"click"}, Visibility: "offscreen"},
		},
	}
}

func resultsPage() Observation {
	return Observation{
		Page:    Page{URL: "https://example.test/results?q=cats", Title: "Results for cats"},
		Outline: "- heading \"Results for cats\"",
	}
}

func noSleep(context.Context, time.Duration) error { return nil }

func TestRunTypesClicksAndVerifiesOnAFreshObservation(t *testing.T) {
	driver := &fakeDriver{pages: []Observation{searchPage(), searchPage(), resultsPage()}}
	chooser := &scriptedChooser{answers: []DecisionResponse{
		answer("type-1-query-into-r1", 0.9, 0.1),
		answer("click-r2", 0.8, 0.2),
		answer(AbstainCandidateID, 0.7, 0.97),
	}}
	runner := Runner{Opener: fakeOpener{driver}, Chooser: chooser, Sleep: noSleep}

	result, errorValue := runner.Run(context.Background(), Request{
		Goal:     "search results for cats are shown",
		StartURL: "https://example.test/",
		Inputs:   []Input{{Name: "query", Text: "cats"}},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.Outcome != OutcomeVerified {
		t.Fatalf("outcome = %q, summary %q", result.Outcome, result.Summary)
	}
	if len(result.Steps) != 2 || result.Steps[0].CandidateID != "type-1-query-into-r1" || result.Steps[1].CandidateID != "click-r2" {
		t.Fatalf("steps = %+v", result.Steps)
	}
	if result.Page.Title != "Results for cats" || !strings.Contains(result.Page.Text, "Results for cats") {
		t.Fatalf("page = %+v", result.Page)
	}
	if !driver.isClosed {
		t.Fatal("the driver was left open")
	}
	expectCalls(t, driver.calls,
		`browser_prepare {"allow_launch":true,"profile":{"mode":"isolated_named","name":"internkim"}}`,
		`list_windows {"pid":4242}`,
		`get_browser_state {"pid":4242,"window_id":9}`,
		`browser_navigate {"tab_id":"tab-b","target_id":"bt1","url":"https://example.test/"}`,
		`get_browser_state {"snapshot_format":"semantic_v2","tab_id":"tab-b","target_id":"bt1"}`,
		`browser_type {"ref":"r1","replace":true,"tab_id":"tab-b","target_id":"bt1","text":"cats"}`,
		`get_browser_state {"snapshot_format":"semantic_v2","tab_id":"tab-b","target_id":"bt1"}`,
		`browser_click {"input_route":"dom_event","ref":"r2","tab_id":"tab-b","target_id":"bt1"}`,
		`get_browser_state {"snapshot_format":"semantic_v2","tab_id":"tab-b","target_id":"bt1"}`,
	)
}

func expectCalls(t *testing.T, calls []string, expected ...string) {
	t.Helper()
	if strings.Join(calls, "\n") != strings.Join(expected, "\n") {
		t.Fatalf("driver calls:\n%s\nexpected:\n%s", strings.Join(calls, "\n"), strings.Join(expected, "\n"))
	}
}

func TestRunOffersOnlyVisibleActionsAndTheReservedCandidates(t *testing.T) {
	driver := &fakeDriver{pages: []Observation{searchPage()}}
	chooser := &scriptedChooser{answers: []DecisionResponse{answer(AbstainCandidateID, 0.5, 0.5)}}
	runner := Runner{Opener: fakeOpener{driver}, Chooser: chooser, Sleep: noSleep}

	result, errorValue := runner.Run(context.Background(), Request{Goal: "anything", Inputs: []Input{{Name: "query", Text: "cats"}}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.Outcome != OutcomeAbstained {
		t.Fatalf("outcome = %q", result.Outcome)
	}
	criteria := map[string]string{}
	if errorValue := json.Unmarshal(chooser.requests[0].Questions[nextActionQuestion].Criteria, &criteria); errorValue != nil {
		t.Fatal(errorValue)
	}
	expected := []string{"type-1-query-into-r1", "enter-in-r1", "click-r1", "click-r2", "scroll-down", "scroll-up", ReobserveCandidateID, AbstainCandidateID}
	if len(criteria) != len(expected) {
		t.Fatalf("criteria = %v", criteria)
	}
	for _, id := range expected {
		if _, isOffered := criteria[id]; !isOffered {
			t.Fatalf("%s was not offered: %v", id, criteria)
		}
	}
	var state decisionState
	if errorValue := json.Unmarshal(chooser.requests[0].State, &state); errorValue != nil {
		t.Fatal(errorValue)
	}
	if state.Goal != "anything" || state.Observation.Page.Title != "Example" || len(state.InputNames) != 1 || state.InputNames[0] != "query" {
		t.Fatalf("state = %+v", state)
	}
	if strings.Contains(string(chooser.requests[0].State), "cats") {
		t.Fatal("the input text itself was sent to the decision model")
	}
}

func TestRunRefutesWhenAbstainingWithNoChanceOfTheGoal(t *testing.T) {
	driver := &fakeDriver{pages: []Observation{searchPage()}}
	chooser := &scriptedChooser{answers: []DecisionResponse{answer(AbstainCandidateID, 0.9, 0.01)}}
	runner := Runner{Opener: fakeOpener{driver}, Chooser: chooser, Sleep: noSleep}

	result, errorValue := runner.Run(context.Background(), Request{Goal: "anything"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.Outcome != OutcomeRefuted {
		t.Fatalf("outcome = %q", result.Outcome)
	}
}

func TestRunFailsClosedOnAnUnlistedCandidate(t *testing.T) {
	driver := &fakeDriver{pages: []Observation{searchPage()}}
	chooser := &scriptedChooser{answers: []DecisionResponse{answer("click-r3", 0.9, 0.1)}}
	runner := Runner{Opener: fakeOpener{driver}, Chooser: chooser, Sleep: noSleep}

	_, errorValue := runner.Run(context.Background(), Request{Goal: "anything"})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "click-r3") {
		t.Fatalf("error = %v", errorValue)
	}
	for _, call := range driver.calls {
		if strings.HasPrefix(call, "browser_click") {
			t.Fatal("an unlisted candidate was acted on")
		}
	}
}

func TestRunReportsUnknownWhenAnActionFails(t *testing.T) {
	driver := &fakeDriver{pages: []Observation{searchPage()}, failing: "browser_click"}
	chooser := &scriptedChooser{answers: []DecisionResponse{answer("click-r2", 0.9, 0.1)}}
	runner := Runner{Opener: fakeOpener{driver}, Chooser: chooser, Sleep: noSleep}

	result, errorValue := runner.Run(context.Background(), Request{Goal: "anything"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.Outcome != OutcomeUnknown || !strings.Contains(result.Summary, "the driver refused") {
		t.Fatalf("result = %+v", result)
	}
}

func TestRunExhaustsTheBudgetAndReportsTheFinalPage(t *testing.T) {
	driver := &fakeDriver{pages: []Observation{searchPage()}}
	chooser := &scriptedChooser{answers: []DecisionResponse{answer(ReobserveCandidateID, 0.9, 0.3)}}
	runner := Runner{Opener: fakeOpener{driver}, Chooser: chooser, Sleep: noSleep}

	result, errorValue := runner.Run(context.Background(), Request{Goal: "anything", MaxSteps: 3})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.Outcome != OutcomeBudgetExhausted || len(result.Steps) != 3 || len(chooser.requests) != 3 {
		t.Fatalf("result = %+v, decisions %d", result, len(chooser.requests))
	}
	if result.Page.URL != "https://example.test/" {
		t.Fatalf("page = %+v", result.Page)
	}
}

func TestCandidateTableStaysWithinTheDecisionLimit(t *testing.T) {
	refs := []Reference{}
	for index := 0; index < 50; index++ {
		refs = append(refs, Reference{Ref: fmt.Sprintf("r%d", index), Role: "link", Name: "Link", Actions: []string{"click"}, Visibility: "in_viewport"})
	}
	table := buildCandidates(Observation{Refs: refs}, nil)
	if len(table.candidates) != largestCandidateTable {
		t.Fatalf("%d candidates", len(table.candidates))
	}
	if _, isOffered := table.lookup(ReobserveCandidateID); !isOffered {
		t.Fatal("reobserve was dropped")
	}
	if _, isOffered := table.lookup(AbstainCandidateID); !isOffered {
		t.Fatal("abstain was dropped")
	}
}

func TestTruncateOutlineCutsOnALineBoundary(t *testing.T) {
	outline := strings.Repeat("- line of text\n", 100)
	truncated := truncateOutline(outline, 100)
	if len(truncated) > 100+len("\n… (outline truncated)") || !strings.HasSuffix(truncated, "(outline truncated)") {
		t.Fatalf("truncated = %q", truncated)
	}
	if strings.Contains(truncated, "- line of tex\n") {
		t.Fatal("a line was cut in the middle")
	}
}
