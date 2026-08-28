package admind

import (
	"encoding/json"
	"errors"
	"gitlab.com/eastriver/internkim/internal/mail"
	"net/http"
	"strings"
)

func (service *Service) handleMail(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.authorizeMailRequest(request) {
		http.Error(responseWriter, "mail access required", http.StatusForbidden)
		return
	}
	path := strings.TrimPrefix(request.URL.Path, "/mail/api")
	switch {
	case request.Method == http.MethodGet && path == "/bootstrap":
		service.writeMailBootstrap(responseWriter, request)
	case request.Method == http.MethodGet && path == "/account":
		service.writeMailAccount(responseWriter, request)
	case request.Method == http.MethodPut && path == "/account":
		service.saveMailAccount(responseWriter, request)
	case request.Method == http.MethodPost && path == "/account/test":
		service.testMailAccount(responseWriter, request)
	case request.Method == http.MethodGet && path == "/mailboxes":
		service.writeMailboxes(responseWriter, request)
	case request.Method == http.MethodGet && path == "/messages":
		service.writeMailMessages(responseWriter, request)
	case request.Method == http.MethodGet && strings.HasPrefix(path, "/messages/"):
		service.writeMailMessage(responseWriter, request)
	case request.Method == http.MethodPost && path == "/messages/send":
		service.sendMailMessage(responseWriter, request)
	case request.Method == http.MethodPost && strings.HasSuffix(path, "/move"):
		service.moveMailMessage(responseWriter, request)
	case request.Method == http.MethodPost && strings.HasSuffix(path, "/flags"):
		service.markMailMessage(responseWriter, request)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) authorizeMailRequest(request *http.Request) bool {
	actorEmail := service.mailActorEmail(request)
	if actorEmail == "" {
		return false
	}
	return isLocalRequest(request) || service.isTaskStaffActor(request.Context(), actorEmail)
}

func (service *Service) writeMailAccount(responseWriter http.ResponseWriter, request *http.Request) {
	account, _, errorValue := service.readMailAccountForRequest(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, mail.AccountToResponse(account))
}

func (service *Service) saveMailAccount(responseWriter http.ResponseWriter, request *http.Request) {
	existingAccount, _, errorValue := service.readMailAccountForRequest(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	var payload mail.AccountWriteRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	account, errorValue := mail.MergeAccountWriteRequest(existingAccount, payload)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := mail.ValidateAccountForSave(account); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := service.saveMailAccountRecord(request.Context(), account); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, mail.AccountToResponse(account))
}

func (service *Service) testMailAccount(responseWriter http.ResponseWriter, request *http.Request) {
	account, _, errorValue := service.readMailAccountForRequest(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if request.Body != nil && request.ContentLength != 0 {
		var payload mail.AccountWriteRequest
		if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
		account, errorValue = mail.MergeAccountWriteRequest(account, payload)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
	}
	if errorValue := mail.ValidateAccountForSave(account); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := service.mailBackend.TestAccount(request.Context(), account); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]bool{"ok": true})
}

func (service *Service) readConfiguredMailAccount(request *http.Request) (mail.Account, bool, error) {
	account, found, errorValue := service.readMailAccountForRequest(request)
	if errorValue != nil {
		return mail.Account{}, false, errorValue
	}
	if !found || !account.IsConfigured() {
		return mail.Account{}, found, errors.New("mail account is not configured")
	}
	return account, found, nil
}

func (service *Service) readMailAccountForRequest(request *http.Request) (mail.Account, bool, error) {
	actorEmail := service.mailActorEmail(request)
	if actorEmail == "" {
		return mail.Account{}, false, errors.New("mail actor email is required")
	}
	return service.readMailAccount(request.Context(), actorEmail)
}

func (service *Service) mailActorEmail(request *http.Request) string {
	if actorEmail := service.authenticatedCallerEmail(request); actorEmail != "" {
		return actorEmail
	}
	if actorEmail := service.webStaffActorEmail(request); actorEmail != "" {
		return actorEmail
	}
	return assertedRequesterEmail(request)
}
