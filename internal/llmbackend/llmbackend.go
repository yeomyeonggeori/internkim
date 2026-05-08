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

type GenerationOptions struct {
	Seed        *int64   `json:"seed,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
}

type RequestContext struct {
	RequesterPersonID       string `json:"requesterPersonID,omitempty"`
	RequesterEmail          string `json:"requesterEmail,omitempty"`
	RequesterName           string `json:"requesterName,omitempty"`
	RequesterPlatformUserID string `json:"requesterPlatformUserID,omitempty"`
	ConversationID          string `json:"conversationID,omitempty"`
	Platform                string `json:"platform,omitempty"`
}

type StructuredRequest struct {
	Model                  string                 `json:"model"`
	Provider               string                 `json:"provider,omitempty"`
	Accelerator            string                 `json:"accelerator,omitempty"`
	ExecutionMode          string                 `json:"executionMode"`
	Context                RequestContext         `json:"context,omitempty"`
	Messages               []Message              `json:"messages"`
	StructuredOutputSchema StructuredOutputSchema `json:"structuredOutputSchema"`
	GenerationOptions      *GenerationOptions     `json:"generationOptions,omitempty"`
	RequireParameters      bool                   `json:"requireParameters"`
	EnableResponseHealing  bool                   `json:"enableResponseHealing"`
}

type TextRequest struct {
	Model                 string         `json:"model"`
	Provider              string         `json:"provider,omitempty"`
	Accelerator           string         `json:"accelerator,omitempty"`
	ExecutionMode         string         `json:"executionMode"`
	Context               RequestContext `json:"context,omitempty"`
	Messages              []Message      `json:"messages"`
	RequireParameters     bool           `json:"requireParameters"`
	EnableResponseHealing bool           `json:"enableResponseHealing"`
}

type Response struct {
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	Content         string `json:"content"`
	SelectedBackend string `json:"selectedBackend"`
	ConstraintMode  string `json:"constraintMode,omitempty"`
}

const (
	ConstraintModeOpenAIJSONSchema           = "openai_json_schema"
	ConstraintModeLlamaJSONSchema            = "llama_json_schema"
	ConstraintModeLlamaGBNF                  = "llama_gbnf"
	ConstraintModeLiteRTLLGuidanceJSONSchema = "litert_llguidance_json_schema"
	ConstraintModeNativeToolCall             = "native_tool_call"
)

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
	Providers               []Provider
	AttemptTimeout          time.Duration
	AllowStructuredFallback bool
}

func (provider AutoProvider) CompleteStructured(ctx context.Context, request StructuredRequest) (Response, error) {
	providers := provider.Providers
	if !provider.AllowStructuredFallback && len(providers) > 1 {
		providers = providers[:1]
	}
	return completeWithProviderChain(providers, func(candidate Provider) (Response, error) {
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
	attempts := make([]string, 0, len(providers))
	for index, candidate := range providers {
		if candidate == nil {
			continue
		}
		response, errorValue := complete(candidate)
		if errorValue == nil {
			return response, nil
		}
		attempts = append(attempts, providerFailure(candidate, errorValue))
		if index < len(providers)-1 {
			logFallback(errorValue)
		}
	}
	if len(attempts) == 0 {
		return Response{}, errors.New("no llm provider is available")
	}
	return Response{}, errors.New("llm provider attempts failed: " + strings.Join(attempts, "; "))
}

func providerFailure(provider Provider, errorValue error) string {
	if namedProvider, ok := provider.(interface{ Name() string }); ok {
		return namedProvider.Name() + ": " + errorValue.Error()
	}
	return errorValue.Error()
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
