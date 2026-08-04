package admind

import (
	"net/http"
	"strings"
)

func (service *Service) listCRMAccountsHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	actor, ok := service.crmHTTPActor(responseWriter, request)
	if !ok {
		return
	}
	includeArchived, ok := requireCRMIncludeArchived(responseWriter, request, actor)
	if !ok {
		return
	}
	accounts, errorValue := service.listCRMAccounts(request.Context(), includeArchived)
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	responses := make([]crmHTTPAccount, 0, len(accounts))
	for _, account := range accounts {
		responses = append(responses, crmHTTPAccountFromDomain(account))
	}
	writeCRMHTTPJSON(responseWriter, http.StatusOK, map[string]any{"accounts": responses})
}

func (service *Service) readCRMAccountHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	actor, ok := service.crmHTTPActor(responseWriter, request)
	if !ok {
		return
	}
	includeArchived, ok := requireCRMIncludeArchived(responseWriter, request, actor)
	if !ok {
		return
	}
	account, errorValue := service.readCRMAccount(request.Context(), request.PathValue("id"), includeArchived)
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	writeCRMHTTPJSON(responseWriter, http.StatusOK, map[string]any{"account": crmHTTPAccountFromDomain(account)})
}

func (service *Service) createCRMAccountHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	actor, ok := service.crmHTTPActor(responseWriter, request)
	if !ok || !requireCRMActorMutation(responseWriter, actor) {
		return
	}
	var payload crmHTTPAccountPayload
	if !decodeCRMHTTPJSON(responseWriter, request, &payload) {
		return
	}
	if errorValue := validateCRMHTTPAccount(payload); errorValue != nil {
		writeCRMHTTPError(responseWriter, http.StatusBadRequest, "invalid_request", errorValue.Error())
		return
	}
	if !service.requireCRMAssignment(responseWriter, request, actor, payload.OwnerPersonID, payload.OwnerCircleID) {
		return
	}
	account := crmAccountFromHTTP(payload, "", crmAuditFields{CreatedByPersonID: actor.PersonID, UpdatedByPersonID: actor.PersonID})
	written, errorValue := service.writeCRMAccount(request.Context(), account)
	if errorValue != nil {
		writeCRMHTTPMutationError(responseWriter, errorValue)
		return
	}
	writeCRMHTTPJSON(responseWriter, http.StatusCreated, map[string]any{"account": crmHTTPAccountFromDomain(written)})
}

func (service *Service) updateCRMAccountHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	actor, ok := service.crmHTTPActor(responseWriter, request)
	if !ok {
		return
	}
	existing, errorValue := service.readCRMAccount(request.Context(), request.PathValue("id"), false)
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	if !requireCRMOwner(responseWriter, actor, existing.OwnerPersonID, existing.OwnerCircleID) {
		return
	}
	var payload crmHTTPAccountPayload
	if !decodeCRMHTTPJSON(responseWriter, request, &payload) {
		return
	}
	if errorValue := validateCRMHTTPAccount(payload); errorValue != nil {
		writeCRMHTTPError(responseWriter, http.StatusBadRequest, "invalid_request", errorValue.Error())
		return
	}
	if !service.requireCRMAssignment(responseWriter, request, actor, payload.OwnerPersonID, payload.OwnerCircleID) {
		return
	}
	account := crmAccountFromHTTP(payload, existing.ID, existing.Audit)
	account.Audit.UpdatedAt = crmCurrentTimestamp()
	account.Audit.UpdatedByPersonID = actor.PersonID
	written, errorValue := service.writeCRMAccount(request.Context(), account)
	if errorValue != nil {
		writeCRMHTTPMutationError(responseWriter, errorValue)
		return
	}
	writeCRMHTTPJSON(responseWriter, http.StatusOK, map[string]any{"account": crmHTTPAccountFromDomain(written)})
}

func (service *Service) archiveCRMAccountHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	service.archiveCRMRecordHTTP(responseWriter, request, service.archiveCRMAccount)
}

func (service *Service) restoreCRMAccountHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	service.restoreCRMRecordHTTP(responseWriter, request, "account")
}

func crmAccountFromHTTP(payload crmHTTPAccountPayload, id string, audit crmAuditFields) crmAccount {
	status := strings.TrimSpace(payload.Status)
	if status == "" {
		status = "prospect"
	}
	importance := strings.TrimSpace(payload.Importance)
	if importance == "" {
		importance = "medium"
	}
	return crmAccount{
		ID: id, Name: strings.TrimSpace(payload.Name), Status: status,
		Types: payload.Types, Tags: payload.Tags, Importance: importance,
		OwnerPersonID: strings.TrimSpace(payload.OwnerPersonID), OwnerCircleID: strings.TrimSpace(payload.OwnerCircleID),
		Address: strings.TrimSpace(payload.Address), Description: strings.TrimSpace(payload.Description), Audit: audit,
	}
}

func crmHTTPAccountFromDomain(account crmAccount) crmHTTPAccount {
	return crmHTTPAccount{
		ID: account.ID, Name: account.Name, Status: account.Status, Types: account.Types, Tags: account.Tags,
		Importance: account.Importance, OwnerPersonID: account.OwnerPersonID, OwnerCircleID: account.OwnerCircleID,
		Address: account.Address, Description: account.Description, Audit: crmHTTPAuditFromDomain(account.Audit),
	}
}
