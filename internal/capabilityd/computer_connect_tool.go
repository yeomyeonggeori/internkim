package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

const computerConnectToolName = "computer_connect"

type companionPairingCodeOwner struct {
	OwnerPlatform       string `json:"ownerPlatform,omitempty"`
	OwnerPlatformUserID string `json:"ownerPlatformUserID,omitempty"`
	OwnerPersonID       string `json:"ownerPersonID,omitempty"`
	OwnerEmail          string `json:"ownerEmail,omitempty"`
	OwnerName           string `json:"ownerName,omitempty"`
}

type companionPairingCode struct {
	Code        string    `json:"code"`
	ExpiresAt   time.Time `json:"expiresAt"`
	PairCommand string    `json:"pairCommand"`
}

type computerConnectResult struct {
	Code             string `json:"code"`
	ExpiresAt        string `json:"expiresAt"`
	InstallCommand   string `json:"installCommand"`
	PairCommand      string `json:"pairCommand"`
	ServiceCommand   string `json:"serviceCommand"`
	DocumentationURL string `json:"documentationURL"`
}

func (service Service) invokeComputerConnectTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	owner := companionPairingCodeOwnerOf(request.Context)
	if owner.isNobody() {
		return capabilities.ToolInvokeResponse{}, errors.New("computer_connect pairs a companion with the person who asked, and this call named nobody")
	}
	pairingCode, errorValue := service.issueCompanionPairingCode(ctx, owner)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	result, errorValue := json.Marshal(computerConnectResult{
		Code:             pairingCode.Code,
		ExpiresAt:        pairingCode.ExpiresAt.UTC().Format(time.RFC3339),
		InstallCommand:   capabilities.CompanionInstallCommand(),
		PairCommand:      pairingCode.PairCommand,
		ServiceCommand:   capabilities.CompanionServiceCommand(),
		DocumentationURL: capabilities.CompanionInstallURL(),
	})
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilitySuccessResponse(request.ToolName, "ok", result)
}

func companionPairingCodeOwnerOf(invokeContext capabilityprotocol.ToolInvokeContext) companionPairingCodeOwner {
	return companionPairingCodeOwner{
		OwnerPlatform:       strings.TrimSpace(invokeContext.Platform),
		OwnerPlatformUserID: strings.TrimSpace(invokeContext.RequesterPlatformUserID),
		OwnerPersonID:       strings.TrimSpace(invokeContext.RequesterPersonID),
		OwnerEmail:          strings.ToLower(strings.TrimSpace(invokeContext.RequesterEmail)),
		OwnerName:           strings.TrimSpace(invokeContext.RequesterName),
	}
}

func (owner companionPairingCodeOwner) isNobody() bool {
	return owner.OwnerEmail == "" && owner.OwnerPersonID == "" && owner.OwnerPlatformUserID == ""
}

func (service Service) issueCompanionPairingCode(ctx context.Context, owner companionPairingCodeOwner) (companionPairingCode, error) {
	body, errorValue := json.Marshal(owner)
	if errorValue != nil {
		return companionPairingCode{}, errorValue
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	endpoint := strings.TrimRight(service.Configuration.AdmindBaseURL, "/") + "/_internkim/companion/pairing-codes"
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if errorValue != nil {
		return companionPairingCode{}, errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpResponse, errorValue := service.httpClient().Do(httpRequest)
	if errorValue != nil {
		return companionPairingCode{}, errorValue
	}
	defer httpResponse.Body.Close()
	answer, errorValue := io.ReadAll(httpResponse.Body)
	if errorValue != nil {
		return companionPairingCode{}, errorValue
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return companionPairingCode{}, fmt.Errorf("the device refused to issue a pairing code: %s", strings.TrimSpace(string(answer)))
	}
	var pairingCode companionPairingCode
	if errorValue := json.Unmarshal(answer, &pairingCode); errorValue != nil {
		return companionPairingCode{}, errorValue
	}
	return pairingCode, nil
}
