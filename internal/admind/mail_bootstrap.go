package admind

import (
	"gitlab.com/eastriver/internkim/internal/mail"
	"net/http"
)

type mailBootstrapResponse struct {
	Account            mail.AccountResponse   `json:"account"`
	Mailboxes          []mail.MailboxResponse `json:"mailboxes"`
	Messages           []mail.MessageResponse `json:"messages"`
	NextCursor         string                 `json:"nextCursor"`
	HasCachedMailboxes bool                   `json:"hasCachedMailboxes"`
	HasCachedMessages  bool                   `json:"hasCachedMessages"`
}

func (service *Service) writeMailBootstrap(responseWriter http.ResponseWriter, request *http.Request) {
	account, found, errorValue := service.readMailAccountForRequest(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	response := mailBootstrapResponse{Account: mail.AccountToResponse(account)}
	if !found || !account.IsConfigured() {
		service.writeJSON(responseWriter, response)
		return
	}
	input, errorValue := mail.MessageListRequestFromURL(request)
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
