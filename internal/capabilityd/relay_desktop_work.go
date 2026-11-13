package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/desktop"
)

const (
	relayDesktopWorkPath          = "/desktop-work"
	relayHasNoGatewayStatus       = http.StatusNotImplemented
	noDesktopTookTheWorkStatus    = http.StatusServiceUnavailable
	desktopWorkPatience           = 150 * time.Second
	desktopWorkNotSentConsequence = "so the desktop work was not sent and whether a computer of the requester's is taking desktop work was not asked"
)

var (
	errNoDesktopTookTheWork         = errors.New("no desktop of the requester's took the work")
	errDesktopUnavailableOnThisHost = errors.New("desktop_unavailable_on_this_host")
)

type desktopWork struct {
	RequestID    string       `json:"requestID"`
	DesktopOwner string       `json:"desktopOwner"`
	Work         desktop.Call `json:"work"`
}

type gatewayAnswer struct {
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

func (service Service) handDesktopWorkToTheRelay(ctx context.Context, work desktopWork) (json.RawMessage, error) {
	answered, errorValue := service.postToTheRelay(ctx, work)
	if errorValue != nil {
		return nil, errorValue
	}
	if answered.Status == noDesktopTookTheWorkStatus {
		return nil, errNoDesktopTookTheWork
	}
	if answered.Status != http.StatusOK {
		return nil, fmt.Errorf("the gateway answered %d to desktop work: %s", answered.Status, answered.Body)
	}
	return answered.Body, nil
}

func (service Service) postToTheRelay(ctx context.Context, work desktopWork) (gatewayAnswer, error) {
	document, errorValue := json.Marshal(work)
	if errorValue != nil {
		return gatewayAnswer{}, errorValue
	}
	address := strings.TrimRight(service.Configuration.WithDefaults().RelayBaseURL, "/") + relayDesktopWorkPath
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, address, bytes.NewReader(document))
	if errorValue != nil {
		return gatewayAnswer{}, errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	response, errorValue := service.desktopWorkClient().Do(request)
	if isUnanswered(errorValue) {
		return gatewayAnswer{}, fmt.Errorf("%w: the relay at %s did not answer (%v), %s", errDesktopUnavailableOnThisHost, address, errorValue, desktopWorkNotSentConsequence)
	}
	if errorValue != nil {
		return gatewayAnswer{}, fmt.Errorf("handing desktop work to the relay at %s: %w", address, errorValue)
	}
	defer response.Body.Close()
	if response.StatusCode == relayHasNoGatewayStatus {
		return gatewayAnswer{}, fmt.Errorf("%w: the relay at %s has no gateway to hand desktop work to, %s", errDesktopUnavailableOnThisHost, address, desktopWorkNotSentConsequence)
	}
	if response.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return gatewayAnswer{}, fmt.Errorf("the relay at %s answered %d to desktop work: %s", address, response.StatusCode, strings.TrimSpace(string(detail)))
	}
	var answered gatewayAnswer
	if errorValue := json.NewDecoder(response.Body).Decode(&answered); errorValue != nil {
		return gatewayAnswer{}, fmt.Errorf("the answer to desktop work: %w", errorValue)
	}
	return answered, nil
}

func (service Service) desktopWorkClient() *http.Client {
	client := *service.httpClient()
	client.Timeout = max(client.Timeout, desktopWorkPatience)
	return &client
}

func isUnanswered(errorValue error) bool {
	var operation *net.OpError
	return errors.As(errorValue, &operation) && operation.Op == "dial"
}
