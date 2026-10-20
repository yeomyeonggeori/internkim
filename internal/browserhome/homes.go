package browserhome

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

var ErrBoxAway = errors.New("the box that holds this person's browser is not connected")

type Homes struct {
	ThisBox    string            `json:"thisBox"`
	GatewayURL string            `json:"gatewayURL"`
	CompanyID  string            `json:"companyID"`
	Members    map[string]string `json:"members"`
}

func (homes Homes) BoxOf(memberEmail string) (string, bool) {
	box, isPlaced := homes.Members[strings.ToLower(strings.TrimSpace(memberEmail))]
	if !isPlaced || box == homes.ThisBox {
		return "", false
	}
	return box, true
}

func Write(path string, homes Homes) error {
	if len(homes.Members) == 0 {
		return Forget(path)
	}
	document, errorValue := json.Marshal(homes)
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

func Read(path string) (Homes, bool) {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return Homes{}, false
	}
	var homes Homes
	if json.Unmarshal(document, &homes) != nil || homes.ThisBox == "" || homes.GatewayURL == "" || homes.CompanyID == "" {
		return Homes{}, false
	}
	return homes, true
}

type Hand struct {
	HTTPClient *http.Client
	Token      func() (string, error)
}

type answered struct {
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

func (hand Hand) Give(ctx context.Context, homes Homes, box string, requestID string, work json.RawMessage) (json.RawMessage, error) {
	token, errorValue := hand.Token()
	if errorValue != nil {
		return nil, fmt.Errorf("reading this box's host session: %w", errorValue)
	}
	body, errorValue := json.Marshal(map[string]any{"boxKey": box, "requestID": requestID, "work": work})
	if errorValue != nil {
		return nil, errorValue
	}
	address := HTTPAddressOf(homes.GatewayURL) + "/company/" + homes.CompanyID + "/worker-call"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, address, bytes.NewReader(body))
	if errorValue != nil {
		return nil, errorValue
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, errorValue := hand.httpClient().Do(request)
	if errorValue != nil {
		return nil, fmt.Errorf("handing work to box %s: %w", box, errorValue)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusServiceUnavailable {
		return nil, ErrBoxAway
	}
	if response.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return nil, fmt.Errorf("the gateway answered %d handing work to box %s: %s", response.StatusCode, box, strings.TrimSpace(string(detail)))
	}
	var answer answered
	if errorValue := json.NewDecoder(response.Body).Decode(&answer); errorValue != nil {
		return nil, fmt.Errorf("the answer from box %s: %w", box, errorValue)
	}
	if answer.Status != http.StatusOK {
		return nil, fmt.Errorf("box %s took the work and answered %d: %s", box, answer.Status, answer.Body)
	}
	return answer.Body, nil
}

func (hand Hand) httpClient() *http.Client {
	if hand.HTTPClient == nil {
		return &http.Client{Timeout: 3 * time.Minute}
	}
	return hand.HTTPClient
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
