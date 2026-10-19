package capabilityd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

const (
	personaIdentityAdmindPath = "/persona/api/identity"
	personaProfileAdmindPath  = "/persona/api/user"
)

var personaAdmindPathByToolName = map[string]string{
	"persona_identity_get":    personaIdentityAdmindPath,
	"persona_identity_update": personaIdentityAdmindPath,
	"persona_profile_get":     personaProfileAdmindPath,
	"persona_profile_update":  personaProfileAdmindPath,
}

func personaAdmindPath(request capabilities.ToolInvokeRequest) string {
	return personaAdmindPathByToolName[strings.TrimSpace(request.ToolName)]
}

func (service Service) invokePersonaGetTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, admindRequesterURL(personaAdmindPath(request)), nil)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return service.answerAdmindRequesterTool(request, httpRequest, nil)
}

func (service Service) invokePersonaUpdateTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	path := personaAdmindPath(request)
	current, refusal, errorValue := service.readPersonaDocument(ctx, request, path)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if refusal != nil {
		return *refusal, nil
	}
	updated, errorValue := mergedPersonaDocument(current, request.Input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return service.answerThroughAdmindAsTheRequester(ctx, request, path, updated)
}

func (service Service) readPersonaDocument(ctx context.Context, request capabilities.ToolInvokeRequest, path string) (json.RawMessage, *capabilities.ToolInvokeResponse, error) {
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, admindRequesterURL(path), nil)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	response, errorValue := service.askAdmindAsTheRequester(httpRequest, request.Context.RequesterEmail)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	defer response.Body.Close()
	document, errorValue := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if errorValue != nil {
		return nil, nil, errorValue
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		refusal := capabilityToolFailure(admindToolOrigin, request.ToolName, response.StatusCode, json.RawMessage(document))
		return nil, &refusal, nil
	}
	return document, nil, nil
}

func mergedPersonaDocument(current json.RawMessage, patch json.RawMessage) ([]byte, error) {
	var document, changes any
	if errorValue := json.Unmarshal(current, &document); errorValue != nil {
		return nil, fmt.Errorf("the persona document admind answered is not JSON: %w", errorValue)
	}
	if errorValue := json.Unmarshal(patch, &changes); errorValue != nil {
		return nil, fmt.Errorf("the persona update input is not JSON: %w", errorValue)
	}
	return json.Marshal(mergePatch(document, changes))
}

// RFC 7396, JSON Merge Patch, section 2.
func mergePatch(target any, patch any) any {
	patchObject, isPatchObject := patch.(map[string]any)
	if !isPatchObject {
		return patch
	}
	merged := map[string]any{}
	if targetObject, isTargetObject := target.(map[string]any); isTargetObject {
		for name, value := range targetObject {
			merged[name] = value
		}
	}
	for name, value := range patchObject {
		if value == nil {
			delete(merged, name)
			continue
		}
		merged[name] = mergePatch(merged[name], value)
	}
	return merged
}
