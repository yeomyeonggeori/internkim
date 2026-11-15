package capabilityd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/llmbackend"
	"github.com/yeomyeonggeori/internkim/internal/modelladder"
)

func (configuration Configuration) reachingModels() (Configuration, error) {
	paysThroughGateway, errorValue := configuration.paysThroughGatewayAsPrepared()
	if errorValue != nil || !paysThroughGateway {
		return configuration, errorValue
	}
	chatCompletionsURL := modelladder.GatewayEndpoint(configuration.CentralPlaneURL) + "/chat/completions"
	configuration.OpenRouterKeyPath = configuration.HostSessionPath
	configuration.OpenRouterBaseURL = chatCompletionsURL
	configuration.OpenRouterWebBaseURL = chatCompletionsURL
	configuration.ModelGatewayNamesRequester = true
	return configuration, nil
}

func (configuration Configuration) paysThroughGatewayAsPrepared() (bool, error) {
	if strings.TrimSpace(configuration.CentralPlaneURL) == "" {
		return false, nil
	}
	document, errorValue := os.ReadFile(configuration.RuntimeDocumentPath)
	if errorValue != nil {
		return false, fmt.Errorf("reading who pays for models from the runtime document the preparation wrote: %w", errorValue)
	}
	var runtime struct {
		LanguageModel modelladder.Document `json:"languageModel"`
	}
	if errorValue := json.Unmarshal(document, &runtime); errorValue != nil {
		return false, fmt.Errorf("reading who pays for models from %s: %w", configuration.RuntimeDocumentPath, errorValue)
	}
	return modelladder.PaysThroughGatewayIn(runtime.LanguageModel, configuration.CentralPlaneURL), nil
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
