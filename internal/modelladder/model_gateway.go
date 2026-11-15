package modelladder

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	modelGatewayFunctionPath = "/functions/v1/model-gateway"
	modelGatewayPath         = modelGatewayFunctionPath + chatAPIPath
	ModelGatewayPayerPath    = modelGatewayFunctionPath + "/payer"
)

const RequesterEmailHeader = "X-INTERNKIM-REQUESTER-EMAIL"

type CompanyKeyUse func() (bool, error)

func GatewayEndpoint(centralPlaneURL string) string {
	return planeRootOf(centralPlaneURL) + modelGatewayPath
}

func planeRootOf(centralPlaneURL string) string {
	return strings.TrimRight(strings.TrimSpace(centralPlaneURL), "/")
}

func PaysThroughGateway(centralPlaneURL string, companyKeyPath string, isCompanyKeyInUse CompanyKeyUse) (bool, error) {
	if planeRootOf(centralPlaneURL) == "" {
		return false, nil
	}
	if !holdsKey(companyKeyPath) {
		return true, nil
	}
	isInUse, errorValue := isCompanyKeyInUse()
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

func GatewaySaysWhetherTheCompanyKeyIsInUse(centralPlaneURL string, credentialPath string) CompanyKeyUse {
	return func() (bool, error) {
		credential, errorValue := os.ReadFile(credentialPath)
		if errorValue != nil {
			return false, fmt.Errorf("reading this computer's credential: %w", errorValue)
		}
		return askedWhetherTheCompanyKeyIsInUse(planeRootOf(centralPlaneURL)+ModelGatewayPayerPath, strings.TrimSpace(string(credential)))
	}
}

func askedWhetherTheCompanyKeyIsInUse(payerURL string, credential string) (bool, error) {
	request, errorValue := http.NewRequest(http.MethodGet, payerURL, nil)
	if errorValue != nil {
		return false, errorValue
	}
	request.Header.Set("Authorization", "Bearer "+credential)
	response, errorValue := (&http.Client{Timeout: 15 * time.Second}).Do(request)
	if errorValue != nil {
		return false, fmt.Errorf("asking %s: %w", payerURL, errorValue)
	}
	defer response.Body.Close()
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

func CompanyLanguageModelDocument(centralPlaneURL string, gatewayKeyPath string, companyEndpoint string, companyKeyPath string, isCompanyKeyInUse CompanyKeyUse) (Document, error) {
	companyDocument := LanguageModelDocument(companyEndpoint, companyKeyPath)
	if runsItsOwnModels(companyEndpoint) {
		return companyDocument, nil
	}
	paysThroughGateway, errorValue := PaysThroughGateway(centralPlaneURL, companyKeyPath, isCompanyKeyInUse)
	if errorValue != nil || !paysThroughGateway {
		return companyDocument, errorValue
	}
	return gatewayLanguageModelDocument(centralPlaneURL, gatewayKeyPath), nil
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
