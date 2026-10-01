package mail

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

const servePrefix = "/v1/mail/"

type call struct {
	Account       Account            `json:"account"`
	MemberID      string             `json:"memberID"`
	Mailbox       string             `json:"mailbox"`
	UID           uint32             `json:"uid"`
	TargetMailbox string             `json:"targetMailbox"`
	List          MessageListRequest `json:"list"`
	Send          MessageSendRequest `json:"send"`
	Mark          MessageMarkRequest `json:"mark"`
}

func Handler(backend Backend, openPasswords PasswordOpener) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || !strings.HasPrefix(request.URL.Path, servePrefix) {
			http.NotFound(responseWriter, request)
			return
		}

		var asked call
		if errorValue := json.NewDecoder(request.Body).Decode(&asked); errorValue != nil {
			refuse(responseWriter, http.StatusBadRequest, "that is not a mail call")
			return
		}
		opened, errorValue := openPasswords(asked.Account, asked.MemberID)
		if errorValue != nil {
			refuse(responseWriter, http.StatusConflict, errorValue.Error())
			return
		}
		asked.Account = opened
		if !asked.Account.IsConfigured() {
			refuse(responseWriter, http.StatusBadRequest, "this call named no account")
			return
		}

		answer, errorValue := serve(request, backend, strings.TrimPrefix(request.URL.Path, servePrefix), asked)
		if errors.Is(errorValue, errUnknownCapability) {
			http.NotFound(responseWriter, request)
			return
		}
		if errorValue != nil {
			refuse(responseWriter, http.StatusBadGateway, errorValue.Error())
			return
		}
		writeJSON(responseWriter, answer)
	})
}

var errUnknownCapability = errors.New("no such mail capability")

func serve(request *http.Request, backend Backend, capability string, asked call) (any, error) {
	context := request.Context()
	switch capability {
	case "test":
		return map[string]bool{"reachable": true}, backend.TestAccount(context, asked.Account)
	case "mailboxes":
		return backend.ListMailboxes(context, asked.Account)
	case "messages":
		return backend.ListMessages(context, asked.Account, asked.List)
	case "message":
		return backend.ReadMessage(context, asked.Account, asked.Mailbox, asked.UID)
	case "send":
		return backend.SendMessage(context, asked.Account, asked.Send)
	case "move":
		return done(backend.MoveMessage(context, asked.Account, asked.Mailbox, asked.UID, asked.TargetMailbox))
	case "mark":
		return done(backend.MarkMessage(context, asked.Account, asked.Mailbox, asked.UID, asked.Mark))
	default:
		return nil, errUnknownCapability
	}
}

func done(errorValue error) (any, error) {
	return map[string]bool{"done": errorValue == nil}, errorValue
}

func refuse(responseWriter http.ResponseWriter, status int, because string) {
	responseWriter.WriteHeader(status)
	writeJSON(responseWriter, map[string]string{"error": because})
}

func writeJSON(responseWriter http.ResponseWriter, answer any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode(answer)
}
