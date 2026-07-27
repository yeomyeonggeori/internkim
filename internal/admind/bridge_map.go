package admind

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type bridgeMessageLookupResponse struct {
	Found   bool                 `json:"found"`
	Mapping bridgeMessageMapping `json:"mapping"`
}

type bridgeChannelLookupResponse struct {
	Found   bool                 `json:"found"`
	Mapping bridgeChannelMapping `json:"mapping"`
}

func (service *Service) handleBridgeMap(responseWriter http.ResponseWriter, request *http.Request) {
	if !isLocalRequest(request) {
		http.Error(responseWriter, "local access required", http.StatusForbidden)
		return
	}
	path := strings.TrimPrefix(request.URL.Path, "/bridge/api")
	switch {
	case request.Method == http.MethodPost && path == "/message":
		service.handleBridgeMessageRecord(responseWriter, request)
	case request.Method == http.MethodGet && path == "/message":
		service.handleBridgeMessageLookup(responseWriter, request)
	case request.Method == http.MethodPost && path == "/channel":
		service.handleBridgeChannelRecord(responseWriter, request)
	case request.Method == http.MethodGet && path == "/channel":
		service.handleBridgeChannelLookup(responseWriter, request)
	case request.Method == http.MethodPost && path == "/channel/resolve":
		service.handleBridgeChannelResolve(responseWriter, request)
	case request.Method == http.MethodPost && path == "/bootstrap":
		service.handleBridgeBootstrap(responseWriter, request)
	case request.Method == http.MethodGet && path == "/identity":
		service.handleBridgeIdentity(responseWriter, request)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) handleBridgeMessageRecord(responseWriter http.ResponseWriter, request *http.Request) {
	var mapping bridgeMessageMapping
	if errorValue := json.NewDecoder(request.Body).Decode(&mapping); errorValue != nil {
		http.Error(responseWriter, "invalid request", http.StatusBadRequest)
		return
	}
	if mapping.BuzzEventID == "" || mapping.Platform == "" || mapping.ExternalID == "" {
		http.Error(responseWriter, "buzzEventId, platform, externalId are required", http.StatusBadRequest)
		return
	}
	database, errorValue := service.openBridgeMapDatabase(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, "bridge_map_unavailable", http.StatusInternalServerError)
		return
	}
	defer database.Close()
	if errorValue := service.recordBridgeMessage(request.Context(), database, mapping); errorValue != nil {
		http.Error(responseWriter, "bridge_map_write_failed", http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, map[string]bool{"recorded": true})
}

func (service *Service) handleBridgeMessageLookup(responseWriter http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	platform := query.Get("platform")
	database, errorValue := service.openBridgeMapDatabase(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, "bridge_map_unavailable", http.StatusInternalServerError)
		return
	}
	defer database.Close()
	var mapping bridgeMessageMapping
	var found bool
	if externalID := query.Get("externalId"); externalID != "" {
		mapping, found, errorValue = service.bridgeMessageByExternal(request.Context(), database, platform, externalID)
	} else if buzzEventID := query.Get("buzzEventId"); buzzEventID != "" {
		mapping, found, errorValue = service.bridgeMessageByEvent(request.Context(), database, buzzEventID, platform)
	} else {
		http.Error(responseWriter, "externalId or buzzEventId is required", http.StatusBadRequest)
		return
	}
	if errorValue != nil {
		http.Error(responseWriter, "bridge_map_read_failed", http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, bridgeMessageLookupResponse{Found: found, Mapping: mapping})
}

func (service *Service) handleBridgeChannelRecord(responseWriter http.ResponseWriter, request *http.Request) {
	var mapping bridgeChannelMapping
	if errorValue := json.NewDecoder(request.Body).Decode(&mapping); errorValue != nil {
		http.Error(responseWriter, "invalid request", http.StatusBadRequest)
		return
	}
	if mapping.BuzzChannelID == "" || mapping.Platform == "" || mapping.ExternalChannelID == "" {
		http.Error(responseWriter, "buzzChannelId, platform, externalChannelId are required", http.StatusBadRequest)
		return
	}
	database, errorValue := service.openBridgeMapDatabase(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, "bridge_map_unavailable", http.StatusInternalServerError)
		return
	}
	defer database.Close()
	if errorValue := service.recordBridgeChannel(request.Context(), database, mapping); errorValue != nil {
		http.Error(responseWriter, "bridge_map_write_failed", http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, map[string]bool{"recorded": true})
}

func (service *Service) handleBridgeChannelLookup(responseWriter http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	platform := query.Get("platform")
	database, errorValue := service.openBridgeMapDatabase(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, "bridge_map_unavailable", http.StatusInternalServerError)
		return
	}
	defer database.Close()
	var mapping bridgeChannelMapping
	var found bool
	if externalChannelID := query.Get("externalChannelId"); externalChannelID != "" {
		mapping, found, errorValue = service.bridgeChannelByExternal(request.Context(), database, platform, externalChannelID)
	} else if buzzChannelID := query.Get("buzzChannelId"); buzzChannelID != "" {
		mapping, found, errorValue = service.bridgeChannelByBuzz(request.Context(), database, buzzChannelID, platform)
	} else {
		http.Error(responseWriter, "externalChannelId or buzzChannelId is required", http.StatusBadRequest)
		return
	}
	if errorValue != nil {
		http.Error(responseWriter, "bridge_map_read_failed", http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, bridgeChannelLookupResponse{Found: found, Mapping: mapping})
}

type bridgeChannelResolveRequest struct {
	Platform          string `json:"platform"`
	ExternalChannelID string `json:"externalChannelId"`
}

func (service *Service) handleBridgeChannelResolve(responseWriter http.ResponseWriter, request *http.Request) {
	var payload bridgeChannelResolveRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, "invalid request", http.StatusBadRequest)
		return
	}
	if payload.Platform == "" || payload.ExternalChannelID == "" {
		http.Error(responseWriter, "platform and externalChannelId are required", http.StatusBadRequest)
		return
	}
	buzzChannelID, errorValue := service.resolveBridgeChannel(request.Context(), payload.Platform, payload.ExternalChannelID)
	if errors.Is(errorValue, errBridgeSeedMissing) {
		http.Error(responseWriter, "buzz key seed is not configured", http.StatusNotImplemented)
		return
	}
	if errorValue != nil {
		http.Error(responseWriter, "bridge_channel_resolve_failed", http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, map[string]string{"buzzChannelId": buzzChannelID})
}

func (service *Service) handleBridgeBootstrap(responseWriter http.ResponseWriter, request *http.Request) {
	count, errorValue := service.bootstrapBridgeChannels(request.Context())
	if errors.Is(errorValue, errBridgeSeedMissing) {
		http.Error(responseWriter, "buzz key seed is not configured", http.StatusNotImplemented)
		return
	}
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, map[string]int{"channels": count})
}

func (service *Service) handleBridgeIdentity(responseWriter http.ResponseWriter, request *http.Request) {
	email := strings.TrimSpace(request.URL.Query().Get("email"))
	if email == "" {
		http.Error(responseWriter, "email is required", http.StatusBadRequest)
		return
	}
	secretHex, errorValue := service.personBuzzSecret(request.Context(), email)
	if errors.Is(errorValue, errBuzzKeySeedMissing) {
		http.Error(responseWriter, "buzz key seed is not configured", http.StatusNotImplemented)
		return
	}
	if errorValue != nil {
		http.Error(responseWriter, "bridge_identity_failed", http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, map[string]string{"secretHex": secretHex})
}
