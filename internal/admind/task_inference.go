package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	blueclawruntime "github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func (service *Service) inferTask(ctx context.Context, prompt string, weekCode string, owner taskMember, members []taskMember, definitions taskDefinitions) (inferredTask, error) {
	requestDocument, errorValue := json.Marshal(taskLLMRequest(prompt, weekCode, owner, members, definitions))
	if errorValue != nil {
		return inferredTask{}, errorValue
	}
	responseDocument, errorValue := service.callCapabilityLLM(ctx, "/v1/llm/structured", requestDocument)
	if errorValue != nil {
		return inferredTask{}, errorValue
	}
	var response capabilityLLMResponse
	if errorValue := json.Unmarshal(responseDocument, &response); errorValue != nil {
		return inferredTask{}, errorValue
	}
	var task inferredTask
	if errorValue := json.Unmarshal([]byte(response.Content), &task); errorValue != nil {
		return inferredTask{}, fmt.Errorf("flow task inference returned invalid JSON: %w", errorValue)
	}
	task.Content = firstNonEmpty(strings.TrimSpace(task.Content), prompt)
	task.Status = firstNonEmpty(cleanTaskStatus(task.Status), defaultTaskStatus())
	if !isAllowedTaskStatus(task.Status) {
		task.Status = defaultTaskStatus()
	}
	task.StartDate = strings.TrimSpace(task.StartDate)
	task.EndDate = strings.TrimSpace(task.EndDate)
	task.ParticipantIDs = cleanParticipantIDs(task.ParticipantIDs, members)
	return task, nil
}

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
