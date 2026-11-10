package modelladder

import (
	"os"
	"strings"
)

const modelGatewayPath = "/functions/v1/model-gateway" + chatAPIPath

const RequesterEmailHeader = "X-INTERNKIM-REQUESTER-EMAIL"

func GatewayEndpoint(centralPlaneURL string) string {
	return strings.TrimRight(strings.TrimSpace(centralPlaneURL), "/") + modelGatewayPath
}

func PaysThroughGateway(companyKeyPath string, centralPlaneURL string) bool {
	return strings.TrimSpace(centralPlaneURL) != "" && !holdsKey(companyKeyPath)
}

func holdsKey(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	document, errorValue := os.ReadFile(path)
	return errorValue == nil && strings.TrimSpace(string(document)) != ""
}

func CompanyLanguageModelDocument(centralPlaneURL string, gatewayKeyPath string, companyEndpoint string, companyKeyPath string) Document {
	companyDocument := LanguageModelDocument(companyEndpoint, companyKeyPath)
	if strings.TrimSpace(centralPlaneURL) == "" || runsItsOwnModels(companyEndpoint) {
		return companyDocument
	}
	gatewayDocument := gatewayLanguageModelDocument(centralPlaneURL, gatewayKeyPath)
	if !holdsKey(companyKeyPath) {
		return gatewayDocument
	}
	for tier, rungs := range gatewayDocument.Tiers {
		gatewayDocument.Tiers[tier] = append(rungs, companyDocument.Tiers[tier]...)
	}
	gatewayDocument.Decision = companyDocument.Decision
	gatewayDocument.VisualReview = companyDocument.VisualReview
	return gatewayDocument
}

func runsItsOwnModels(companyEndpoint string) bool {
	endpoint := strings.TrimRight(strings.TrimSpace(companyEndpoint), "/")
	return endpoint != "" && endpoint != Endpoint
}

func gatewayLanguageModelDocument(centralPlaneURL string, gatewayKeyPath string) Document {
	document := LanguageModelDocument(GatewayEndpoint(centralPlaneURL), gatewayKeyPath)
	for _, rungs := range document.Tiers {
		for index := range rungs {
			rungs[index].RequesterEmailHeader = RequesterEmailHeader
		}
	}
	document.Decision.RequesterEmailHeader = RequesterEmailHeader
	document.VisualReview.RequesterEmailHeader = RequesterEmailHeader
	return document
}
