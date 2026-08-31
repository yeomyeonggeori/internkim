package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const centralTaskAnswerSource = "central"

type taskStateAnswer struct {
	Source string `json:"source"`
}

// A scenario proves what the product does for a company, so a fleet answering
// out of the device's own store proves nothing. The board says which it read.
func refuseAFleetWithoutACompany(contextValue context.Context, adminURL string) error {
	source, errorValue := taskAnswerSourceOf(contextValue, adminURL)
	if errorValue != nil {
		return fmt.Errorf("the fleet did not say where its board comes from: %w", errorValue)
	}
	if source != centralTaskAnswerSource {
		return fmt.Errorf(
			"the fleet reads its board from %q rather than the company's record; "+
				"provisioning gives the device a company, and an expensive scenario against %q proves nothing about the product",
			source, source)
	}
	return nil
}

func taskAnswerSourceOf(contextValue context.Context, adminURL string) (string, error) {
	address := strings.TrimRight(strings.TrimSpace(adminURL), "/") + "/task/api/state"
	request, errorValue := http.NewRequestWithContext(contextValue, http.MethodGet, address, nil)
	if errorValue != nil {
		return "", errorValue
	}
	client := http.Client{Timeout: 30 * time.Second}
	response, errorValue := client.Do(request)
	if errorValue != nil {
		return "", errorValue
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("the board answered %d", response.StatusCode)
	}
	body, errorValue := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if errorValue != nil {
		return "", errorValue
	}
	var answer taskStateAnswer
	if errorValue := json.Unmarshal(body, &answer); errorValue != nil {
		return "", errorValue
	}
	source := strings.TrimSpace(answer.Source)
	if source == "" {
		return "", errors.New("the board named no source")
	}
	return source, nil
}
