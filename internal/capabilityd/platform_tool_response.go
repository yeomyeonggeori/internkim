package capabilityd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type platformToolFailure struct {
	ErrorCode    string `json:"errorCode"`
	FailureStage string `json:"failureStage"`
	Message      string `json:"message"`
}

type platformToolPolicyDocument struct {
	People []platformToolPolicyPerson `json:"people"`
}

type platformToolPolicyPerson struct {
	PersonID string   `json:"personID"`
	Emails   []string `json:"emails"`
}

func platformToolSuccessResponse(toolName string, status string, result any) (capabilities.ToolInvokeResponse, error) {
	resultDocument, errorValue := json.Marshal(result)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilitySuccessResponse(toolName, status, resultDocument)
}

func platformToolDeniedResponse(toolName string, failure platformToolFailure) capabilities.ToolInvokeResponse {
	response := platformToolErrorResponse(toolName, failure)
	response.Outcome = capabilities.ToolOutcomeDenied
	response.Status = "denied"
	return response
}

func platformToolErrorResponse(toolName string, failure platformToolFailure) capabilities.ToolInvokeResponse {
	resultDocument, _ := json.Marshal(failure)
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        toolName,
		Outcome:         capabilities.ToolOutcomeFailed,
		Status:          "error",
		Content:         failure.Message,
		IsError:         true,
		Message:         failure.Message,
		ErrorCode:       failure.ErrorCode,
		FailureStage:    failure.FailureStage,
		Result:          resultDocument,
	}
}

func platformToolStaticFailure(errorCode string, failureStage string, message string) platformToolFailure {
	return platformToolFailure{
		ErrorCode:    strings.TrimSpace(errorCode),
		FailureStage: strings.TrimSpace(failureStage),
		Message:      strings.TrimSpace(message),
	}
}

func platformToolFailureForError(failureStage string, errorCode string, errorValue error) platformToolFailure {
	return platformToolStaticFailure(errorCode, failureStage, errorValue.Error())
}

func (service Service) requesterIsACompanyMember(ctx context.Context, toolContext capabilities.ToolInvokeContext) bool {
	policyDocument, errorValue := service.fetchPlatformToolPolicy(ctx)
	if errorValue != nil {
		return false
	}
	for _, person := range policyDocument.People {
		if platformToolPersonMatchesContext(person, toolContext) {
			return true
		}
	}
	return false
}

func (service Service) fetchPlatformToolPolicy(ctx context.Context) (platformToolPolicyDocument, error) {
	endpoint := strings.TrimRight(firstNonEmpty(service.Configuration.BlueclawBaseURL, DefaultConfiguration().BlueclawBaseURL), "/") + "/admin/api/policy"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if errorValue != nil {
		return platformToolPolicyDocument{}, errorValue
	}
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return platformToolPolicyDocument{}, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return platformToolPolicyDocument{}, fmt.Errorf("policy lookup failed with status %d", response.StatusCode)
	}
	var policyDocument platformToolPolicyDocument
	if errorValue := json.NewDecoder(response.Body).Decode(&policyDocument); errorValue != nil {
		return platformToolPolicyDocument{}, errorValue
	}
	return policyDocument, nil
}

func platformToolPersonMatchesContext(person platformToolPolicyPerson, toolContext capabilities.ToolInvokeContext) bool {
	if strings.TrimSpace(toolContext.RequesterPersonID) != "" && strings.TrimSpace(person.PersonID) == strings.TrimSpace(toolContext.RequesterPersonID) {
		return true
	}
	requesterEmail := normalizePlatformDMMatchValue(toolContext.RequesterEmail)
	if requesterEmail == "" {
		return false
	}
	for _, email := range person.Emails {
		if normalizePlatformDMMatchValue(email) == requesterEmail {
			return true
		}
	}
	return false
}
