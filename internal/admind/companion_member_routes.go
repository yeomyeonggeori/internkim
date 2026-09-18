package admind

import (
	"encoding/json"
	"net/http"
	"strings"
)

const companionMemberAPIPrefix = "/companion/api"

type companionDisconnectRequest struct {
	CompanionID string `json:"companionID"`
}

func (service *Service) registerCompanionMemberRoutes(multiplexer *http.ServeMux) {
	multiplexer.HandleFunc(companionMemberAPIPrefix+"/", service.handleMemberCompanion)
}

func (service *Service) handleMemberCompanion(responseWriter http.ResponseWriter, request *http.Request) {
	actor, found, errorValue := resolvePersonaActor(service, request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(responseWriter, "workspace access required", http.StatusForbidden)
		return
	}
	if strings.TrimSpace(actor.email) == "" {
		http.Error(responseWriter, "a companion is paired with a signed-in email, and this call carried none", http.StatusForbidden)
		return
	}
	path := strings.TrimPrefix(request.URL.Path, companionMemberAPIPrefix)
	switch {
	case request.Method == http.MethodGet && path == "/mine":
		service.writeJSON(responseWriter, companionStatusResponse{Companions: service.companionsOwnedBy(actor)})
	case request.Method == http.MethodPost && path == "/pairing-codes":
		service.writeCompanionPairingCode(responseWriter, request, companionPairingCodeRequest{OwnerEmail: actor.email, OwnerPersonID: actor.personID})
	case request.Method == http.MethodPost && path == "/mine/disconnect":
		service.disconnectOwnCompanion(responseWriter, request, actor)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) companionsOwnedBy(actor personaActor) []CompanionStatus {
	owned := []CompanionStatus{}
	for _, status := range service.companionStatuses() {
		if companionBelongsTo(status, actor) {
			owned = append(owned, status)
		}
	}
	return owned
}

func companionBelongsTo(status CompanionStatus, actor personaActor) bool {
	if actor.personID != "" && strings.TrimSpace(status.OwnerPersonID) == actor.personID {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(status.OwnerEmail), strings.TrimSpace(actor.email))
}

func (service *Service) disconnectOwnCompanion(responseWriter http.ResponseWriter, request *http.Request, actor personaActor) {
	var asked companionDisconnectRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&asked); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	companionID := strings.TrimSpace(asked.CompanionID)
	if !service.ownsCompanion(actor, companionID) {
		http.NotFound(responseWriter, request)
		return
	}
	service.revokeCompanion(responseWriter, request, companionID)
}

func (service *Service) ownsCompanion(actor personaActor, companionID string) bool {
	for _, status := range service.companionsOwnedBy(actor) {
		if status.CompanionID == companionID {
			return true
		}
	}
	return false
}
