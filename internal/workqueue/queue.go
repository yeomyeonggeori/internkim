package workqueue

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var ErrNoBoxHadRoom = errors.New("no box had room for this work")

const noBoxHadRoomStatus = http.StatusServiceUnavailable

type Address struct {
	GatewayURL string `json:"gatewayURL"`
	CompanyID  string `json:"companyID"`
}

func Write(path string, address Address) error {
	document, errorValue := json.Marshal(address)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o755); errorValue != nil {
		return errorValue
	}
	partialPath := path + ".partial"
	if errorValue := os.WriteFile(partialPath, document, 0o600); errorValue != nil {
		return errorValue
	}
	return os.Rename(partialPath, path)
}

func Forget(path string) error {
	errorValue := os.Remove(path)
	if errors.Is(errorValue, fs.ErrNotExist) {
		return nil
	}
	return errorValue
}

func Read(path string) (Address, bool) {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return Address{}, false
	}
	var address Address
	if json.Unmarshal(document, &address) != nil || address.GatewayURL == "" || address.CompanyID == "" {
		return Address{}, false
	}
	return address, true
}

type Queue struct {
	HTTPClient *http.Client
	Token      func() (string, error)
}

type answered struct {
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

func (queue Queue) Post(ctx context.Context, address Address, requestID string, affinityKey string, work json.RawMessage) (json.RawMessage, error) {
	token, errorValue := queue.Token()
	if errorValue != nil {
		return nil, fmt.Errorf("reading this box's host session: %w", errorValue)
	}
	body, errorValue := json.Marshal(map[string]any{"requestID": requestID, "affinityKey": affinityKey, "work": work})
	if errorValue != nil {
		return nil, errorValue
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, HTTPAddressOf(address.GatewayURL)+"/company/"+address.CompanyID+"/work", bytes.NewReader(body))
	if errorValue != nil {
		return nil, errorValue
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, errorValue := queue.httpClient().Do(request)
	if errorValue != nil {
		return nil, fmt.Errorf("putting work in the company queue: %w", errorValue)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return nil, fmt.Errorf("the gateway answered %d to work put in the queue: %s", response.StatusCode, strings.TrimSpace(string(detail)))
	}
	var answer answered
	if errorValue := json.NewDecoder(response.Body).Decode(&answer); errorValue != nil {
		return nil, fmt.Errorf("the answer to queued work: %w", errorValue)
	}
	if answer.Status == noBoxHadRoomStatus {
		return nil, ErrNoBoxHadRoom
	}
	if answer.Status != http.StatusOK {
		return nil, fmt.Errorf("the box that took the work answered %d: %s", answer.Status, answer.Body)
	}
	return answer.Body, nil
}

func (queue Queue) httpClient() *http.Client {
	if queue.HTTPClient == nil {
		return &http.Client{Timeout: 3 * time.Minute}
	}
	return queue.HTTPClient
}

func HTTPAddressOf(gatewayURL string) string {
	trimmed := strings.TrimRight(gatewayURL, "/")
	if rest, isSocket := strings.CutPrefix(trimmed, "wss://"); isSocket {
		return "https://" + rest
	}
	if rest, isSocket := strings.CutPrefix(trimmed, "ws://"); isSocket {
		return "http://" + rest
	}
	return trimmed
}

func SocketAddressOf(gatewayURL string) string {
	trimmed := strings.TrimRight(gatewayURL, "/")
	if rest, isPlain := strings.CutPrefix(trimmed, "https://"); isPlain {
		return "wss://" + rest
	}
	if rest, isPlain := strings.CutPrefix(trimmed, "http://"); isPlain {
		return "ws://" + rest
	}
	return trimmed
}
