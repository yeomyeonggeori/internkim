package admind

import (
	"io"
	"net/http"
	"strings"
)

// The relay mints blob URLs at its own origin (the device loopback), which a
// browser can never reach. Every media URL handed to the web is rewritten to
// this prefix and served back through admind on the public domain, which then
// fetches the blob from the loopback relay on the browser's behalf.
const buzzMediaProxyPrefix = "/buzz-media/"

func (service *Service) buzzMediaOrigin() string {
	relayURL := strings.TrimSpace(service.Configuration.BuzzRelayURL)
	if strings.HasPrefix(relayURL, "wss://") {
		return "https://" + strings.TrimPrefix(relayURL, "wss://")
	}
	if strings.HasPrefix(relayURL, "ws://") {
		return "http://" + strings.TrimPrefix(relayURL, "ws://")
	}
	return relayURL
}

func (service *Service) rewriteBuzzMedia(value string) string {
	origin := service.buzzMediaOrigin()
	if origin == "" || value == "" {
		return value
	}
	return strings.ReplaceAll(value, origin+"/", buzzMediaProxyPrefix)
}

func (service *Service) handleBuzzMediaProxy(responseWriter http.ResponseWriter, request *http.Request) {
	if service.webActorEmail(request) == "" {
		http.Error(responseWriter, "unauthorized", http.StatusUnauthorized)
		return
	}
	origin := service.buzzMediaOrigin()
	if origin == "" {
		http.Error(responseWriter, "buzz media unavailable", http.StatusNotImplemented)
		return
	}
	upstreamURL := origin + "/" + strings.TrimPrefix(request.URL.Path, buzzMediaProxyPrefix)
	upstreamRequest, errorValue := http.NewRequestWithContext(request.Context(), http.MethodGet, upstreamURL, nil)
	if errorValue != nil {
		http.Error(responseWriter, "invalid media request", http.StatusBadRequest)
		return
	}
	response, errorValue := service.httpClient().Do(upstreamRequest)
	if errorValue != nil {
		http.Error(responseWriter, "media_unreachable", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	copyHeader(responseWriter, response, "Content-Type")
	copyHeader(responseWriter, response, "Content-Length")
	copyHeader(responseWriter, response, "Cache-Control")
	responseWriter.WriteHeader(response.StatusCode)
	io.Copy(responseWriter, response.Body)
}

func copyHeader(responseWriter http.ResponseWriter, response *http.Response, name string) {
	if value := response.Header.Get(name); value != "" {
		responseWriter.Header().Set(name, value)
	}
}

func (service *Service) rewriteConversationMedia(response agentConversationResponse) agentConversationResponse {
	for messageIndex := range response.Messages {
		message := &response.Messages[messageIndex]
		message.Text = service.rewriteBuzzMedia(message.Text)
		message.Sender.AvatarURL = service.rewriteBuzzMedia(message.Sender.AvatarURL)
		for attachmentIndex := range message.Attachments {
			message.Attachments[attachmentIndex].URL = service.rewriteBuzzMedia(message.Attachments[attachmentIndex].URL)
		}
		for reactionIndex := range message.Reactions {
			message.Reactions[reactionIndex].ImageURL = service.rewriteBuzzMedia(message.Reactions[reactionIndex].ImageURL)
		}
	}
	return response
}
