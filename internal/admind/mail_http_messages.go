package admind

import (
	"context"
	"encoding/json"
	"github.com/yeomyeonggeori/internkim/internal/mail"
	"log/slog"
	"net/http"
	"strconv"
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
	if errorValue := service.saveCachedMailboxes(request.Context(), account.ActorEmail, mailboxes); errorValue != nil {
		logMailCacheFailure("save mailboxes", account.ActorEmail, errorValue)
	}
	service.writeJSON(responseWriter, map[string]any{"mailboxes": mailboxes})
}

func (service *Service) writeMailMessages(responseWriter http.ResponseWriter, request *http.Request) {
	account, _, errorValue := service.readConfiguredMailAccount(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	input, errorValue := mail.MessageListRequestFromURL(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	result, errorValue := service.mailBackend.ListMessages(request.Context(), account, input)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if errorValue := service.saveCachedMailMessages(request.Context(), account.ActorEmail, input, result); errorValue != nil {
		logMailCacheFailure("save message list", account.ActorEmail, errorValue)
	}
	service.writeJSON(responseWriter, result)
}

func (service *Service) writeMailMessage(responseWriter http.ResponseWriter, request *http.Request) {
	account, _, errorValue := service.readConfiguredMailAccount(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	mailbox, uid, errorValue := mail.MessagePathParts(request, "")
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	message, errorValue := service.mailBackend.ReadMessage(request.Context(), account, mailbox, uid)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if errorValue := service.saveCachedMailMessageDetail(request.Context(), account.ActorEmail, message); errorValue != nil {
		logMailCacheFailure("save message detail", account.ActorEmail, errorValue)
	}
	service.writeJSON(responseWriter, message)
}

func (service *Service) sendMailMessage(responseWriter http.ResponseWriter, request *http.Request) {
	account, _, errorValue := service.readConfiguredMailAccount(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	var payload mail.MessageSendRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	payload, errorValue = mail.ValidateMessageSendRequest(payload)
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
	mailbox, uid, errorValue := mail.MessagePathParts(request, "/move")
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	var payload mail.MessageMoveRequest
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
	if errorValue := service.deleteCachedMailMessage(request.Context(), account.ActorEmail, mailbox, uid); errorValue != nil {
		logMailCacheFailure("delete moved message", account.ActorEmail, errorValue)
		service.clearMailCacheBestEffort(request.Context(), account.ActorEmail)
	}
	service.writeJSON(responseWriter, map[string]any{"moved": true, "uid": strconv.FormatUint(uint64(uid), 10)})
}

func (service *Service) markMailMessage(responseWriter http.ResponseWriter, request *http.Request) {
	account, _, errorValue := service.readConfiguredMailAccount(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	mailbox, uid, errorValue := mail.MessagePathParts(request, "/flags")
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	var payload mail.MessageMarkRequest
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
	if errorValue := service.updateCachedMailMessageFlags(request.Context(), account.ActorEmail, mailbox, uid, payload); errorValue != nil {
		logMailCacheFailure("update message flags", account.ActorEmail, errorValue)
		service.clearMailCacheBestEffort(request.Context(), account.ActorEmail)
	}
	service.writeJSON(responseWriter, map[string]any{"marked": true, "uid": strconv.FormatUint(uint64(uid), 10)})
}

func logMailCacheFailure(operation string, actorEmail string, errorValue error) {
	slog.Warn("mail cache operation failed", "operation", operation, "actor_email", strings.ToLower(strings.TrimSpace(actorEmail)), "error", errorValue.Error())
}

func (service *Service) clearMailCacheBestEffort(ctx context.Context, actorEmail string) {
	if errorValue := service.clearMailCache(ctx, actorEmail); errorValue != nil {
		logMailCacheFailure("clear", actorEmail, errorValue)
	}
}
