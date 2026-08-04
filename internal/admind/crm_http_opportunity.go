package admind

import (
	"fmt"
	"math"
	"net/http"
	"strings"
)

func (service *Service) listCRMOpportunitiesHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	actor, ok := service.crmHTTPActor(responseWriter, request)
	if !ok {
		return
	}
	includeArchived, ok := requireCRMIncludeArchived(responseWriter, request, actor)
	if !ok {
		return
	}
	opportunities, errorValue := service.listCRMOpportunities(request.Context(), includeArchived)
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	responses := make([]crmHTTPOpportunity, 0, len(opportunities))
	for _, opportunity := range opportunities {
		responses = append(responses, crmHTTPOpportunityFromDomain(opportunity, nil))
	}
	writeCRMHTTPJSON(responseWriter, http.StatusOK, map[string]any{"opportunities": responses})
}

func (service *Service) readCRMOpportunityHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	actor, ok := service.crmHTTPActor(responseWriter, request)
	if !ok {
		return
	}
	includeArchived, ok := requireCRMIncludeArchived(responseWriter, request, actor)
	if !ok {
		return
	}
	opportunity, contacts, errorValue := service.readCRMOpportunity(request.Context(), request.PathValue("id"), includeArchived)
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	writeCRMHTTPJSON(responseWriter, http.StatusOK, map[string]any{"opportunity": crmHTTPOpportunityFromDomain(opportunity, contacts)})
}

func (service *Service) createCRMOpportunityHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	actor, ok := service.crmHTTPActor(responseWriter, request)
	if !ok || !requireCRMActorMutation(responseWriter, actor) {
		return
	}
	var payload crmHTTPOpportunityPayload
	if !decodeCRMHTTPJSON(responseWriter, request, &payload) {
		return
	}
	if errorValue := validateCRMHTTPOpportunity(payload); errorValue != nil {
		writeCRMHTTPError(responseWriter, http.StatusBadRequest, "invalid_request", errorValue.Error())
		return
	}
	if !service.requireCRMAssignment(responseWriter, request, actor, payload.OwnerPersonID, payload.OwnerCircleID) {
		return
	}
	opportunity := crmOpportunityFromHTTP(payload, crmOpportunity{Audit: crmAuditFields{
		CreatedByPersonID: actor.PersonID,
		UpdatedByPersonID: actor.PersonID,
	}})
	written, errorValue := service.writeCRMOpportunity(request.Context(), opportunity, crmOpportunityContactsFromHTTP(payload.Contacts))
	if errorValue != nil {
		writeCRMHTTPMutationError(responseWriter, errorValue)
		return
	}
	_, contacts, errorValue := service.readCRMOpportunity(request.Context(), written.ID, false)
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	writeCRMHTTPJSON(responseWriter, http.StatusCreated, map[string]any{"opportunity": crmHTTPOpportunityFromDomain(written, contacts)})
}

func (service *Service) updateCRMOpportunityHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	actor, ok := service.crmHTTPActor(responseWriter, request)
	if !ok {
		return
	}
	existing, _, errorValue := service.readCRMOpportunity(request.Context(), request.PathValue("id"), false)
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	if !requireCRMOwner(responseWriter, actor, existing.OwnerPersonID, existing.OwnerCircleID) {
		return
	}
	var payload crmHTTPOpportunityPayload
	if !decodeCRMHTTPJSON(responseWriter, request, &payload) {
		return
	}
	if errorValue := validateCRMHTTPOpportunity(payload); errorValue != nil {
		writeCRMHTTPError(responseWriter, http.StatusBadRequest, "invalid_request", errorValue.Error())
		return
	}
	if !service.requireCRMAssignment(responseWriter, request, actor, payload.OwnerPersonID, payload.OwnerCircleID) {
		return
	}
	existing.Audit.UpdatedAt = crmCurrentTimestamp()
	existing.Audit.UpdatedByPersonID = actor.PersonID
	opportunity := crmOpportunityFromHTTP(payload, existing)
	written, errorValue := service.writeCRMOpportunity(request.Context(), opportunity, crmOpportunityContactsFromHTTP(payload.Contacts))
	if errorValue != nil {
		writeCRMHTTPMutationError(responseWriter, errorValue)
		return
	}
	_, contacts, errorValue := service.readCRMOpportunity(request.Context(), written.ID, false)
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	writeCRMHTTPJSON(responseWriter, http.StatusOK, map[string]any{"opportunity": crmHTTPOpportunityFromDomain(written, contacts)})
}

func (service *Service) archiveCRMOpportunityHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	service.archiveCRMRecordHTTP(responseWriter, request, service.archiveCRMOpportunity)
}

func (service *Service) restoreCRMOpportunityHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	service.restoreCRMRecordHTTP(responseWriter, request, "opportunity")
}

func (service *Service) transitionCRMOpportunityHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	actor, ok := service.crmHTTPActor(responseWriter, request)
	if !ok {
		return
	}
	opportunity, _, errorValue := service.readCRMOpportunity(request.Context(), request.PathValue("id"), false)
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	if !requireCRMOwner(responseWriter, actor, opportunity.OwnerPersonID, opportunity.OwnerCircleID) {
		return
	}
	var payload crmHTTPTransitionPayload
	if !decodeCRMHTTPJSON(responseWriter, request, &payload) {
		return
	}
	if errorValue := validateCRMHTTPTransition(payload); errorValue != nil {
		writeCRMHTTPError(responseWriter, http.StatusBadRequest, "invalid_request", errorValue.Error())
		return
	}
	errorValue = service.transitionCRMOpportunityStage(request.Context(), crmOpportunityStageTransition{
		OpportunityID:       opportunity.ID,
		Stage:               payload.Stage,
		StagePosition:       payload.StagePosition,
		BeforeOpportunityID: payload.BeforeOpportunityID,
		OccurredAt:          payload.OccurredAt,
		ActorPersonID:       actor.PersonID,
		LostReason:          payload.LostReason,
		BaseAmountMinor:     payload.BaseAmountMinor,
		BaseCurrencyCode:    payload.BaseCurrencyCode,
	})
	if errorValue != nil {
		writeCRMHTTPMutationError(responseWriter, errorValue)
		return
	}
	written, contacts, errorValue := service.readCRMOpportunity(request.Context(), opportunity.ID, false)
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	writeCRMHTTPJSON(responseWriter, http.StatusOK, map[string]any{"opportunity": crmHTTPOpportunityFromDomain(written, contacts)})
}

func (service *Service) positionCRMOpportunityHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	actor, ok := service.crmHTTPActor(responseWriter, request)
	if !ok {
		return
	}
	opportunity, _, errorValue := service.readCRMOpportunity(request.Context(), request.PathValue("id"), false)
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	if !requireCRMOwner(responseWriter, actor, opportunity.OwnerPersonID, opportunity.OwnerCircleID) {
		return
	}
	var payload crmHTTPPositionPayload
	if !decodeCRMHTTPJSON(responseWriter, request, &payload) {
		return
	}
	if errorValue := validateCRMHTTPPosition(payload); errorValue != nil {
		writeCRMHTTPError(responseWriter, http.StatusBadRequest, "invalid_request", errorValue.Error())
		return
	}
	errorValue = service.setCRMOpportunityStagePosition(
		request.Context(), opportunity.ID, payload.Position, payload.BeforeOpportunityID, payload.UpdatedAt, actor.PersonID,
	)
	if errorValue != nil {
		writeCRMHTTPMutationError(responseWriter, errorValue)
		return
	}
	written, contacts, errorValue := service.readCRMOpportunity(request.Context(), opportunity.ID, false)
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	writeCRMHTTPJSON(responseWriter, http.StatusOK, map[string]any{"opportunity": crmHTTPOpportunityFromDomain(written, contacts)})
}

func validateCRMHTTPOpportunity(payload crmHTTPOpportunityPayload) error {
	if strings.TrimSpace(payload.Name) == "" || strings.TrimSpace(payload.Pipeline) == "" || strings.TrimSpace(payload.OwnerPersonID) == "" {
		return fmt.Errorf("name, pipeline, and ownerPersonID are required")
	}
	if strings.TrimSpace(payload.AccountID) == "" && len(payload.Contacts) == 0 {
		return fmt.Errorf("accountID or contacts are required")
	}
	if payload.CurrencyCode != "" && !crmValueAllowed(payload.CurrencyCode, "KRW", "USD", "JPY", "EUR") {
		return fmt.Errorf("invalid currencyCode")
	}
	if payload.Importance != "" && !crmValueAllowed(payload.Importance, "high", "medium", "low") {
		return fmt.Errorf("invalid importance")
	}
	if errorValue := crmValidateDueTime(payload.DueAt, payload.DueTimeZone); errorValue != nil {
		return errorValue
	}
	return validateCRMOpportunityContacts(crmOpportunityContactsFromHTTP(payload.Contacts))
}

