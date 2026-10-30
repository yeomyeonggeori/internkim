package capabilityd

import "github.com/yeomyeonggeori/internkim/internal/modelladder"

func (configuration Configuration) reachingModels() Configuration {
	if !modelladder.PaysThroughGateway(configuration.OpenRouterKeyPath, configuration.CentralPlaneURL) {
		return configuration
	}
	chatCompletionsURL := modelladder.GatewayEndpoint(configuration.CentralPlaneURL) + "/chat/completions"
	configuration.OpenRouterKeyPath = configuration.HostSessionPath
	configuration.OpenRouterBaseURL = chatCompletionsURL
	configuration.OpenRouterWebBaseURL = chatCompletionsURL
	return configuration
}
