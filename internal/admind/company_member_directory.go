package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (service *Service) deviceBelongsToACompany() bool {
	return readTrimmedFile(service.Configuration.CentralPlaneAgentKeyPath) != ""
}

func (service *Service) companyMemberRecords(ctx context.Context) ([]adminUserMutation, error) {
	appURL := strings.TrimRight(strings.TrimSpace(service.Configuration.CentralPlaneAppURL), "/")
	if appURL == "" {
		return nil, fmt.Errorf("this device holds a company key but no company app address")
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, appURL+"/api/agent/member", nil)
	if errorValue != nil {
		return nil, errorValue
	}
	request.Header.Set("Authorization", "Bearer "+readTrimmedFile(service.Configuration.CentralPlaneAgentKeyPath))
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return nil, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		document, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("company member lookup returned %d: %s", response.StatusCode, strings.TrimSpace(string(document)))
	}
	var membersResponse struct {
		Members []adminUserMutation `json:"members"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&membersResponse); errorValue != nil {
		return nil, errorValue
	}
	return membersResponse.Members, nil
}
