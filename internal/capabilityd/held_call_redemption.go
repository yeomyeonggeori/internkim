package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

const (
	heldCallRedemptionPath    = "/admin/api/held-call/redeem"
	heldCallRedemptionTimeout = 10 * time.Second
)

type heldCallRedemption struct {
	TaskRunID         string          `json:"taskRunID"`
	HoldID            string          `json:"holdID"`
	RequesterPersonID string          `json:"requesterPersonID"`
	ToolName          string          `json:"toolName"`
	ToolInput         json.RawMessage `json:"toolInput"`
}

func heldCallRedemptionOf(request capabilities.ToolInvokeRequest) heldCallRedemption {
	return heldCallRedemption{
		TaskRunID:         strings.TrimSpace(request.Context.TaskRunID),
		HoldID:            strings.TrimSpace(request.Context.HoldID),
		RequesterPersonID: request.Context.RequesterPersonID,
		ToolName:          strings.TrimSpace(request.ToolName),
		ToolInput:         json.RawMessage(emptyObjectIfBlank(request.Input)),
	}
}

func (service Service) redeemHeldCall(ctx context.Context, request capabilities.ToolInvokeRequest) error {
	document, errorValue := json.Marshal(heldCallRedemptionOf(request))
	if errorValue != nil {
		return errorValue
	}
	ctx, cancel := context.WithTimeout(ctx, heldCallRedemptionTimeout)
	defer cancel()
	ledgerURL := strings.TrimRight(service.Configuration.WithDefaults().BlueclawBaseURL, "/") + heldCallRedemptionPath
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, ledgerURL, bytes.NewReader(document))
	if errorValue != nil {
		return errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, errorValue := service.httpClient().Do(httpRequest)
	if errorValue != nil {
		return fmt.Errorf("the ledger that holds the approval could not be asked: %w", errorValue)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusOK {
		return nil
	}
	return fmt.Errorf("the ledger refused the approval: %s", refusalReasonOf(response))
}

func refusalReasonOf(response *http.Response) string {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	var refusal struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &refusal) == nil && strings.TrimSpace(refusal.Error) != "" {
		return refusal.Error
	}
	return fmt.Sprintf("%d %s", response.StatusCode, strings.TrimSpace(string(body)))
}
