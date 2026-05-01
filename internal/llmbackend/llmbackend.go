package llmbackend

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type StructuredOutputSchema struct {
	Name               string          `json:"name"`
	Document           json.RawMessage `json:"document"`
	IsStrictlyEnforced bool            `json:"isStrictlyEnforced"`
}

type StructuredRequest struct {
	Model                  string                 `json:"model"`
	Backend                string                 `json:"backend,omitempty"`
	ExecutionMode          string                 `json:"executionMode"`
	Messages               []Message              `json:"messages"`
	StructuredOutputSchema StructuredOutputSchema `json:"structuredOutputSchema"`
	RequireParameters      bool                   `json:"requireParameters"`
	EnableResponseHealing  bool                   `json:"enableResponseHealing"`
}

type TextRequest struct {
	Model                 string    `json:"model"`
	Backend               string    `json:"backend,omitempty"`
	ExecutionMode         string    `json:"executionMode"`
	Messages              []Message `json:"messages"`
	RequireParameters     bool      `json:"requireParameters"`
	EnableResponseHealing bool      `json:"enableResponseHealing"`
}

type Response struct {
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	Content         string `json:"content"`
	SelectedBackend string `json:"selectedBackend"`
	ConstraintMode  string `json:"constraintMode,omitempty"`
}

type StructuredCompleter interface {
	CompleteStructured(context.Context, StructuredRequest) (Response, error)
}

type TextCompleter interface {
	CompleteText(context.Context, TextRequest) (Response, error)
}

type Provider interface {
	StructuredCompleter
	TextCompleter
}

type Backend interface {
	Provider
	Name() string
	Ping(context.Context) error
}

const DefaultAttemptTimeout = 90 * time.Second

type AutoProvider struct {
	Providers      []Provider
	AttemptTimeout time.Duration
}

func (provider AutoProvider) CompleteStructured(ctx context.Context, request StructuredRequest) (Response, error) {
	return completeWithProviderChain(provider.Providers, func(candidate Provider) (Response, error) {
		attemptContext, cancel := context.WithTimeout(ctx, provider.attemptTimeout())
		defer cancel()
		return candidate.CompleteStructured(attemptContext, request)
	})
}

func (provider AutoProvider) CompleteText(ctx context.Context, request TextRequest) (Response, error) {
	return completeWithProviderChain(provider.Providers, func(candidate Provider) (Response, error) {
		attemptContext, cancel := context.WithTimeout(ctx, provider.attemptTimeout())
		defer cancel()
		return candidate.CompleteText(attemptContext, request)
	})
}

func (provider AutoProvider) attemptTimeout() time.Duration {
	if provider.AttemptTimeout <= 0 {
		return DefaultAttemptTimeout
	}
	return provider.AttemptTimeout
}

func completeWithProviderChain(providers []Provider, complete func(Provider) (Response, error)) (Response, error) {
	var lastError error
	for index, candidate := range providers {
		if candidate == nil {
			continue
		}
		response, errorValue := complete(candidate)
		if errorValue == nil {
			return response, nil
		}
		lastError = errorValue
		if index == 0 {
			logFallback(errorValue)
		}
	}
	if lastError == nil {
		lastError = errors.New("no llm provider is available")
	}
	return Response{}, lastError
}

func logFallback(errorValue error) {
	if errorValue != nil {
		log.Printf("llm provider failed; trying next provider: %v", errorValue)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue != "" {
			return trimmedValue
		}
	}
	return ""
}
