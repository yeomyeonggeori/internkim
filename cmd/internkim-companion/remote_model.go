package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"strings"

	companionruntime "gitlab.com/eastriver/internkim/internal/companion"
)

type remoteModelResponse struct {
	Model       string `json:"model"`
	Restarted   bool   `json:"restarted,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
	RuntimePath string `json:"runtimePath,omitempty"`
}

func runRuntimeModel(arguments []string, httpClient *http.Client, secureStore companionruntime.SecureStore) error {
	if len(arguments) == 0 {
		return errors.New("usage: runtime-model <get|set>")
	}
	switch arguments[0] {
	case "get":
		return runRuntimeModelGet(arguments[1:], httpClient, secureStore)
	case "set":
		return runRuntimeModelSet(arguments[1:], httpClient, secureStore)
	default:
		return errors.New("usage: runtime-model <get|set>")
	}
}

func runRuntimeModelGet(arguments []string, httpClient *http.Client, secureStore companionruntime.SecureStore) error {
	flags := flag.NewFlagSet("runtime-model get", flag.ContinueOnError)
	statePath := flags.String("state", defaultStatePath(), "companion state path")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return errorValue
	}
	state, privateKey, errorValue := loadCompanionStateAndPrivateKey(*statePath, secureStore)
	if errorValue != nil {
		return errorValue
	}
	var response remoteModelResponse
	if errorValue := signedJSONRequest(httpClient, state, privateKey, http.MethodGet, runtimeRemoteModelEndpoint(state), nil, &response); errorValue != nil {
		return errorValue
	}
	fmt.Println(response.Model)
	return nil
}

func runRuntimeModelSet(arguments []string, httpClient *http.Client, secureStore companionruntime.SecureStore) error {
	flags := flag.NewFlagSet("runtime-model set", flag.ContinueOnError)
	statePath := flags.String("state", defaultStatePath(), "companion state path")
	model := flags.String("model", "", "remote model ID")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return errorValue
	}
	if flags.NArg() > 0 && strings.TrimSpace(*model) == "" {
		*model = flags.Arg(0)
	}
	trimmedModel := strings.TrimSpace(*model)
	if trimmedModel == "" {
		return errors.New("remote model is required")
	}
	state, privateKey, errorValue := loadCompanionStateAndPrivateKey(*statePath, secureStore)
	if errorValue != nil {
		return errorValue
	}
	var response remoteModelResponse
	body := map[string]string{"model": trimmedModel}
	if errorValue := signedJSONRequest(httpClient, state, privateKey, http.MethodPut, runtimeRemoteModelEndpoint(state), body, &response); errorValue != nil {
		return errorValue
	}
	fmt.Println(response.Model)
	return nil
}

func runtimeRemoteModelEndpoint(state companionState) string {
	return state.DeviceURL + "/_internkim/runtime/remote-model"
}

func loadCompanionStateAndPrivateKey(statePath string, secureStore companionruntime.SecureStore) (companionState, string, error) {
	state, errorValue := loadStateAndMigrateSecrets(context.Background(), statePath, secureStore)
	if errorValue != nil {
		return companionState{}, "", errorValue
	}
	privateKey, errorValue := secureStore.Get(context.Background(), state.PrivateKeyID)
	if errorValue != nil {
		return companionState{}, "", fmt.Errorf("%s signing key missing: %w", companionPairingExpiredMessage, errorValue)
	}
	return state, privateKey, nil
}

func signedJSONRequest(httpClient *http.Client, state companionState, privateKey string, method string, endpoint string, requestBody any, responseBody any) error {
	var document []byte
	var errorValue error
	if requestBody != nil {
		document, errorValue = json.Marshal(requestBody)
		if errorValue != nil {
			return errorValue
		}
	}
	request, errorValue := http.NewRequest(method, endpoint, bytes.NewReader(document))
	if errorValue != nil {
		return errorValue
	}
	if requestBody != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range companionHeaders(state) {
		request.Header.Set(key, value)
	}
	if errorValue := companionruntime.SignRequest(request, document, privateKey); errorValue != nil {
		return errorValue
	}
	response, errorValue := httpClient.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	return decodeJSONResponse(endpoint, response, responseBody)
}
