package capabilityd

import (
	"net/http"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/llmbackend"
	"github.com/yeomyeonggeori/internkim/internal/modelladder"
)

func (configuration Configuration) reachingModels() Configuration {
	if !modelladder.PaysThroughGateway(configuration.OpenRouterKeyPath, configuration.CentralPlaneURL) {
		return configuration
	}
	chatCompletionsURL := modelladder.GatewayEndpoint(configuration.CentralPlaneURL) + "/chat/completions"
	configuration.OpenRouterKeyPath = configuration.HostSessionPath
	configuration.OpenRouterBaseURL = chatCompletionsURL
	configuration.OpenRouterWebBaseURL = chatCompletionsURL
	configuration.ModelGatewayNamesRequester = true
	return configuration
}

func (service Service) nameRequesterToTheGateway(request *http.Request) {
	if !service.Configuration.ModelGatewayNamesRequester {
		return
	}
	llmbackend.NameRequester(request)
}

func (service Service) requesterHeadersToTheGateway(requesterEmail string) map[string]string {
	email := strings.TrimSpace(requesterEmail)
	if !service.Configuration.ModelGatewayNamesRequester || email == "" {
		return nil
	}
	return map[string]string{llmbackend.RequesterEmailHeader: email}
}
