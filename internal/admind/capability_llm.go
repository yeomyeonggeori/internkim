package admind

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	blueclawruntime "github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func (service *Service) callCapabilityLLM(ctx context.Context, path string, requestDocument []byte) ([]byte, error) {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			_ = network
			_ = address
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", blueclawruntime.CapabilitySocketPath)
		},
	}
	client := http.Client{Transport: transport}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, "http://internkim"+path, bytes.NewReader(requestDocument))
	if errorValue != nil {
		return nil, errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	response, errorValue := client.Do(request)
	if errorValue != nil {
		return nil, errorValue
	}
	defer response.Body.Close()
	body, readError := io.ReadAll(response.Body)
	if readError != nil {
		return nil, readError
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("capabilityd %s failed: %s", path, strings.TrimSpace(string(body)))
	}
	return body, nil
}
