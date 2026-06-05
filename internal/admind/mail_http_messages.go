package admind

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (service *Service) writeMailboxes(responseWriter http.ResponseWriter, request *http.Request) {
	account, _, errorValue := service.readConfiguredMailAccount(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	mailboxes, errorValue := service.mailBackend.ListMailboxes(request.Context(), account)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]any{"mailboxes": mailboxes})
}

func (service *Service) writeMailMessages(responseWriter http.ResponseWriter, request *http.Request) {
	account, _, errorValue := service.readConfiguredMailAccount(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	input, errorValue := mailMessageListRequestFromURL(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	result, errorValue := service.mailBackend.ListMessages(request.Context(), account, input)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, result)
}

func (service *Service) writeMailMessage(responseWriter http.ResponseWriter, request *http.Request) {
	account, _, errorValue := service.readConfiguredMailAccount(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	mailbox, uid, errorValue := mailMessagePathParts(request, "")
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	message, errorValue := service.mailBackend.ReadMessage(request.Context(), account, mailbox, uid)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, message)
}

func (service *Service) sendMailMessage(responseWriter http.ResponseWriter, request *http.Request) {
	account, _, errorValue := service.readConfiguredMailAccount(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	var payload mailMessageSendRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	payload, errorValue = validateMailMessageSendRequest(payload)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	result, errorValue := service.mailBackend.SendMessage(request.Context(), account, payload)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, result)
}

func (service *Service) moveMailMessage(responseWriter http.ResponseWriter, request *http.Request) {
	account, _, errorValue := service.readConfiguredMailAccount(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	mailbox, uid, errorValue := mailMessagePathParts(request, "/move")
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	var payload mailMessageMoveRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	payload.TargetMailbox = strings.TrimSpace(payload.TargetMailbox)
	if payload.TargetMailbox == "" {
		http.Error(responseWriter, "targetMailbox is required", http.StatusBadRequest)
		return
	}
	if errorValue := service.mailBackend.MoveMessage(request.Context(), account, mailbox, uid, payload.TargetMailbox); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]bool{"moved": true})
}

func (service *Service) markMailMessage(responseWriter http.ResponseWriter, request *http.Request) {
	account, _, errorValue := service.readConfiguredMailAccount(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	mailbox, uid, errorValue := mailMessagePathParts(request, "/flags")
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	var payload mailMessageMarkRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if payload.Seen == nil && payload.Flagged == nil {
		http.Error(responseWriter, "seen or flagged is required", http.StatusBadRequest)
		return
	}
	if errorValue := service.mailBackend.MarkMessage(request.Context(), account, mailbox, uid, payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]bool{"marked": true})
}
