package admind

import (
	"net/http"
)

type mailBootstrapResponse struct {
	Account            mailAccountResponse   `json:"account"`
	Mailboxes          []mailMailboxResponse `json:"mailboxes"`
	Messages           []mailMessageResponse `json:"messages"`
	NextCursor         string                `json:"nextCursor"`
	HasCachedMailboxes bool                  `json:"hasCachedMailboxes"`
	HasCachedMessages  bool                  `json:"hasCachedMessages"`
}

func (service *Service) writeMailBootstrap(responseWriter http.ResponseWriter, request *http.Request) {
	account, found, errorValue := service.readMailAccountForRequest(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	response := mailBootstrapResponse{Account: mailAccountToResponse(account)}
	if !found || !account.isConfigured() {
		service.writeJSON(responseWriter, response)
		return
	}
	input, errorValue := mailMessageListRequestFromURL(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	mailboxes, hasCachedMailboxes, errorValue := service.readCachedMailboxes(request.Context(), account.ActorEmail)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	messages, hasCachedMessages, errorValue := service.readCachedMailMessages(request.Context(), account.ActorEmail, input)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	response.Mailboxes = mailboxes
	response.Messages = messages.Messages
	response.NextCursor = messages.NextCursor
	response.HasCachedMailboxes = hasCachedMailboxes
	response.HasCachedMessages = hasCachedMessages
	service.writeJSON(responseWriter, response)
}
