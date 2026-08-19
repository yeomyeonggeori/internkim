package admind

import (
	"net/http"
	"strings"
)

func (service *Service) listCRMActivitiesHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	actor, ok := service.crmHTTPActor(responseWriter, request)
	if !ok {
		return
	}
	includeArchived, ok := requireCRMIncludeArchived(responseWriter, request, actor)
	if !ok {
		return
	}
	activities, errorValue := service.listCRMActivities(request.Context(), includeArchived)
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	responses := make([]crmHTTPActivity, 0, len(activities))
	for _, activity := range activities {
		responses = append(responses, crmHTTPActivityFromDomain(activity))
	}
	writeCRMHTTPJSON(responseWriter, http.StatusOK, map[string]any{"activities": responses})
}

func (service *Service) readCRMActivityHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	actor, ok := service.crmHTTPActor(responseWriter, request)
	if !ok {
		return
	}
	includeArchived, ok := requireCRMIncludeArchived(responseWriter, request, actor)
	if !ok {
		return
	}
	activity, errorValue := service.readCRMActivity(request.Context(), request.PathValue("id"), includeArchived)
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	writeCRMHTTPJSON(responseWriter, http.StatusOK, map[string]any{"activity": crmHTTPActivityFromDomain(activity)})
}

func (service *Service) createCRMActivityHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	actor, ok := service.crmHTTPActor(responseWriter, request)
	if !ok || !requireCRMActorMutation(responseWriter, actor) {
		return
	}
	var payload crmHTTPActivityPayload
	if !decodeCRMHTTPJSON(responseWriter, request, &payload) {
		return
	}
	if errorValue := validateCRMHTTPActivity(payload, ""); errorValue != nil {
		writeCRMHTTPError(responseWriter, http.StatusBadRequest, "invalid_request", errorValue.Error())
		return
	}
	activity := crmActivityFromHTTP(payload, "", crmAuditFields{CreatedByPersonID: actor.PersonID, UpdatedByPersonID: actor.PersonID})
	activity, errorValue := service.resolveCRMActivityReferences(request.Context(), activity)
	if errorValue != nil {
		writeCRMHTTPMutationError(responseWriter, errorValue)
		return
	}
	if errorValue := service.authorizeCRMActivity(request.Context(), actor, activity); errorValue != nil {
		writeCRMHTTPMutationError(responseWriter, errorValue)
		return
	}
	written, errorValue := service.writeCRMActivity(request.Context(), activity)
	if errorValue != nil {
		writeCRMHTTPMutationError(responseWriter, errorValue)
		return
	}
	writeCRMHTTPJSON(responseWriter, http.StatusCreated, map[string]any{"activity": crmHTTPActivityFromDomain(written)})
}

func (service *Service) updateCRMActivityHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	actor, ok := service.crmHTTPActor(responseWriter, request)
	if !ok || !requireCRMActorMutation(responseWriter, actor) {
		return
	}
	existing, errorValue := service.readCRMActivity(request.Context(), request.PathValue("id"), false)
	if errorValue != nil {
		writeCRMHTTPReadError(responseWriter, errorValue)
		return
	}
	if errorValue := service.authorizeCRMActivity(request.Context(), actor, existing); errorValue != nil {
		writeCRMHTTPMutationError(responseWriter, errorValue)
		return
	}
	var payload crmHTTPActivityPayload
	if !decodeCRMHTTPJSON(responseWriter, request, &payload) {
		return
	}
	if errorValue := validateCRMHTTPActivity(payload, existing.Kind); errorValue != nil {
		writeCRMHTTPError(responseWriter, http.StatusBadRequest, "invalid_request", errorValue.Error())
		return
	}
	activity := crmActivityFromHTTP(payload, existing.ID, existing.Audit)
	activity.Audit.UpdatedAt = crmCurrentTimestamp()
	activity.Audit.UpdatedByPersonID = actor.PersonID
	activity, errorValue = service.resolveCRMActivityReferences(request.Context(), activity)
	if errorValue != nil {
		writeCRMHTTPMutationError(responseWriter, errorValue)
		return
	}
	if errorValue := service.authorizeCRMActivity(request.Context(), actor, activity); errorValue != nil {
		writeCRMHTTPMutationError(responseWriter, errorValue)
		return
	}
	written, errorValue := service.writeCRMActivity(request.Context(), activity)
	if errorValue != nil {
		writeCRMHTTPMutationError(responseWriter, errorValue)
		return
	}
	writeCRMHTTPJSON(responseWriter, http.StatusOK, map[string]any{"activity": crmHTTPActivityFromDomain(written)})
}

func (service *Service) archiveCRMActivityHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	service.archiveCRMRecordHTTP(responseWriter, request, service.archiveCRMActivity)
}

func (service *Service) restoreCRMActivityHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	service.restoreCRMRecordHTTP(responseWriter, request, "activity")
}

func crmActivityFromHTTP(payload crmHTTPActivityPayload, id string, audit crmAuditFields) crmActivity {
	return crmActivity{
		ID: id, AccountID: strings.TrimSpace(payload.AccountID), ContactID: strings.TrimSpace(payload.ContactID),
		OpportunityID: strings.TrimSpace(payload.OpportunityID), Business: strings.TrimSpace(payload.Business),
		Kind: strings.TrimSpace(payload.Kind), Title: strings.TrimSpace(payload.Title), OccurredAt: strings.TrimSpace(payload.OccurredAt),
		Content: strings.TrimSpace(payload.Content), Audit: audit,
	}
}

func crmHTTPActivityFromDomain(activity crmActivity) crmHTTPActivity {
	return crmHTTPActivity{
		ID: activity.ID, AccountID: activity.AccountID, ContactID: activity.ContactID, OpportunityID: activity.OpportunityID,
		Business: activity.Business, Kind: activity.Kind, Title: activity.Title, OccurredAt: activity.OccurredAt,
		Content: activity.Content, Audit: crmHTTPAuditFromDomain(activity.Audit),
	}
}
