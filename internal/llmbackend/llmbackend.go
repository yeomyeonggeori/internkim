package llmbackend

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

type Message struct {
	Role    string        `json:"role"`
	Content string        `json:"content,omitempty"`
	Parts   []MessagePart `json:"parts,omitempty"`
}

type MessagePart struct {
	Type       string `json:"type"`
	Text       string `json:"text,omitempty"`
	MimeType   string `json:"mimeType,omitempty"`
	DataBase64 string `json:"dataBase64,omitempty"`
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

type Usage struct {
	PromptTokens          int64   `json:"promptTokens"`
	CompletionTokens      int64   `json:"completionTokens"`
	TotalTokens           int64   `json:"totalTokens"`
	CachedPromptTokens    int64   `json:"cachedPromptTokens,omitempty"`
	CacheWriteTokens      int64   `json:"cacheWriteTokens,omitempty"`
	ReasoningTokens       int64   `json:"reasoningTokens,omitempty"`
	CostUSD               float64 `json:"costUSD,omitempty"`
	UpstreamInferenceCost float64 `json:"upstreamInferenceCostUSD,omitempty"`
}

type Response struct {
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	Content         string `json:"content"`
	SelectedBackend string `json:"selectedBackend"`
	ConstraintMode  string `json:"constraintMode,omitempty"`
	Usage           Usage  `json:"usage"`
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
	return completeWithProviderChain(providers, structuredRequestTrace(request), func(candidate Provider) (Response, error) {
		attemptContext, cancel := context.WithTimeout(ctx, provider.attemptTimeout())
		defer cancel()
		return candidate.CompleteStructured(attemptContext, request)
	})
}

func (provider AutoProvider) CompleteText(ctx context.Context, request TextRequest) (Response, error) {
	return completeWithProviderChain(provider.Providers, textRequestTrace(request), func(candidate Provider) (Response, error) {
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

func completeWithProviderChain(providers []Provider, requestTrace string, complete func(Provider) (Response, error)) (Response, error) {
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
			logFallback(candidate, errorValue, requestTrace)
		}
	}
	if len(attempts) == 0 {
		return Response{}, errors.New("no llm provider is available")
	}
	return Response{}, errors.New("llm provider attempts failed: " + strings.Join(attempts, "; ") + "; " + requestTrace)
}

func providerFailure(provider Provider, errorValue error) string {
	if failure, isUnavailable := providerUnavailableFailure(errorValue); isUnavailable {
		return failure
	}
	if namedProvider, ok := provider.(interface{ Name() string }); ok {
		providerName := namedProvider.Name()
		if strings.HasPrefix(errorValue.Error(), providerName+": ") {
			return errorValue.Error()
		}
		return providerName + ": " + errorValue.Error()
	}
	return errorValue.Error()
}

func logFallback(provider Provider, errorValue error, requestTrace string) {
	if errorValue != nil {
		if failure, isUnavailable := providerUnavailableFailure(errorValue); isUnavailable {
			log.Printf("llm provider failed; trying next provider: %s; %s", failure, requestTrace)
			return
		}
		log.Printf("llm provider failed; trying next provider: %v; %s", errorValue, requestTrace)
	}
}

func structuredRequestTrace(request StructuredRequest) string {
	return strings.Join([]string{
		"kind=structured",
		"constraintMode=" + structuredConstraintModeTrace(request),
		"toolChoice=" + structuredToolChoiceTrace(request),
		"executionMode=" + traceValue(request.ExecutionMode),
		"provider=" + traceValue(request.Provider),
		"model=" + traceValue(request.Model),
		"schemaName=" + traceValue(request.StructuredOutputSchema.Name),
		"schemaHash=" + hashTraceValue(request.StructuredOutputSchema.Document),
		"messagesHash=" + hashTraceValue(request.Messages),
		"seed=" + seedTraceValue(request.GenerationOptions),
		"temperature=" + temperatureTraceValue(request.GenerationOptions),
	}, " ")
}

func textRequestTrace(request TextRequest) string {
	return strings.Join([]string{
		"kind=text",
		"constraintMode=none",
		"toolChoice=none",
		"executionMode=" + traceValue(request.ExecutionMode),
		"provider=" + traceValue(request.Provider),
		"model=" + traceValue(request.Model),
		"messagesHash=" + hashTraceValue(request.Messages),
	}, " ")
}

func structuredConstraintModeTrace(request StructuredRequest) string {
	if isActionTurnStructuredRequest(request) {
		return ConstraintModeNativeToolCall
	}
	return ConstraintModeOpenAIJSONSchema
}

func structuredToolChoiceTrace(request StructuredRequest) string {
	if isActionTurnStructuredRequest(request) {
		return "required"
	}
	return "none"
}

func isActionTurnStructuredRequest(request StructuredRequest) bool {
	return strings.TrimSpace(request.StructuredOutputSchema.Name) == "blueclaw_agent_turn_action"
}

func hashTraceValue(value any) string {
	document, errorValue := json.Marshal(value)
	if errorValue != nil {
		return "unavailable"
	}
	sum := sha256.Sum256(document)
	return fmt.Sprintf("%x", sum[:8])
}

func seedTraceValue(options *GenerationOptions) string {
	if options == nil || options.Seed == nil {
		return "none"
	}
	return fmt.Sprintf("%d", *options.Seed)
}

func temperatureTraceValue(options *GenerationOptions) string {
	if options == nil || options.Temperature == nil {
		return "none"
	}
	return fmt.Sprintf("%.6g", *options.Temperature)
}

func traceValue(value string) string {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return "default"
	}
	return trimmedValue
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
