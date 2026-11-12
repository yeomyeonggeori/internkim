package admind

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

var (
	errDirectoryUnavailable = errors.New("this host has no company directory configured")
	errNoActiveMember       = errors.New("no active member has this address, so nothing can be sent as them")
)

type directoryDirectMessageRequest struct {
	SenderEmail    string `json:"senderEmail"`
	RecipientEmail string `json:"recipientEmail"`
	Message        string `json:"message"`
}

type directoryDirectMessageResponse struct {
	ChannelID string `json:"channelID"`
	MessageID string `json:"messageID"`
}

// handleDirectoryDirectMessage carries a message somebody asked the agent to
// pass on, and sends it under their own name rather than the agent's. The
// recipient sees a colleague writing to them, which is what happened.
//
// The sender's secret is derived here and handed only to chatd, because the
// seed lives with this process: the capability layer names two members and
// never holds a key. A sender the company cannot key is refused rather than
// quietly rewritten to the agent, so nobody's words go out under a name they
// did not choose.
func (service *Service) handleDirectoryDirectMessage(responseWriter http.ResponseWriter, request *http.Request) {
	var payload directoryDirectMessageRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, "invalid request", http.StatusBadRequest)
		return
	}
	senderEmail := strings.ToLower(strings.TrimSpace(payload.SenderEmail))
	recipientEmail := strings.ToLower(strings.TrimSpace(payload.RecipientEmail))
	message := strings.TrimSpace(payload.Message)
	if senderEmail == "" || message == "" {
		http.Error(responseWriter, "senderEmail and message are required", http.StatusBadRequest)
		return
	}

	senderSecretHex, errorValue := service.memberBuzzSecret(request.Context(), senderEmail)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusConflict)
		return
	}
	sendRequest := map[string]any{"userSecretHex": senderSecretHex, "message": message}
	if recipientEmail != "" {
		recipientPubkeyHex, status, errorValue := service.memberBuzzPublicKey(request.Context(), recipientEmail)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), status)
			return
		}
		sendRequest["counterpartPubkeyHex"] = recipientPubkeyHex
	}

	var sent directoryDirectMessageResponse
	if errorValue := service.chatdPlatformRequest(request.Context(), "dm.send", sendRequest, &sent); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode(sent)
}

func (service *Service) memberBuzzPublicKey(ctx context.Context, email string) (string, int, error) {
	secretHex, errorValue := service.memberBuzzSecret(ctx, email)
	if errorValue != nil {
		return "", http.StatusNotFound, errorValue
	}
	pubkeyHex, errorValue := buzzPublicKey(secretHex)
	if errorValue != nil {
		return "", http.StatusBadGateway, errorValue
	}
	return pubkeyHex, http.StatusOK, nil
}

// memberBuzzSecret answers the key of somebody who works here. Being in the
// directory is what makes an identity derivable, so a stranger's address never
// mints one.
func (service *Service) memberBuzzSecret(ctx context.Context, email string) (string, error) {
	client := service.centralPlane()
	if client == nil {
		return "", errDirectoryUnavailable
	}
	member, isKnown, errorValue := client.MemberByEmail(ctx, email)
	if errorValue != nil {
		return "", errorValue
	}
	if !isKnown || !member.IsActive() {
		return "", errNoActiveMember
	}
	return service.personBuzzSecret(ctx, email)
}
