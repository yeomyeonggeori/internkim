package computer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"gitlab.com/eastriver/internkim/internal/llmbackend"
)

const (
	nextActionQuestion  = "next_action"
	goalReachedQuestion = "goal_reached"
)

// DecisionRequest is what the companion asks the device to put to the decision
// model; the device holds the inference key, the companion never does.
type DecisionRequest struct {
	State     json.RawMessage                        `json:"state"`
	Questions map[string]llmbackend.DecisionQuestion `json:"questions"`
}

type DecisionResponse struct {
	Answers map[string]llmbackend.DecisionAnswer `json:"answers"`
}

type Chooser interface {
	Decide(ctx context.Context, request DecisionRequest) (DecisionResponse, error)
}

type decisionState struct {
	Goal        string          `json:"goal"`
	Step        int             `json:"step"`
	MaxSteps    int             `json:"max_steps"`
	InputNames  []string        `json:"available_inputs"`
	Observation observationView `json:"observation"`
	History     []Step          `json:"history"`
}

type observationView struct {
	Page    Page   `json:"page"`
	Outline string `json:"outline"`
}

type Decision struct {
	CandidateID string
	Confidence  float64
	GoalReached float64
}

func decisionStateDocument(goal string, step int, maxSteps int, inputs []Input, observation Observation, history []Step) (json.RawMessage, error) {
	names := make([]string, 0, len(inputs))
	for _, input := range inputs {
		names = append(names, input.Name)
	}
	return json.Marshal(decisionState{
		Goal:        goal,
		Step:        step,
		MaxSteps:    maxSteps,
		InputNames:  names,
		Observation: observationView{Page: observation.Page, Outline: observation.Outline},
		History:     history,
	})
}

func decisionQuestions(table candidateTable) map[string]llmbackend.DecisionQuestion {
	return map[string]llmbackend.DecisionQuestion{
		nextActionQuestion: llmbackend.ChoiceQuestion(
			"Select exactly one candidate ID: the single action that best moves the observed page toward the goal. Choose reobserve when the page may still be loading or changing. Choose abstain when the goal is already reached, cannot be reached from here, or no listed action is safe.",
			table.criteria(),
		),
		goalReachedQuestion: llmbackend.NoulQuestion(
			"How strongly does the observed page show that the goal is already reached? 1 means the page plainly shows it; 0 means it plainly does not or shows the goal cannot be reached.",
			nil,
		),
	}
}

func decisionOf(response DecisionResponse, table candidateTable) (Decision, error) {
	choice, isChoice := response.Answers[nextActionQuestion].AsChoice()
	if !isChoice {
		return Decision{}, errors.New("the decision model did not answer which action to take")
	}
	if _, isKnown := table.lookup(choice.Choice); !isKnown {
		return Decision{}, fmt.Errorf("the decision model chose %q, which is not a listed candidate", choice.Choice)
	}
	if !isUnitInterval(choice.Confidence) {
		return Decision{}, errors.New("the decision model returned an invalid confidence")
	}
	goalReached, isNoul := response.Answers[goalReachedQuestion].AsNoul()
	if !isNoul || !isUnitInterval(goalReached.Noul) {
		return Decision{}, errors.New("the decision model did not say whether the goal is reached")
	}
	return Decision{CandidateID: choice.Choice, Confidence: choice.Confidence, GoalReached: goalReached.Noul}, nil
}

func isUnitInterval(value float64) bool {
	return !math.IsNaN(value) && value >= 0 && value <= 1
}
