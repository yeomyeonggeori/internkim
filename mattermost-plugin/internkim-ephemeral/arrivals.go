package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

const defaultArrivalsURL = "http://127.0.0.1:18091"
const arrivalPreviewLimit = 140
const arrivalMemberLimit = 200

type arrivedMessage struct {
	ConversationID       string   `json:"conversationID"`
	MessageID            string   `json:"messageID"`
	AuthorExternalID     string   `json:"authorExternalID"`
	AuthorName           string   `json:"authorName,omitempty"`
	RecipientExternalIDs []string `json:"recipientExternalIDs"`
	Preview              string   `json:"preview"`
}

func arrivalFromPost(post *model.Post, authorName string, memberIDs []string) arrivedMessage {
	return arrivedMessage{
		ConversationID:       post.ChannelId,
		MessageID:            post.Id,
		AuthorExternalID:     post.UserId,
		AuthorName:           authorName,
		RecipientExternalIDs: memberIDs,
		Preview:              trimPreview(post.Message),
	}
}

func displayNameOf(user *model.User) string {
	if user == nil {
		return ""
	}
	if name := strings.TrimSpace(user.GetFullName()); name != "" {
		return name
	}
	return user.Username
}

func trimPreview(message string) string {
	runes := []rune(message)
	if len(runes) <= arrivalPreviewLimit {
		return message
	}
	return string(runes[:arrivalPreviewLimit])
}

func userIDsOf(members model.ChannelMembers) []string {
	userIDs := make([]string, 0, len(members))
	for _, member := range members {
		userIDs = append(userIDs, member.UserId)
	}
	return userIDs
}

func (pluginValue *Plugin) reportArrival(post *model.Post) {
	defer func() {
		if recovered := recover(); recovered != nil {
			pluginValue.API.LogWarn("arrival not reported", "panic", recovered)
		}
	}()

	members, appError := pluginValue.API.GetChannelMembers(post.ChannelId, 0, arrivalMemberLimit)
	if appError != nil {
		pluginValue.API.LogWarn("arrival not reported: member lookup failed", "error", appError.Error())
		return
	}
	pluginValue.postArrival(arrivalFromPost(post, pluginValue.authorNameOf(post.UserId), userIDsOf(members)))
}

func (pluginValue *Plugin) authorNameOf(userID string) string {
	user, appError := pluginValue.API.GetUser(userID)
	if appError != nil {
		pluginValue.API.LogWarn("arrival carries no author name", "error", appError.Error())
		return ""
	}
	return displayNameOf(user)
}

func (pluginValue *Plugin) postArrival(arrival arrivedMessage) {
	payload, errorValue := json.Marshal(arrival)
	if errorValue != nil {
		return
	}
	requestContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	request, errorValue := http.NewRequestWithContext(
		requestContext,
		http.MethodPost,
		pluginValue.arrivalsURL(),
		bytes.NewReader(payload),
	)
	if errorValue != nil {
		return
	}
	request.Header.Set("Content-Type", "application/json")
	response, errorValue := pluginValue.runtimeHealthHTTPClient().Do(request)
	if errorValue != nil {
		pluginValue.API.LogWarn("arrival not delivered", "error", errorValue.Error())
		return
	}
	_ = response.Body.Close()
}
