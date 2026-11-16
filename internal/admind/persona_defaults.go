package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

const soulRevisionPath = "/admin/api/persona/agent/soul"

const (
	soulRevisionOriginAdministrator = "administrator"
	soulRevisionOriginDefault       = "default"
)

type soulRevisionRequest struct {
	ExpectedVersion int             `json:"expectedVersion"`
	Document        json.RawMessage `json:"document"`
	Reason          string          `json:"reason"`
	Origin          string          `json:"origin"`
	Author          string          `json:"author,omitempty"`
}

func (service *Service) replaceUntouchedDefaults(ctx context.Context) error {
	if errorValue := service.replaceUntouchedDefaultIdentity(ctx); errorValue != nil {
		return errorValue
	}
	return service.replaceUntouchedDefaultSoul(ctx)
}

func (service *Service) replaceUntouchedDefaultIdentity(ctx context.Context) error {
	identity, _, errorValue := service.loadIdentity()
	if errorValue != nil {
		return errorValue
	}
	updated := withCurrentDefaultWording(identity)
	if updated.Role == identity.Role && updated.Introduction == identity.Introduction {
		return nil
	}
	if errorValue := service.saveIdentity(updated); errorValue != nil {
		return errorValue
	}
	log.Printf("persona: the role or introduction was the retired default and now reads the current one")
	return service.syncAgentPersona(ctx)
}

func withCurrentDefaultWording(identity identityDocument) identityDocument {
	retired := retiredDefaultIdentityDocument()
	current := defaultIdentityDocument()
	if identity.Role == retired.Role {
		identity.Role = current.Role
	}
	if identity.Introduction == retired.Introduction {
		identity.Introduction = current.Introduction
	}
	return identity
}

func (service *Service) replaceUntouchedDefaultSoul(ctx context.Context) error {
	var installed rawAgentPersonaDocuments
	if errorValue := service.blueclawPersonaRequest(ctx, http.MethodGet, "/admin/api/persona/agent", nil, "internkim-persona-service", &installed); errorValue != nil {
		return errorValue
	}
	isUntouched, errorValue := isRetiredDefaultSoul(installed.Soul)
	if errorValue != nil || !isUntouched {
		return errorValue
	}
	document, errorValue := canonicalSoulDocument(defaultSoulDocument())
	if errorValue != nil {
		return errorValue
	}
	payload, errorValue := json.Marshal(soulRevisionRequest{ExpectedVersion: installed.SoulVersion, Document: document, Origin: soulRevisionOriginDefault})
	if errorValue != nil {
		return errorValue
	}
	if errorValue := service.blueclawPersonaRequest(ctx, http.MethodPost, soulRevisionPath, payload, "internkim-persona-service", nil); errorValue != nil {
		return fmt.Errorf("replace the retired default soul: %w", errorValue)
	}
	log.Printf("persona: the soul was the retired default and now reads the current one")
	return service.saveSoul(defaultSoulDocument())
}

func isRetiredDefaultSoul(installed json.RawMessage) (bool, error) {
	soul, errorValue := parseSoulDocument(installed)
	if errorValue != nil {
		return false, errorValue
	}
	current, errorValue := canonicalSoulDocument(soul)
	if errorValue != nil {
		return false, errorValue
	}
	retired, errorValue := canonicalSoulDocument(retiredDefaultSoulDocument())
	if errorValue != nil {
		return false, errorValue
	}
	return bytes.Equal(current, retired), nil
}

func (service *Service) reviseSoulAsAdministrator(responseWriter http.ResponseWriter, request *http.Request) {
	actor, found, errorValue := resolvePersonaActor(service, request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if !found || !actor.isAdmin {
		http.Error(responseWriter, "only an administrator changes the working principles", http.StatusForbidden)
		return
	}
	var revision soulRevisionRequest
	if errorValue := json.NewDecoder(http.MaxBytesReader(responseWriter, request.Body, 64*1024)).Decode(&revision); errorValue != nil {
		http.Error(responseWriter, "invalid request body", http.StatusBadRequest)
		return
	}
	soul, errorValue := parseSoulDocument(revision.Document)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	document, errorValue := canonicalSoulDocument(soul)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	payload, errorValue := json.Marshal(soulRevisionRequest{ExpectedVersion: revision.ExpectedVersion, Document: document, Reason: revision.Reason, Origin: soulRevisionOriginAdministrator, Author: actor.personID})
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	status, answer, errorValue := service.blueclawSignedAnswer(request.Context(), http.MethodPost, soulRevisionPath, payload, "internkim-persona-service")
	if errorValue != nil {
		http.Error(responseWriter, "learning service unavailable", http.StatusBadGateway)
		return
	}
	if status >= 200 && status < 300 {
		responseWriter.Header().Set("Content-Type", "application/json")
	}
	responseWriter.WriteHeader(status)
	_, _ = responseWriter.Write(answer)
}
