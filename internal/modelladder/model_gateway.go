package modelladder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

const (
	firstWaitForThePayer   = time.Second
	longestWaitForThePayer = time.Minute
)

const RequesterEmailHeader = "X-INTERNKIM-REQUESTER-EMAIL"

type CompanyKeyUse func(isKeyHeld bool) (bool, error)

func GatewayEndpoint(centralPlaneURL string) string {
	return planeRootOf(centralPlaneURL) + modelGatewayPath
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

func GatewaySaysWhetherTheCompanyKeyIsInUse(centralPlaneURL string, credentialPath string) CompanyKeyUse {
	payerURL := planeRootOf(centralPlaneURL) + ModelGatewayPayerPath
	return func(isKeyHeld bool) (bool, error) {
		report := func() (bool, error) { return reportedTheKey(payerURL, credentialPath, isKeyHeld) }
		if !isKeyHeld {
			return reportedOnce(report)
		}
		return answeredEventually(report, time.Sleep)
	}
}

func reportedOnce(report func() (bool, error)) (bool, error) {
	if _, errorValue := report(); errorValue != nil {
		log.Printf("telling the central plane this computer holds no OpenRouter key: %v; it pays through the gateway all the same", errorValue)
	}
	return false, nil
}

func answeredEventually(report func() (bool, error), wait func(time.Duration)) (bool, error) {
	pause := firstWaitForThePayer
	for {
		isInUse, errorValue := report()
		if errorValue == nil {
			return isInUse, nil
		}
		log.Printf("asking the central plane whether the company uses the OpenRouter key this computer holds: %v; asking again in %s", errorValue, pause)
		wait(pause)
		pause = min(2*pause, longestWaitForThePayer)
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

func CompanyLanguageModelDocument(centralPlaneURL string, gatewayKeyPath string, companyEndpoint string, companyKeyPath string, reportingTheKey CompanyKeyUse) (Document, error) {
	companyDocument := LanguageModelDocument(companyEndpoint, companyKeyPath)
	if runsItsOwnModels(companyEndpoint) {
		return companyDocument, nil
	}
	paysThroughGateway, errorValue := PaysThroughGateway(centralPlaneURL, companyKeyPath, reportingTheKey)
	if errorValue != nil || !paysThroughGateway {
		return companyDocument, errorValue
	}
	return gatewayLanguageModelDocument(centralPlaneURL, gatewayKeyPath), nil
}

func PaysThroughGatewayIn(languageModel Document, centralPlaneURL string) bool {
	if planeRootOf(centralPlaneURL) == "" {
		return false
	}
	gateway := GatewayEndpoint(centralPlaneURL)
	for _, rungs := range languageModel.Tiers {
		for _, rung := range rungs {
			if rung.Endpoint == gateway {
				return true
			}
		}
	}
	return false
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
