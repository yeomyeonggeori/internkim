package admind

import (
	"net/http"
	"strings"
)

func (service *Service) listCRMContactsHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	actor, ok := service.crmHTTPActor(responseWriter, request)
	if !ok {
		return
	}
	includeArchived, ok := requireCRMIncludeArchived(responseWriter, request, actor)
	if !ok {
		return
	}
	contacts, errorValue := service.listCRMContacts(request.Context(), includeArchived)
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	responses := make([]crmHTTPContact, 0, len(contacts))
	for _, contact := range contacts {
		responses = append(responses, crmHTTPContactFromDomain(contact))
	}
	writeCRMHTTPJSON(responseWriter, http.StatusOK, map[string]any{"contacts": responses})
}

func (service *Service) readCRMContactHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	actor, ok := service.crmHTTPActor(responseWriter, request)
	if !ok {
		return
	}
	includeArchived, ok := requireCRMIncludeArchived(responseWriter, request, actor)
	if !ok {
		return
	}
	contact, errorValue := service.readCRMContact(request.Context(), request.PathValue("id"), includeArchived)
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	writeCRMHTTPJSON(responseWriter, http.StatusOK, map[string]any{"contact": crmHTTPContactFromDomain(contact)})
}

func (service *Service) createCRMContactHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	actor, ok := service.crmHTTPActor(responseWriter, request)
	if !ok || !requireCRMActorMutation(responseWriter, actor) {
		return
	}
	var payload crmHTTPContactPayload
	if !decodeCRMHTTPJSON(responseWriter, request, &payload) {
		return
	}
	if errorValue := validateCRMHTTPContact(payload); errorValue != nil {
		writeCRMHTTPError(responseWriter, http.StatusBadRequest, "invalid_request", errorValue.Error())
		return
	}
	if !service.requireCRMAssignment(responseWriter, request, actor, payload.OwnerPersonID, payload.OwnerCircleID) {
		return
	}
	contact := crmContactFromHTTP(payload, "", crmAuditFields{CreatedByPersonID: actor.PersonID, UpdatedByPersonID: actor.PersonID})
	written, errorValue := service.writeCRMContact(request.Context(), contact)
	if errorValue != nil {
		writeCRMHTTPMutationError(responseWriter, errorValue)
		return
	}
	writeCRMHTTPJSON(responseWriter, http.StatusCreated, map[string]any{"contact": crmHTTPContactFromDomain(written)})
}

func (service *Service) updateCRMContactHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	actor, ok := service.crmHTTPActor(responseWriter, request)
	if !ok {
		return
	}
	existing, errorValue := service.readCRMContact(request.Context(), request.PathValue("id"), false)
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	if !requireCRMOwner(responseWriter, actor, existing.OwnerPersonID, existing.OwnerCircleID) {
		return
	}
	var payload crmHTTPContactPayload
	if !decodeCRMHTTPJSON(responseWriter, request, &payload) {
		return
	}
	if errorValue := validateCRMHTTPContact(payload); errorValue != nil {
		writeCRMHTTPError(responseWriter, http.StatusBadRequest, "invalid_request", errorValue.Error())
		return
	}
	if !service.requireCRMAssignment(responseWriter, request, actor, payload.OwnerPersonID, payload.OwnerCircleID) {
		return
	}
	contact := crmContactFromHTTP(payload, existing.ID, existing.Audit)
	contact.Audit.UpdatedAt = crmCurrentTimestamp()
	contact.Audit.UpdatedByPersonID = actor.PersonID
	written, errorValue := service.writeCRMContact(request.Context(), contact)
	if errorValue != nil {
		writeCRMHTTPMutationError(responseWriter, errorValue)
		return
	}
	writeCRMHTTPJSON(responseWriter, http.StatusOK, map[string]any{"contact": crmHTTPContactFromDomain(written)})
}

func (service *Service) archiveCRMContactHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	service.archiveCRMRecordHTTP(responseWriter, request, service.archiveCRMContact)
}

func (service *Service) restoreCRMContactHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	service.restoreCRMRecordHTTP(responseWriter, request, "contact")
}

func crmContactFromHTTP(payload crmHTTPContactPayload, id string, audit crmAuditFields) crmContact {
	return crmContact{
		ID: id, AccountID: strings.TrimSpace(payload.AccountID), Name: strings.TrimSpace(payload.Name),
		Email: strings.TrimSpace(payload.Email), Phone: strings.TrimSpace(payload.Phone), Title: strings.TrimSpace(payload.Title),
		Department: strings.TrimSpace(payload.Department), IsPrimary: payload.IsPrimary,
		OwnerPersonID: strings.TrimSpace(payload.OwnerPersonID), OwnerCircleID: strings.TrimSpace(payload.OwnerCircleID),
		Description: strings.TrimSpace(payload.Description), Audit: audit,
	}
}

func crmHTTPContactFromDomain(contact crmContact) crmHTTPContact {
	return crmHTTPContact{
		ID: contact.ID, AccountID: contact.AccountID, Name: contact.Name, Email: contact.Email, Phone: contact.Phone,
		Title: contact.Title, Department: contact.Department, IsPrimary: contact.IsPrimary,
		OwnerPersonID: contact.OwnerPersonID, OwnerCircleID: contact.OwnerCircleID,
		Description: contact.Description, Audit: crmHTTPAuditFromDomain(contact.Audit),
	}
}
