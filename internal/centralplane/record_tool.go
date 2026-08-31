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

// A tool whose rows live in the record runs on the plane, as the person who
// asked for it. The plane resolves an email to a member the same way it
// resolves a messenger identity, so the session this borrows is that member's
// own and row level security decides the rest.
type RecordToolAnswer struct {
	Status int
	Body   json.RawMessage
}

func (client *Client) InvokeRecordTool(
	ctx context.Context,
	requesterEmail string,
	toolName string,
	input json.RawMessage,
) (RecordToolAnswer, error) {
	email := strings.ToLower(strings.TrimSpace(requesterEmail))
	if email == "" {
		return RecordToolAnswer{}, fmt.Errorf("%s runs as the person who asked, and this call named nobody", toolName)
	}

	session, errorValue := client.sessionFor(ctx, "email", email)
	if errorValue != nil {
		return RecordToolAnswer{}, errorValue
	}

	payload, errorValue := json.Marshal(map[string]json.RawMessage{"input": inputOrEmptyObject(input)})
	if errorValue != nil {
		return RecordToolAnswer{}, errorValue
	}

	address := strings.TrimSuffix(client.settings.AppURL, "/") + "/api/v1/tools/" + toolName + "/invoke"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, address, bytes.NewReader(payload))
	if errorValue != nil {
		return RecordToolAnswer{}, errorValue
	}
	request.Header.Set("Authorization", "Bearer "+session.accessToken)
	request.Header.Set("Content-Type", "application/json")

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return RecordToolAnswer{}, errorValue
	}
	defer response.Body.Close()

	body, errorValue := io.ReadAll(response.Body)
	if errorValue != nil {
		return RecordToolAnswer{}, errorValue
	}
	return RecordToolAnswer{Status: response.StatusCode, Body: json.RawMessage(body)}, nil
}

func inputOrEmptyObject(input json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(input)) == 0 {
		return json.RawMessage("{}")
	}
	return input
}
