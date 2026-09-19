package llmbackend

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const servingWatchInterval = 250 * time.Millisecond

type openAIStreamChunk struct {
	Provider string `json:"provider"`
	Choices  []struct {
		Delta struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			Reasoning string `json:"reasoning"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *openAIUsage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type streamProgress struct {
	mutex            sync.Mutex
	startedAt        time.Time
	provider         string
	outputCharacters int64
}

func (progress *streamProgress) note(provider string, characters int) {
	progress.mutex.Lock()
	defer progress.mutex.Unlock()
	if provider != "" {
		progress.provider = provider
	}
	progress.outputCharacters += int64(characters)
}

func (progress *streamProgress) snapshot(now time.Time) (string, time.Duration, int64) {
	progress.mutex.Lock()
	defer progress.mutex.Unlock()
	return progress.provider, now.Sub(progress.startedAt), progress.outputCharacters
}

type streamedCompletion struct {
	response  openAIResponseWithUsage
	toolCalls map[int]*openAIToolCall
	toolOrder []int
}

type streamOutcome struct {
	response   openAIResponseWithUsage
	errorValue error
}

func (backend OpenRouterBackend) streamCompletion(ctx context.Context, apiKey string, requestDocument []byte, modelName string, sessionID string, slowSignal chan<- slowServingError) (openAIResponseWithUsage, error) {
	progress := &streamProgress{startedAt: time.Now()}
	if expectation, isKnown := sharedServingRecord.expectation(modelName); isKnown && slowSignal != nil {
		go watchServing(ctx, slowSignal, progress, expectation)
	}
	response, errorValue := backend.readStream(ctx, apiKey, requestDocument, sessionID, progress)
	provider, elapsed, outputCharacters := progress.snapshot(time.Now())
	if errorValue == nil {
		sharedServingRecord.recordSample(modelName, servingSample{Provider: provider, Duration: elapsed, CharactersPerSecond: float64(outputCharacters) / elapsed.Seconds()})
		return response, nil
	}
	if ctx.Err() != nil {
		sharedServingRecord.noteCut(modelName, provider)
	}
	return openAIResponseWithUsage{}, errorValue
}

func watchServing(ctx context.Context, slowSignal chan<- slowServingError, progress *streamProgress, expectation servingExpectation) {
	ticker := time.NewTicker(servingWatchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			provider, elapsed, outputCharacters := progress.snapshot(now)
			if !expectation.judgesSlow(elapsed, outputCharacters) {
				continue
			}
			slowSignal <- slowServingError{Provider: provider, Elapsed: elapsed, CharactersPerSecond: float64(outputCharacters) / elapsed.Seconds(), Expectation: expectation}
			return
		}
	}
}

func (backend OpenRouterBackend) readStream(ctx context.Context, apiKey string, requestDocument []byte, sessionID string, progress *streamProgress) (openAIResponseWithUsage, error) {
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, backend.BaseURL, bytes.NewReader(requestDocument))
	if errorValue != nil {
		return openAIResponseWithUsage{}, errorValue
	}
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "text/event-stream")
	setSessionAffinityHeader(httpRequest, sessionID)
	backend.setGatewaySecretHeader(httpRequest)

	httpResponse, errorValue := backend.HTTPClient.Do(httpRequest)
	if errorValue != nil {
		return openAIResponseWithUsage{}, errorValue
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode >= http.StatusBadRequest {
		responseDocument, _ := io.ReadAll(httpResponse.Body)
		return openAIResponseWithUsage{}, normalizeProviderError("openrouter", httpResponse.StatusCode, responseDocument)
	}
	if !strings.Contains(httpResponse.Header.Get("Content-Type"), "text/event-stream") {
		return readWholeResponse(httpResponse.Body, progress)
	}
	return assembleStream(httpResponse.Body, progress)
}

func readWholeResponse(body io.Reader, progress *streamProgress) (openAIResponseWithUsage, error) {
	responseDocument, errorValue := io.ReadAll(body)
	if errorValue != nil {
		return openAIResponseWithUsage{}, errors.New("read openrouter response: " + errorValue.Error())
	}
	if len(bytes.TrimSpace(responseDocument)) == 0 {
		return openAIResponseWithUsage{}, errors.New("openrouter response body was empty")
	}
	var response openAIResponseWithUsage
	if errorValue := json.Unmarshal(responseDocument, &response); errorValue != nil {
		return openAIResponseWithUsage{}, errorValue
	}
	if len(response.Choices) == 0 {
		return openAIResponseWithUsage{}, errors.New("openrouter response did not include choices")
	}
	progress.note(response.Provider, len(response.Choices[0].Message.Content))
	return response, nil
}

func assembleStream(body io.Reader, progress *streamProgress) (openAIResponseWithUsage, error) {
	completion := streamedCompletion{toolCalls: map[int]*openAIToolCall{}}
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	sawDone := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			sawDone = true
			break
		}
		if errorValue := completion.absorb(payload, progress); errorValue != nil {
			return openAIResponseWithUsage{}, errorValue
		}
	}
	if errorValue := scanner.Err(); errorValue != nil {
		return openAIResponseWithUsage{}, errors.New("read openrouter stream: " + errorValue.Error())
	}
	if !sawDone && len(completion.response.Choices) == 0 {
		return openAIResponseWithUsage{}, errors.New("openrouter stream ended without choices")
	}
	return completion.finish(), nil
}

func (completion *streamedCompletion) absorb(payload string, progress *streamProgress) error {
	var chunk openAIStreamChunk
	if errorValue := json.Unmarshal([]byte(payload), &chunk); errorValue != nil {
		return errors.New("decode openrouter stream chunk: " + errorValue.Error())
	}
	if chunk.Error != nil {
		return errors.New("openrouter stream error: " + chunk.Error.Message)
	}
	if chunk.Provider != "" {
		completion.response.Provider = chunk.Provider
	}
	if chunk.Usage != nil {
		completion.response.Usage = *chunk.Usage
	}
	if len(chunk.Choices) == 0 {
		progress.note(chunk.Provider, 0)
		return nil
	}
	if len(completion.response.Choices) == 0 {
		completion.response.Choices = []openAIChoice{{}}
	}
	choice := &completion.response.Choices[0]
	delta := chunk.Choices[0].Delta
	if delta.Role != "" {
		choice.Message.Role = delta.Role
	}
	choice.Message.Content += delta.Content
	characters := len(delta.Content) + len(delta.Reasoning)
	for _, toolCallDelta := range delta.ToolCalls {
		toolCall := completion.toolCalls[toolCallDelta.Index]
		if toolCall == nil {
			toolCall = &openAIToolCall{}
			completion.toolCalls[toolCallDelta.Index] = toolCall
			completion.toolOrder = append(completion.toolOrder, toolCallDelta.Index)
		}
		toolCall.ID = firstNonEmpty(toolCall.ID, toolCallDelta.ID)
		toolCall.Type = firstNonEmpty(toolCall.Type, toolCallDelta.Type)
		toolCall.Function.Name = firstNonEmpty(toolCall.Function.Name, toolCallDelta.Function.Name)
		toolCall.Function.Arguments += toolCallDelta.Function.Arguments
		characters += len(toolCallDelta.Function.Name) + len(toolCallDelta.Function.Arguments)
	}
	if chunk.Choices[0].FinishReason != "" {
		choice.FinishReason = chunk.Choices[0].FinishReason
	}
	progress.note(chunk.Provider, characters)
	return nil
}

func (completion *streamedCompletion) finish() openAIResponseWithUsage {
	if len(completion.response.Choices) == 0 {
		return completion.response
	}
	choice := &completion.response.Choices[0]
	choice.Message.Role = firstNonEmpty(choice.Message.Role, "assistant")
	for _, index := range completion.toolOrder {
		choice.Message.ToolCalls = append(choice.Message.ToolCalls, *completion.toolCalls[index])
	}
	return completion.response
}
