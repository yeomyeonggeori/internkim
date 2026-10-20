package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

const (
	personMessageDeleteToolName = "person_message_delete"
	admindPersonMessageWipePath = "/agent/api/buzz-admin-wipe"
	personHintField             = "personHint"
)

type personMessageDeleteInput struct {
	PersonHint string `json:"personHint"`
}

type personMessageWipe struct {
	Email string `json:"email"`
}

type personMessageWiped struct {
	Email             string `json:"email"`
	BuzzDeleted       int    `json:"buzzDeleted"`
	MattermostDeleted int    `json:"mattermostDeleted"`
}

type personMessageDeletion struct {
	Email        string `json:"email"`
	DeletedCount int    `json:"deletedCount"`
}

func (service Service) invokePersonMessageDeleteTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	person, refusal, isRefused := service.personNamedByTheCall(ctx, request)
	if isRefused {
		return refusal, nil
	}
	body, _ := json.Marshal(personMessageWipe{Email: person.Email})
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, admindRequesterURL(admindPersonMessageWipePath), bytes.NewReader(body))
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	return service.answerAdmindRequesterTool(request, httpRequest, countedPersonMessageDeletion)
}

func countedPersonMessageDeletion(answer json.RawMessage) (json.RawMessage, error) {
	var wiped personMessageWiped
	if errorValue := json.Unmarshal(answer, &wiped); errorValue != nil {
		return nil, errorValue
	}
	return json.Marshal(personMessageDeletion{Email: wiped.Email, DeletedCount: wiped.BuzzDeleted + wiped.MattermostDeleted})
}

func (service Service) resolvePersonMessageDeleteTarget(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	person, refusal, isRefused := service.personNamedByTheCall(ctx, request)
	if isRefused {
		return refusal, nil
	}
	return capabilityToolTargetResponse(request.ToolName, capabilities.ApprovalTarget{
		InputField: personHintField,
		ID:         exactRecipientIdentifier(person),
		Title:      recipientTitle(person),
	}), nil
}

func (service Service) personNamedByTheCall(ctx context.Context, request capabilities.ToolInvokeRequest) (directoryPerson, capabilities.ToolInvokeResponse, bool) {
	var input personMessageDeleteInput
	json.Unmarshal(request.Input, &input)
	return service.resolveMessageRecipient(ctx, request, input.PersonHint)
}
