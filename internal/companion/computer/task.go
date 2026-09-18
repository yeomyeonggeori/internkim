package computer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	OutcomeVerified        = "verified"
	OutcomeRefuted         = "refuted"
	OutcomeUnknown         = "unknown"
	OutcomeAbstained       = "abstained"
	OutcomeBudgetExhausted = "budget_exhausted"
)

const (
	DefaultMaxSteps     = 12
	DefaultProfileName  = "internkim"
	goalReachedAtLeast  = 0.85
	goalRefutedAtMost   = 0.05
	windowReadyAttempts = 60
	windowPollInterval  = 250 * time.Millisecond
	settleAfterAction   = 700 * time.Millisecond
	largestPageExcerpt  = 2000
)

type Request struct {
	Goal     string
	StartURL string
	Inputs   []Input
	MaxSteps int
}

type Step struct {
	Step        int     `json:"step"`
	CandidateID string  `json:"candidateID"`
	Description string  `json:"description"`
	Confidence  float64 `json:"confidence"`
}

type PageExcerpt struct {
	URL   string `json:"url"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

type Result struct {
	Outcome string      `json:"outcome"`
	Summary string      `json:"summary"`
	Steps   []Step      `json:"steps"`
	Page    PageExcerpt `json:"page"`
}

type Runner struct {
	Opener      DriverOpener
	Chooser     Chooser
	ProfileName string
	Sleep       func(context.Context, time.Duration) error
}

type binding struct {
	TargetID string
	TabID    string
}

func (runner Runner) Run(ctx context.Context, request Request) (Result, error) {
	if strings.TrimSpace(request.Goal) == "" {
		return Result{}, errors.New("a computer task needs a goal")
	}
	driver, errorValue := runner.Opener.Open(ctx, "internkim-"+time.Now().UTC().Format("20060102T150405"))
	if errorValue != nil {
		return Result{}, errorValue
	}
	defer driver.Close()
	bound, errorValue := runner.bindBrowser(ctx, driver)
	if errorValue != nil {
		return Result{}, errorValue
	}
	if errorValue := navigateToStart(ctx, driver, bound, request.StartURL); errorValue != nil {
		return Result{}, errorValue
	}
	return runner.loop(ctx, driver, bound, request)
}

func (runner Runner) bindBrowser(ctx context.Context, driver Driver) (binding, error) {
	prepared, errorValue := driver.Call(ctx, "browser_prepare", map[string]any{
		"allow_launch": true,
		"profile":      map[string]any{"mode": "isolated_named", "name": runner.profileName()},
	})
	if errorValue != nil {
		return binding{}, errorValue
	}
	var preparation struct {
		PreparedPID int64 `json:"prepared_pid"`
	}
	if errorValue := json.Unmarshal(prepared, &preparation); errorValue != nil || preparation.PreparedPID == 0 {
		return binding{}, errors.New("browser_prepare did not report the prepared browser process")
	}
	windowID, errorValue := runner.waitForWindow(ctx, driver, preparation.PreparedPID)
	if errorValue != nil {
		return binding{}, errorValue
	}
	return bindWindow(ctx, driver, preparation.PreparedPID, windowID)
}

func (runner Runner) profileName() string {
	if runner.ProfileName == "" {
		return DefaultProfileName
	}
	return runner.ProfileName
}

type nativeWindow struct {
	WindowID   int64 `json:"window_id"`
	IsOnScreen bool  `json:"is_on_screen"`
	Bounds     struct {
		Width  float64 `json:"width"`
		Height float64 `json:"height"`
	} `json:"bounds"`
}

func (runner Runner) waitForWindow(ctx context.Context, driver Driver, pid int64) (int64, error) {
	for attempt := 0; attempt < windowReadyAttempts; attempt++ {
		windowID, isFound, errorValue := largestVisibleWindow(ctx, driver, pid)
		if errorValue != nil {
			return 0, errorValue
		}
		if isFound {
			return windowID, nil
		}
		if errorValue := runner.sleep(ctx, windowPollInterval); errorValue != nil {
			return 0, errorValue
		}
	}
	return 0, errors.New("the companion browser window did not appear")
}

func largestVisibleWindow(ctx context.Context, driver Driver, pid int64) (int64, bool, error) {
	listed, errorValue := driver.Call(ctx, "list_windows", map[string]any{"pid": pid})
	if errorValue != nil {
		return 0, false, errorValue
	}
	var listing struct {
		Windows []nativeWindow `json:"windows"`
	}
	if errorValue := json.Unmarshal(listed, &listing); errorValue != nil {
		return 0, false, fmt.Errorf("list_windows answered unexpectedly: %w", errorValue)
	}
	var largest nativeWindow
	isFound := false
	for _, window := range listing.Windows {
		if !window.IsOnScreen {
			continue
		}
		if !isFound || window.Bounds.Width*window.Bounds.Height > largest.Bounds.Width*largest.Bounds.Height {
			largest, isFound = window, true
		}
	}
	return largest.WindowID, isFound, nil
}

func bindWindow(ctx context.Context, driver Driver, pid int64, windowID int64) (binding, error) {
	boundDocument, errorValue := driver.Call(ctx, "get_browser_state", map[string]any{"pid": pid, "window_id": windowID})
	if errorValue != nil {
		return binding{}, errorValue
	}
	var bound struct {
		Status          string `json:"status"`
		TargetID        string `json:"target_id"`
		BindingQuality  string `json:"binding_quality"`
		MutationAllowed bool   `json:"mutation_allowed"`
		Tabs            []struct {
			TabID  string `json:"tab_id"`
			Active bool   `json:"active"`
		} `json:"tabs"`
	}
	if errorValue := json.Unmarshal(boundDocument, &bound); errorValue != nil {
		return binding{}, fmt.Errorf("browser binding answered unexpectedly: %w", errorValue)
	}
	if bound.Status != "ok" || bound.BindingQuality != "exact" || !bound.MutationAllowed {
		return binding{}, errors.New("the companion browser window could not be bound exactly")
	}
	if len(bound.Tabs) == 0 {
		return binding{}, errors.New("the companion browser has no tab to work in")
	}
	tabID := bound.Tabs[0].TabID
	for _, tab := range bound.Tabs {
		if tab.Active {
			tabID = tab.TabID
		}
	}
	return binding{TargetID: bound.TargetID, TabID: tabID}, nil
}

func navigateToStart(ctx context.Context, driver Driver, bound binding, startURL string) error {
	if strings.TrimSpace(startURL) == "" {
		return nil
	}
	_, errorValue := driver.Call(ctx, "browser_navigate", bound.arguments(map[string]any{"url": startURL}))
	return errorValue
}

func (bound binding) arguments(extra map[string]any) map[string]any {
	arguments := map[string]any{"target_id": bound.TargetID, "tab_id": bound.TabID}
	for name, value := range extra {
		arguments[name] = value
	}
	return arguments
}

func (runner Runner) loop(ctx context.Context, driver Driver, bound binding, request Request) (Result, error) {
	maxSteps := request.MaxSteps
	if maxSteps <= 0 {
		maxSteps = DefaultMaxSteps
	}
	steps := []Step{}
	for step := 1; step <= maxSteps; step++ {
		observation, errorValue := observe(ctx, driver, bound)
		if errorValue != nil {
			return Result{}, errorValue
		}
		table := buildCandidates(observation, request.Inputs)
		decision, errorValue := runner.decide(ctx, request, step, maxSteps, observation, table, steps)
		if errorValue != nil {
			return Result{}, errorValue
		}
		if outcome, isFinished := finishedOutcome(decision); isFinished {
			return finished(outcome, steps, observation, decision), nil
		}
		candidate, _ := table.lookup(decision.CandidateID)
		steps = append(steps, Step{Step: step, CandidateID: candidate.ID, Description: candidate.Description, Confidence: decision.Confidence})
		if candidate.isReserved() {
			continue
		}
		if _, errorValue := driver.Call(ctx, candidate.Tool, bound.arguments(candidate.Arguments)); errorValue != nil {
			return finished(OutcomeUnknown, steps, observation, decision).withSummary("Step %d (%s) failed: %v. The page may or may not have changed.", step, candidate.Description, errorValue), nil
		}
		if errorValue := runner.sleep(ctx, settleAfterAction); errorValue != nil {
			return Result{}, errorValue
		}
	}
	observation, errorValue := observe(ctx, driver, bound)
	if errorValue != nil {
		return Result{}, errorValue
	}
	return finished(OutcomeBudgetExhausted, steps, observation, Decision{}), nil
}

func observe(ctx context.Context, driver Driver, bound binding) (Observation, error) {
	document, errorValue := driver.Call(ctx, "get_browser_state", bound.arguments(map[string]any{"snapshot_format": "semantic_v2"}))
	if errorValue != nil {
		return Observation{}, errorValue
	}
	return parseObservation(document)
}

func (runner Runner) decide(ctx context.Context, request Request, step int, maxSteps int, observation Observation, table candidateTable, history []Step) (Decision, error) {
	state, errorValue := decisionStateDocument(request.Goal, step, maxSteps, request.Inputs, observation, history)
	if errorValue != nil {
		return Decision{}, errorValue
	}
	response, errorValue := runner.Chooser.Decide(ctx, DecisionRequest{State: state, Questions: decisionQuestions(table)})
	if errorValue != nil {
		return Decision{}, fmt.Errorf("the decision model could not be asked: %w", errorValue)
	}
	return decisionOf(response, table)
}

func finishedOutcome(decision Decision) (string, bool) {
	if decision.GoalReached >= goalReachedAtLeast {
		return OutcomeVerified, true
	}
	if decision.CandidateID != AbstainCandidateID {
		return "", false
	}
	if decision.GoalReached <= goalRefutedAtMost {
		return OutcomeRefuted, true
	}
	return OutcomeAbstained, true
}

func finished(outcome string, steps []Step, observation Observation, decision Decision) Result {
	return Result{
		Outcome: outcome,
		Summary: summaryOf(outcome, len(steps), decision),
		Steps:   steps,
		Page:    excerptOf(observation),
	}
}

func (result Result) withSummary(format string, arguments ...any) Result {
	result.Summary = fmt.Sprintf(format, arguments...)
	return result
}

func summaryOf(outcome string, stepCount int, decision Decision) string {
	switch outcome {
	case OutcomeVerified:
		return fmt.Sprintf("The page showed the goal reached after %d step(s) (goal-reached score %.2f).", stepCount, decision.GoalReached)
	case OutcomeRefuted:
		return fmt.Sprintf("The decision model stopped after %d step(s): the page shows the goal cannot be reached from here (goal-reached score %.2f).", stepCount, decision.GoalReached)
	case OutcomeAbstained:
		return fmt.Sprintf("The decision model stopped after %d step(s) because no listed action was safe or useful (goal-reached score %.2f).", stepCount, decision.GoalReached)
	case OutcomeBudgetExhausted:
		return fmt.Sprintf("The step budget ran out after %d step(s) without the page showing the goal reached.", stepCount)
	}
	return ""
}

func excerptOf(observation Observation) PageExcerpt {
	return PageExcerpt{URL: observation.Page.URL, Title: observation.Page.Title, Text: shorten(observation.Outline, largestPageExcerpt)}
}

func (runner Runner) sleep(ctx context.Context, duration time.Duration) error {
	if runner.Sleep != nil {
		return runner.Sleep(ctx, duration)
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
