package admind

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	sqlite "modernc.org/sqlite"
)

const crmHTTPMaxRequestBytes = 1 << 20
const sqliteConstraintPrimaryCode = 19

func (service *Service) crmHTTPActor(responseWriter http.ResponseWriter, request *http.Request) (crmActor, bool) {
	actor, errorValue := service.resolveCRMActor(request)
	if errorValue == nil {
		return actor, true
	}
	switch {
	case errors.Is(errorValue, errCRMAuthenticationRequired):
		writeCRMHTTPError(responseWriter, http.StatusUnauthorized, "authentication_required", "authentication required")
	case errors.Is(errorValue, errCRMPermissionDenied):
		writeCRMHTTPError(responseWriter, http.StatusForbidden, "permission_denied", "CRM access required")
	default:
		writeCRMHTTPError(responseWriter, http.StatusInternalServerError, "internal_error", "CRM identity lookup failed")
	}
	return crmActor{}, false
}

func requireCRMActorMutation(responseWriter http.ResponseWriter, actor crmActor) bool {
	if errorValue := actor.requireMutationIdentity(); errorValue != nil {
		writeCRMHTTPError(responseWriter, http.StatusForbidden, "actor_identity_missing", "organization identity required")
		return false
	}
	return true
}

func requireCRMAdmin(responseWriter http.ResponseWriter, actor crmActor) bool {
	if !actor.IsAdmin {
		writeCRMHTTPError(responseWriter, http.StatusForbidden, "admin_required", "administrator access required")
		return false
	}
	return requireCRMActorMutation(responseWriter, actor)
}

func requireCRMOwner(responseWriter http.ResponseWriter, actor crmActor, ownerPersonID string, ownerCircleID string) bool {
	if !requireCRMActorMutation(responseWriter, actor) {
		return false
	}
	if !actor.canEdit(ownerPersonID, ownerCircleID) {
		writeCRMHTTPError(responseWriter, http.StatusForbidden, "permission_denied", "record owner or owner team access required")
		return false
	}
	return true
}

func (service *Service) requireCRMAssignment(responseWriter http.ResponseWriter, request *http.Request, actor crmActor, ownerPersonID string, ownerCircleID string) bool {
	if !requireCRMActorMutation(responseWriter, actor) {
		return false
	}
	errorValue := service.validateCRMAssignment(request, actor, ownerPersonID, ownerCircleID)
	switch {
	case errorValue == nil:
		return true
	case errors.Is(errorValue, errCRMInvalidOwnerAssignment):
		writeCRMHTTPError(responseWriter, http.StatusBadRequest, "invalid_owner", "ownerPersonID and ownerCircleID must match the organization directory")
	default:
		writeCRMHTTPError(responseWriter, http.StatusForbidden, "permission_denied", "owner assignment requires the owner or a shared owner team")
	}
	return false
}

func decodeCRMHTTPJSON(responseWriter http.ResponseWriter, request *http.Request, destination any) bool {
	request.Body = http.MaxBytesReader(responseWriter, request.Body, crmHTTPMaxRequestBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if errorValue := decoder.Decode(destination); errorValue != nil {
		writeCRMHTTPError(responseWriter, http.StatusBadRequest, "invalid_request", "invalid JSON request body")
		return false
	}
	if errorValue := decoder.Decode(&struct{}{}); errorValue != io.EOF {
		writeCRMHTTPError(responseWriter, http.StatusBadRequest, "invalid_request", "request body must contain one JSON document")
		return false
	}
	return true
}

func crmHTTPIncludeArchived(request *http.Request) (bool, error) {
	return crmHTTPBooleanQuery(request, "includeArchived", false)
}

func crmHTTPBooleanQuery(request *http.Request, name string, defaultValue bool) (bool, error) {
	value := request.URL.Query().Get(name)
	if value == "" {
		return defaultValue, nil
	}
	return strconv.ParseBool(value)
}

func requireCRMIncludeArchived(responseWriter http.ResponseWriter, request *http.Request, actor crmActor) (bool, bool) {
	includeArchived, errorValue := crmHTTPIncludeArchived(request)
	if errorValue != nil {
		writeCRMHTTPError(responseWriter, http.StatusBadRequest, "invalid_request", "includeArchived must be true or false")
		return false, false
	}
	if includeArchived && !actor.IsAdmin {
		writeCRMHTTPError(responseWriter, http.StatusForbidden, "admin_required", "administrator access required for archived records")
		return false, false
	}
	return includeArchived, true
}

func writeCRMHTTPJSON(responseWriter http.ResponseWriter, status int, document any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.Header().Set("Cache-Control", "private, no-store")
	responseWriter.WriteHeader(status)
	_ = json.NewEncoder(responseWriter).Encode(document)
}

func writeCRMHTTPError(responseWriter http.ResponseWriter, status int, code string, message string) {
	writeCRMHTTPJSON(responseWriter, status, crmHTTPErrorDocument{Error: crmHTTPError{Code: code, Message: message}})
}

func writeCRMHTTPReadError(responseWriter http.ResponseWriter, errorValue error) {
	if errors.Is(errorValue, errCRMRecordNotFound) {
		writeCRMHTTPError(responseWriter, http.StatusNotFound, "not_found", "CRM record not found")
		return
	}
	writeCRMHTTPError(responseWriter, http.StatusInternalServerError, "internal_error", "CRM data could not be read")
}

func writeCRMHTTPMutationError(responseWriter http.ResponseWriter, errorValue error) {
	switch {
	case errors.Is(errorValue, errCRMRecordNotFound):
		writeCRMHTTPError(responseWriter, http.StatusNotFound, "not_found", "CRM record not found")
	case errors.Is(errorValue, errCRMPermissionDenied):
		writeCRMHTTPError(responseWriter, http.StatusForbidden, "permission_denied", "record owner or owner team access required")
	case errors.Is(errorValue, errCRMConflict), crmHTTPIsConstraintError(errorValue):
		writeCRMHTTPError(responseWriter, http.StatusConflict, "conflict", "CRM record conflicts with current data")
	default:
		writeCRMHTTPError(responseWriter, http.StatusInternalServerError, "internal_error", "CRM data could not be changed")
	}
}

func crmHTTPIsConstraintError(errorValue error) bool {
	var sqliteError *sqlite.Error
	return errors.As(errorValue, &sqliteError) && sqliteError.Code()&0xff == sqliteConstraintPrimaryCode
}
