package admind

import (
	"io"
	"net/http"
	"strings"
)

// The messenger backend mints blob URLs at its own origin — the device loopback
// for live traffic, or the public relay host for imported history — neither of
// which a browser should fetch directly (the loopback is unreachable and the
// relay host has no public ingress). Every media URL handed to the web is
// rewritten to this prefix and served back through admind on the public domain,
// which then fetches the blob from the loopback backend on the browser's behalf.
// The prefix is backend-neutral: it names what the resource is, not which
// messaging backend produced it, so a future backend swap needs no URL change.
const mediaProxyPrefix = "/attachments/"

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

// buzzPublicRelayOrigin is the https origin imported media is stamped with,
// derived from the device URL the same way the importer does (host -> host with
// "-relay" before the first label), e.g. https://foo.example.test ->
// https://foo-relay.example.test.
func (service *Service) buzzPublicRelayOrigin() string {
	return relayOriginForDeviceURL(service.mediaPublicBase())
}

func mediaRelayHost(origin string) string {
	if index := strings.Index(origin, "://"); index >= 0 {
		return origin[index+3:]
	}
	return origin
}

func relayOriginForDeviceURL(deviceURL string) string {
	scheme, host := "", deviceURL
	if index := strings.Index(deviceURL, "://"); index >= 0 {
		scheme, host = deviceURL[:index+3], deviceURL[index+3:]
	}
	if host == "" {
		return ""
	}
	if dot := strings.Index(host, "."); dot >= 0 {
		host = host[:dot] + "-relay" + host[dot:]
	} else {
		host += "-relay"
	}
	return scheme + host
}

func (service *Service) mediaPublicBase() string {
	return strings.TrimSuffix(strings.TrimSpace(readTrimmedFile(service.Configuration.DeviceURLPath)), "/")
}

func (service *Service) rewriteBuzzMedia(value string) string {
	if value == "" {
		return value
	}
	replacement := service.mediaPublicBase() + mediaProxyPrefix
	for _, origin := range []string{service.buzzMediaOrigin(), service.buzzPublicRelayOrigin()} {
		if origin != "" {
			value = strings.ReplaceAll(value, origin+"/", replacement)
		}
	}
	return value
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
	upstreamURL := origin + "/" + strings.TrimPrefix(request.URL.Path, mediaProxyPrefix)
	upstreamRequest, errorValue := http.NewRequestWithContext(request.Context(), http.MethodGet, upstreamURL, nil)
	if errorValue != nil {
		http.Error(responseWriter, "invalid media request", http.StatusBadRequest)
		return
	}
	// The relay serves media per-community, routed by Host header. Fetching over
	// loopback would present Host 127.0.0.1 and miss the community, so present
	// the public relay host the way a browser would have.
	if relayHost := mediaRelayHost(service.buzzPublicRelayOrigin()); relayHost != "" {
		upstreamRequest.Host = relayHost
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
		for emojiIndex := range message.CustomEmoji {
			message.CustomEmoji[emojiIndex].URL = service.rewriteBuzzMedia(message.CustomEmoji[emojiIndex].URL)
		}
	}
	return response
}