func validateCRMHTTPTransition(payload crmHTTPTransitionPayload) error {
	if strings.TrimSpace(payload.Stage) == "" {
		return fmt.Errorf("stage is required")
	}
	if math.IsNaN(payload.StagePosition) || math.IsInf(payload.StagePosition, 0) {
		return fmt.Errorf("invalid stagePosition")
	}
	if errorValue := crmValidateTimestamp(payload.OccurredAt); errorValue != nil {
		return errorValue
	}
	if payload.BaseCurrencyCode != "" && !crmValueAllowed(payload.BaseCurrencyCode, "KRW", "USD", "JPY", "EUR") {
		return fmt.Errorf("invalid baseCurrencyCode")
	}
	return nil
}

func validateCRMHTTPPosition(payload crmHTTPPositionPayload) error {
	if math.IsNaN(payload.Position) || math.IsInf(payload.Position, 0) {
		return fmt.Errorf("invalid position")
	}
	return crmValidateTimestamp(payload.UpdatedAt)
}

func crmOpportunityFromHTTP(payload crmHTTPOpportunityPayload, opportunity crmOpportunity) crmOpportunity {
	opportunity.AccountID = strings.TrimSpace(payload.AccountID)
	opportunity.Business = strings.TrimSpace(payload.Business)
	opportunity.Name = strings.TrimSpace(payload.Name)
	opportunity.Pipeline = strings.TrimSpace(payload.Pipeline)
	opportunity.OwnerPersonID = strings.TrimSpace(payload.OwnerPersonID)
	opportunity.OwnerCircleID = strings.TrimSpace(payload.OwnerCircleID)
	opportunity.AmountMinor = payload.AmountMinor
	opportunity.CurrencyCode = strings.TrimSpace(payload.CurrencyCode)
	opportunity.Importance = strings.TrimSpace(payload.Importance)
	opportunity.DueAt = strings.TrimSpace(payload.DueAt)
	opportunity.DueTimeZone = strings.TrimSpace(payload.DueTimeZone)
	opportunity.Description = strings.TrimSpace(payload.Description)
	return opportunity
}

func crmOpportunityContactsFromHTTP(contacts []crmHTTPOpportunityContact) []crmOpportunityContact {
	if contacts == nil {
		return nil
	}
	result := make([]crmOpportunityContact, 0, len(contacts))
	for _, contact := range contacts {
		result = append(result, crmOpportunityContact{ContactID: strings.TrimSpace(contact.ContactID), IsPrimary: contact.IsPrimary})
	}
	return result
}

func crmHTTPOpportunityFromDomain(opportunity crmOpportunity, contacts []crmOpportunityContact) crmHTTPOpportunity {
	responseContacts := make([]crmHTTPOpportunityContact, 0, len(contacts))
	for _, contact := range contacts {
		responseContacts = append(responseContacts, crmHTTPOpportunityContact{ContactID: contact.ContactID, IsPrimary: contact.IsPrimary})
	}
	return crmHTTPOpportunity{
		ID: opportunity.ID, AccountID: opportunity.AccountID, Business: opportunity.Business, Name: opportunity.Name,
		Pipeline: opportunity.Pipeline, Stage: opportunity.Stage, StagePosition: opportunity.StagePosition,
		StageChangedAt: opportunity.StageChangedAt, OwnerPersonID: opportunity.OwnerPersonID, OwnerCircleID: opportunity.OwnerCircleID,
		AmountMinor: opportunity.AmountMinor, CurrencyCode: opportunity.CurrencyCode, BaseAmountMinor: opportunity.BaseAmountMinor,
		BaseCurrencyCode: opportunity.BaseCurrencyCode, Importance: opportunity.Importance, DueAt: opportunity.DueAt,
		DueTimeZone: opportunity.DueTimeZone, LostReason: opportunity.LostReason, Description: opportunity.Description,
		Contacts: responseContacts, Audit: crmHTTPAuditFromDomain(opportunity.Audit),
	}
}
