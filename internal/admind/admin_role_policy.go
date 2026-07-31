package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const operationsAdminInvalidRequestBody = "invalid request body"

func (service *Service) adminSessionRole(ctx context.Context, callerEmail string) string {
	if service.isFlowAdminEmail(ctx, callerEmail) {
		return adminUserRoleAdmin
	}
	return service.currentAdminUserRole(ctx, callerEmail)
}

func (service *Service) isOperationsAdminRequest(request *http.Request, path string) bool {
	if service.currentAdminUserRole(request.Context(), service.adminConsoleActorEmail(request)) != adminUserRoleOperationsAdmin {
		return false
	}
	return isOperationsAdminPath(request.Method, path)
}

func isOperationsAdminPath(method string, path string) bool {
	switch {
	case method == http.MethodGet && path == "/users":
		return true
	case method == http.MethodPost && path == "/users":
		return true
	case method == http.MethodPost && path == "/users/batch":
		return true
	case method == http.MethodPost && path == "/users/org-profiles":
		return true
	case method == http.MethodPut && path == "/org-groups":
		return true
	case method == http.MethodPost && path == "/circles":
		return true
	case method == http.MethodDelete && strings.HasPrefix(path, "/circles/"):
		return true
	case method == http.MethodPost && strings.HasPrefix(path, "/users/") && strings.HasSuffix(path, "/password-reset"):
		return true
	case method == http.MethodDelete && strings.HasPrefix(path, "/users/"):
		return true
	case method == http.MethodGet && path == "/workspace-settings":
		return true
	case method == http.MethodPut && path == "/workspace-settings":
		return true
	case method == http.MethodGet && path == "/holiday-countries":
		return true
	case method == http.MethodGet && path == "/attendance-locations":
		return true
	case method == http.MethodPut && path == "/attendance-locations":
		return true
	case (method == http.MethodGet || method == http.MethodPut) && path == "/attendance-leave-policy":
		return true
	default:
		return false
	}
}

func (service *Service) rejectOperationsAdminRestrictedMutation(responseWriter http.ResponseWriter, request *http.Request, path string) bool {
	if service.currentAdminUserRole(request.Context(), service.adminConsoleActorEmail(request)) != adminUserRoleOperationsAdmin {
		return false
	}

	forbiddenReason, errorValue := service.operationsAdminRestrictedMutationForbidden(request, path)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return true
	}
	if forbiddenReason == "" {
		return false
	}

	if forbiddenReason == operationsAdminInvalidRequestBody {
		http.Error(responseWriter, forbiddenReason, http.StatusBadRequest)
		return true
	}
	http.Error(responseWriter, forbiddenReason, http.StatusForbidden)
	return true
}

func (service *Service) operationsAdminRestrictedMutationForbidden(request *http.Request, path string) (string, error) {
	switch {
	case request.Method == http.MethodPost && (path == "/users" || path == "/users/batch"):
		body, errorValue := io.ReadAll(request.Body)
		if errorValue != nil {
			return operationsAdminInvalidRequestBody, nil
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
		return service.operationsAdminUserPayloadForbidden(request.Context(), path, body)
	case request.Method == http.MethodPost && strings.HasPrefix(path, "/users/") && strings.HasSuffix(path, "/password-reset"):
		return service.operationsAdminTargetForbidden(request.Context(), strings.TrimSuffix(strings.TrimPrefix(path, "/users/"), "/password-reset"))
	case request.Method == http.MethodDelete && strings.HasPrefix(path, "/users/"):
		return service.operationsAdminTargetForbidden(request.Context(), strings.TrimPrefix(path, "/users/"))
	case request.Method == http.MethodPost && path == "/circles":
		body, errorValue := io.ReadAll(request.Body)
		if errorValue != nil {
			return operationsAdminInvalidRequestBody, nil
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
		return operationsAdminCirclePayloadForbidden(body), nil
	case request.Method == http.MethodDelete && strings.HasPrefix(path, "/circles/"):
		return operationsAdminCircleTargetForbidden(strings.TrimPrefix(path, "/circles/")), nil
	default:
		return "", nil
	}
}

func (service *Service) operationsAdminUserPayloadForbidden(ctx context.Context, path string, body []byte) (string, error) {
	payloads, isValidPayload := operationsAdminUserPayloads(path, body)
	if !isValidPayload {
		return "", nil
	}
	for _, payload := range payloads {
		if normalizeAdminUserRole(payload.Role) == adminUserRoleAdmin {
			return "operations admin cannot grant admin role", nil
		}
	}

	records, errorValue := service.currentUserRecords(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	for _, payload := range payloads {
		if hasAdminUserRecord(records, payload.Email) {
			return "operations admin cannot manage admin users", nil
		}
	}
	return "", nil
}

func operationsAdminUserPayloads(path string, body []byte) ([]adminUserMutation, bool) {
	if path == "/users" {
		var payload adminUserMutation
		if json.Unmarshal(body, &payload) != nil {
			return nil, false
		}
		return []adminUserMutation{payload}, true
	}

	var payload struct {
		Users []adminUserMutation `json:"users"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return nil, false
	}
	return payload.Users, true
}

func operationsAdminCirclePayloadForbidden(body []byte) string {
	var payload adminCircleRecord
	if json.Unmarshal(body, &payload) != nil {
		return ""
	}
	if isReservedAdminCircleID(payload.CircleID) {
		return "operations admin cannot manage reserved groups"
	}
	return ""
}

func operationsAdminCircleTargetForbidden(encodedCircleID string) string {
	circleID, errorValue := url.PathUnescape(strings.Trim(encodedCircleID, "/"))
	if errorValue != nil {
		return "invalid group"
	}
	if isReservedAdminCircleID(circleID) {
		return "operations admin cannot manage reserved groups"
	}
	return ""
}

func (service *Service) operationsAdminTargetForbidden(ctx context.Context, encodedEmail string) (string, error) {
	email, errorValue := url.PathUnescape(strings.Trim(encodedEmail, "/"))
	if errorValue != nil {
		return "invalid email", nil
	}
	records, errorValue := service.currentUserRecords(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	if hasAdminUserRecord(records, email) {
		return "operations admin cannot manage admin users", nil
	}
	return "", nil
}

func hasAdminUserRecord(records []adminUserMutation, email string) bool {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	for _, record := range records {
		if normalizeAdminUserRole(record.Role) == adminUserRoleAdmin && strings.EqualFold(record.Email, normalizedEmail) {
			return true
		}
	}
	return false
}

func isReservedAdminCircleID(circleID string) bool {
	switch strings.ToLower(strings.TrimSpace(circleID)) {
	case "staff", "admin":
		return true
	default:
		return false
	}
}
