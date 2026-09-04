package centralplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type MCPAnswer struct {
	Status int
	Body   json.RawMessage
}

func (client *Client) CarryMCPMessage(
	ctx context.Context,
	requesterEmail string,
	message json.RawMessage,
) (MCPAnswer, error) {
	email := strings.ToLower(strings.TrimSpace(requesterEmail))
	if email == "" {
		return MCPAnswer{}, fmt.Errorf("an MCP message runs as the person who asked, and this one named nobody")
	}

	session, errorValue := client.sessionFor(ctx, "email", email)
	if errorValue != nil {
		return MCPAnswer{}, errorValue
	}

	address := strings.TrimSuffix(client.settings.AppURL, "/") + "/api/v1/mcp"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, address, bytes.NewReader(message))
	if errorValue != nil {
		return MCPAnswer{}, errorValue
	}
	request.Header.Set("Authorization", "Bearer "+session.accessToken)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return MCPAnswer{}, errorValue
	}
	defer response.Body.Close()

	body, errorValue := io.ReadAll(response.Body)
	if errorValue != nil {
		return MCPAnswer{}, errorValue
	}
	return MCPAnswer{Status: response.StatusCode, Body: json.RawMessage(body)}, nil
}
