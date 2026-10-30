package modelladder

import (
	"os"
	"strings"
)

const modelGatewayPath = "/functions/v1/model-gateway" + chatAPIPath

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
