package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (service *Service) centralPlaneAgentKey() string {
	if stored := readTrimmedFile(service.Configuration.CentralPlaneAgentKeyPath); stored != "" {
		return stored
	}
	issued, errorValue := service.askCentralPlaneForAgentKey()
	if errorValue != nil {
		return ""
	}
	if errorValue := writeAgentKeyFile(service.Configuration.CentralPlaneAgentKeyPath, issued); errorValue != nil {
		return ""
	}
	return issued
}

func (service *Service) askCentralPlaneForAgentKey() (string, error) {
	fleetID := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath))
	fleetSecret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	appURL := strings.TrimRight(strings.TrimSpace(service.Configuration.CentralPlaneAppURL), "/")
	if fleetID == "" || fleetSecret == "" || appURL == "" {
		return "", fmt.Errorf("this device cannot name itself to the central plane")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	request, errorValue := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		appURL+"/api/agent/key?fleet_id="+url.QueryEscape(fleetID),
		nil,
	)
	if errorValue != nil {
		return "", errorValue
	}
	request.Header.Set("X-InternKim-Fleet-ID", fleetID)
	request.Header.Set("X-InternKim-Fleet-Secret", fleetSecret)

	client := service.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	response, errorValue := client.Do(request)
	if errorValue != nil {
		return "", errorValue
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("the central plane answered %s", response.Status)
	}

	var issued struct {
		APIKey string `json:"apiKey"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&issued); errorValue != nil {
		return "", errorValue
	}
	if strings.TrimSpace(issued.APIKey) == "" {
		return "", fmt.Errorf("the central plane issued an empty key")
	}
	return strings.TrimSpace(issued.APIKey), nil
}

func writeAgentKeyFile(path string, key string) error {
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return errorValue
	}
	return os.WriteFile(path, []byte(key+"\n"), 0o600)
}
