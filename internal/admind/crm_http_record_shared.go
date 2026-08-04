package admind

import (
	"context"
	"net/http"
)

type crmArchiveOperation func(context.Context, string, string, string) error

func (service *Service) archiveCRMRecordHTTP(responseWriter http.ResponseWriter, request *http.Request, operation crmArchiveOperation) {
	actor, ok := service.crmHTTPActor(responseWriter, request)
	if !ok || !requireCRMAdmin(responseWriter, actor) {
		return
	}
	if errorValue := operation(request.Context(), request.PathValue("id"), crmCurrentTimestamp(), actor.PersonID); errorValue != nil {
		writeCRMHTTPMutationError(responseWriter, errorValue)
		return
	}
	writeCRMHTTPJSON(responseWriter, http.StatusOK, map[string]bool{"ok": true})
}

func (service *Service) restoreCRMRecordHTTP(responseWriter http.ResponseWriter, request *http.Request, entityType string) {
	actor, ok := service.crmHTTPActor(responseWriter, request)
	if !ok || !requireCRMAdmin(responseWriter, actor) {
		return
	}
	if errorValue := service.restoreCRMRecord(request.Context(), entityType, request.PathValue("id"), crmCurrentTimestamp(), actor.PersonID); errorValue != nil {
		writeCRMHTTPMutationError(responseWriter, errorValue)
		return
	}
	writeCRMHTTPJSON(responseWriter, http.StatusOK, map[string]bool{"ok": true})
}

func crmHTTPAuditFromDomain(audit crmAuditFields) crmHTTPAudit {
	return crmHTTPAudit{
		CreatedAt: audit.CreatedAt, CreatedByPersonID: audit.CreatedByPersonID,
		UpdatedAt: audit.UpdatedAt, UpdatedByPersonID: audit.UpdatedByPersonID,
		ArchivedAt: audit.ArchivedAt, ArchivedByPersonID: audit.ArchivedByPersonID,
	}
}
