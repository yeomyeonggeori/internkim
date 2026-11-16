package capabilityd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

const (
	agentLearningAdmindPath        = "/agent-learning/api"
	soulAdmindPath                 = agentLearningAdmindPath + "/soul"
	soulHistoryAdmindPath          = soulAdmindPath + "/history"
	learnedSkillsAdmindPath        = agentLearningAdmindPath + "/skills?includeRetired=true"
	learningSettingsAdmindPath     = agentLearningAdmindPath + "/settings"
	learnedSkillActionAdmindPrefix = agentLearningAdmindPath + "/skills/"
)

type soulRevision struct {
	Version  int             `json:"version"`
	Document json.RawMessage `json:"document"`
}

type soulRevisionRequestBody struct {
	ExpectedVersion int             `json:"expectedVersion"`
	Document        json.RawMessage `json:"document"`
	Reason          string          `json:"reason"`
}

type soulPrinciplesUpdateInput map[string]json.RawMessage

type soulPrinciplesRevertInput struct {
	Version int    `json:"version"`
	Reason  string `json:"reason"`
}

type learnedSkillUpdateInput struct {
	ID        string `json:"id"`
	Protected *bool  `json:"protected"`
	Retired   *bool  `json:"retired"`
}

type learnedSkillAction struct {
	name string
	body map[string]any
}

func (service Service) invokePersonaPrinciplesGetTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	return service.getThroughAdmind(ctx, request, soulHistoryAdmindPath)
}

func (service Service) invokePersonaLearningGetTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	return service.getThroughAdmind(ctx, request, learnedSkillsAdmindPath)
}

func (service Service) getThroughAdmind(ctx context.Context, request capabilities.ToolInvokeRequest, path string) (capabilities.ToolInvokeResponse, error) {
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, admindRequesterURL(path), nil)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return service.answerAdmindRequesterTool(request, httpRequest, nil)
}

func (service Service) invokePersonaPrinciplesUpdateTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	var changes soulPrinciplesUpdateInput
	if errorValue := json.Unmarshal(request.Input, &changes); errorValue != nil {
		return personaLearningRefusal(request, http.StatusBadRequest, "the principles update input is not a JSON object"), nil
	}
	reason := stringFieldOf(changes, "reason")
	delete(changes, "reason")
	current, refusal, errorValue := service.currentSoulRevision(ctx, request)
	if errorValue != nil || refusal != nil {
		return personaLearningOutcome(refusal, errorValue)
	}
	patch, errorValue := json.Marshal(changes)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	document, errorValue := mergedPersonaDocument(current.Document, patch)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return service.reviseSoul(ctx, request, soulRevisionRequestBody{ExpectedVersion: current.Version, Document: document, Reason: reason})
}

func (service Service) invokePersonaPrinciplesRevertTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	var input soulPrinciplesRevertInput
	if errorValue := json.Unmarshal(request.Input, &input); errorValue != nil {
		return personaLearningRefusal(request, http.StatusBadRequest, "the principles revert input is not a JSON object"), nil
	}
	history, refusal, errorValue := service.soulHistory(ctx, request)
	if errorValue != nil || refusal != nil {
		return personaLearningOutcome(refusal, errorValue)
	}
	target, isKnown := soulRevisionNumbered(history, input.Version)
	if !isKnown {
		return personaLearningRefusal(request, http.StatusNotFound, fmt.Sprintf("working principles have no version %d; persona_principles_get lists the versions", input.Version)), nil
	}
	current := history[len(history)-1]
	return service.reviseSoul(ctx, request, soulRevisionRequestBody{ExpectedVersion: current.Version, Document: target.Document, Reason: input.Reason})
}

func (service Service) reviseSoul(ctx context.Context, request capabilities.ToolInvokeRequest, revision soulRevisionRequestBody) (capabilities.ToolInvokeResponse, error) {
	body, errorValue := json.Marshal(revision)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return service.answerThroughAdmindAsTheRequester(ctx, request, soulAdmindPath, body)
}

