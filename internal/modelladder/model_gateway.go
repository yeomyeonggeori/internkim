package modelladder

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	modelGatewayFunctionPath = "/functions/v1/model-gateway"
	ModelGatewayPayerPath    = modelGatewayFunctionPath + "/payer"
)

const RequesterEmailHeader = "X-INTERNKIM-REQUESTER-EMAIL"

type CompanyKeyUse func(isKeyHeld bool) (bool, error)

var errNoModelRoute = errors.New("the ladder reaches the model route on this computer, and no model route was named")

// GatewayOrigin is the root the model gateway answers OpenRouter's own paths
// under.
func GatewayOrigin(centralPlaneURL string) string {
	return planeRootOf(centralPlaneURL) + modelGatewayFunctionPath
}

func OpenRouterOrigin() string {
	return apiRootOf(Endpoint)
}

// ModelRouteEndpoint is the OpenAI-compatible root of the route capabilityd
// answers on loopback, which sends each call to whoever pays at that moment.
func ModelRouteEndpoint(modelRouteURL string) string {
	return strings.TrimRight(strings.TrimSpace(modelRouteURL), "/") + chatAPIPath
}

func planeRootOf(centralPlaneURL string) string {
	return strings.TrimRight(strings.TrimSpace(centralPlaneURL), "/")
}

func PaysThroughGateway(centralPlaneURL string, companyKeyPath string, reportingTheKey CompanyKeyUse) (bool, error) {
	if planeRootOf(centralPlaneURL) == "" {
		return false, nil
	}
	isKeyHeld := holdsKey(companyKeyPath)
	isInUse, errorValue := reportingTheKey(isKeyHeld)
	if !isKeyHeld {
		return true, nil
	}
	if errorValue != nil {
		return false, fmt.Errorf("this computer holds the company's OpenRouter key and could not learn whether the company uses it: %w", errorValue)
	}
	return !isInUse, nil
}

func holdsKey(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	document, errorValue := os.ReadFile(path)
	return errorValue == nil && strings.TrimSpace(string(document)) != ""
}

// GatewaySaysWhetherTheCompanyKeyIsInUse asks the plane once per call, and the
// asking is also the report the setup page reads the payer from.
func GatewaySaysWhetherTheCompanyKeyIsInUse(centralPlaneURL string, credentialPath string) CompanyKeyUse {
	payerURL := planeRootOf(centralPlaneURL) + ModelGatewayPayerPath
	return func(isKeyHeld bool) (bool, error) {
		return reportedTheKey(payerURL, credentialPath, isKeyHeld)
	}
}

func reportedTheKey(payerURL string, credentialPath string, isKeyHeld bool) (bool, error) {
	credential, errorValue := os.ReadFile(credentialPath)
	if errorValue != nil {
		return false, fmt.Errorf("reading this computer's credential: %w", errorValue)
	}
	body, errorValue := json.Marshal(map[string]bool{"holdsModelKey": isKeyHeld})
	if errorValue != nil {
		return false, errorValue
	}
	request, errorValue := http.NewRequest(http.MethodPost, payerURL, bytes.NewReader(body))
	if errorValue != nil {
		return false, errorValue
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(credential)))
	request.Header.Set("Content-Type", "application/json")
	response, errorValue := (&http.Client{Timeout: 15 * time.Second}).Do(request)
	if errorValue != nil {
		return false, fmt.Errorf("asking %s: %w", payerURL, errorValue)
	}
	defer response.Body.Close()
	return usesOwnModelKeyIn(payerURL, response)
}

func usesOwnModelKeyIn(payerURL string, response *http.Response) (bool, error) {
	if response.StatusCode >= 400 && response.StatusCode < 500 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return false, fmt.Errorf("the central plane refused this computer: %s answered %s: %s", payerURL, response.Status, strings.TrimSpace(string(body)))
	}
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return false, fmt.Errorf("%s answered %s: %s", payerURL, response.Status, strings.TrimSpace(string(body)))
	}
	var answer struct {
		UsesOwnModelKey *bool `json:"usesOwnModelKey"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&answer); errorValue != nil {
		return false, fmt.Errorf("reading what %s answered: %w", payerURL, errorValue)
	}
	if answer.UsesOwnModelKey == nil {
		return false, fmt.Errorf("%s answered no usesOwnModelKey", payerURL)
	}
	return *answer.UsesOwnModelKey, nil
}

// CompanyLanguageModelDocument is the ladder the agent starts with. A company
// on a central plane reaches its models through capabilityd's model route,
// which follows the payer while the services run; the route takes the service
// token, and names the requester to the gateway only.
func CompanyLanguageModelDocument(centralPlaneURL string, modelRouteURL string, modelRouteKeyPath string, companyEndpoint string, companyKeyPath string) (Document, error) {
	if runsItsOwnModels(companyEndpoint) || planeRootOf(centralPlaneURL) == "" {
		return LanguageModelDocument(companyEndpoint, companyKeyPath), nil
	}
	if strings.TrimSpace(modelRouteURL) == "" {
		return Document{}, errNoModelRoute
	}
	return requesterNamingLanguageModelDocument(ModelRouteEndpoint(modelRouteURL), modelRouteKeyPath), nil
}

func runsItsOwnModels(companyEndpoint string) bool {
	endpoint := strings.TrimRight(strings.TrimSpace(companyEndpoint), "/")
	return endpoint != "" && endpoint != Endpoint
}

func requesterNamingLanguageModelDocument(endpoint string, keyPath string) Document {
	document := LanguageModelDocument(endpoint, keyPath)
	for _, rungs := range document.Tiers {
		for index := range rungs {
			rungs[index].RequesterEmailHeader = RequesterEmailHeader
		}
	}
	document.Decision.RequesterEmailHeader = RequesterEmailHeader
	document.VisualReview.RequesterEmailHeader = RequesterEmailHeader
	return document
}