func (service Service) currentSoulRevision(ctx context.Context, request capabilities.ToolInvokeRequest) (soulRevision, *capabilities.ToolInvokeResponse, error) {
	answer, refusal, errorValue := service.readPersonaDocument(ctx, request, soulAdmindPath)
	if errorValue != nil || refusal != nil {
		return soulRevision{}, refusal, errorValue
	}
	var revision soulRevision
	if errorValue := json.Unmarshal(answer, &revision); errorValue != nil {
		return soulRevision{}, nil, fmt.Errorf("the working principles admind answered are not JSON: %w", errorValue)
	}
	return revision, nil, nil
}

func (service Service) soulHistory(ctx context.Context, request capabilities.ToolInvokeRequest) ([]soulRevision, *capabilities.ToolInvokeResponse, error) {
	answer, refusal, errorValue := service.readPersonaDocument(ctx, request, soulHistoryAdmindPath)
	if errorValue != nil || refusal != nil {
		return nil, refusal, errorValue
	}
	var history struct {
		History []soulRevision `json:"history"`
	}
	if errorValue := json.Unmarshal(answer, &history); errorValue != nil {
		return nil, nil, fmt.Errorf("the working principles history admind answered is not JSON: %w", errorValue)
	}
	if len(history.History) == 0 {
		return nil, nil, fmt.Errorf("admind answered an empty working principles history")
	}
	return history.History, nil, nil
}

func soulRevisionNumbered(history []soulRevision, version int) (soulRevision, bool) {
	for _, revision := range history {
		if revision.Version == version {
			return revision, true
		}
	}
	return soulRevision{}, false
}

func (service Service) invokePersonaLearningUpdateTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	current, refusal, errorValue := service.readPersonaDocument(ctx, request, learningSettingsAdmindPath)
	if errorValue != nil || refusal != nil {
		return personaLearningOutcome(refusal, errorValue)
	}
	settings, errorValue := mergedPersonaDocument(current, request.Input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return service.answerThroughAdmindAsTheRequester(ctx, request, learningSettingsAdmindPath, settings)
}

func (service Service) invokePersonaLearnedSkillUpdateTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	var input learnedSkillUpdateInput
	if errorValue := json.Unmarshal(request.Input, &input); errorValue != nil {
		return personaLearningRefusal(request, http.StatusBadRequest, "the learned skill update input is not a JSON object"), nil
	}
	actions := learnedSkillActionsFor(input)
	if input.ID == "" || len(actions) == 0 {
		return personaLearningRefusal(request, http.StatusBadRequest, "a learned skill update names the skill's id and at least one of protected or retired"), nil
	}
	var response capabilities.ToolInvokeResponse
	for _, action := range actions {
		body, errorValue := json.Marshal(action.body)
		if errorValue != nil {
			return capabilities.ToolInvokeResponse{}, errorValue
		}
		response, errorValue = service.answerThroughAdmindAsTheRequester(ctx, request, learnedSkillActionAdmindPrefix+action.name, body)
		if errorValue != nil || response.IsError {
			return response, errorValue
		}
	}
	return response, nil
}

func learnedSkillActionsFor(input learnedSkillUpdateInput) []learnedSkillAction {
	var actions []learnedSkillAction
	if input.Protected != nil {
		actions = append(actions, learnedSkillAction{name: "protect", body: map[string]any{"id": input.ID, "protected": *input.Protected}})
	}
	if input.Retired != nil {
		name := "restore"
		if *input.Retired {
			name = "retire"
		}
		actions = append(actions, learnedSkillAction{name: name, body: map[string]any{"id": input.ID}})
	}
	return actions
}

func personaLearningOutcome(refusal *capabilities.ToolInvokeResponse, errorValue error) (capabilities.ToolInvokeResponse, error) {
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return *refusal, nil
}

func personaLearningRefusal(request capabilities.ToolInvokeRequest, status int, message string) capabilities.ToolInvokeResponse {
	answer, _ := json.Marshal(map[string]string{"error": message, "errorCode": "invalid_input"})
	return capabilityToolFailure(admindToolOrigin, request.ToolName, status, answer)
}

func stringFieldOf(fields map[string]json.RawMessage, name string) string {
	var value string
	json.Unmarshal(fields[name], &value)
	return value
}
