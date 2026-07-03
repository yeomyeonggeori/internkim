package llmbackend

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

type ManagedLlamaCppBackend struct {
	Backend      LlamaCppBackend
	ServiceName  string
	RunCommand   func(context.Context, string, []string, []byte) ([]byte, error)
	StartTimeout time.Duration
	PollInterval time.Duration
}

func (backend ManagedLlamaCppBackend) Name() string {
	return backend.Backend.Name()
}

func (backend ManagedLlamaCppBackend) ContextWindowTokens() int64 {
	return backend.Backend.ContextWindowTokens()
}

func (backend ManagedLlamaCppBackend) Ping(ctx context.Context) error {
	errorValue := backend.Backend.Ping(ctx)
	if errorValue == nil || !isLlamaCppServerUnavailable(errorValue) {
		return errorValue
	}
	if errorValue := backend.startAndWait(ctx); errorValue != nil {
		return errorValue
	}
	return nil
}

func (backend ManagedLlamaCppBackend) CompleteStructured(ctx context.Context, request StructuredRequest) (Response, error) {
	response, errorValue := backend.Backend.CompleteStructured(ctx, request)
	if errorValue == nil || !isLlamaCppServerUnavailable(errorValue) {
		return response, errorValue
	}
	if errorValue := backend.startAndWait(ctx); errorValue != nil {
		return Response{}, errorValue
	}
	return backend.Backend.CompleteStructured(ctx, request)
}

func (backend ManagedLlamaCppBackend) CompleteText(ctx context.Context, request TextRequest) (Response, error) {
	response, errorValue := backend.Backend.CompleteText(ctx, request)
	if errorValue == nil || !isLlamaCppServerUnavailable(errorValue) {
		return response, errorValue
	}
	if errorValue := backend.startAndWait(ctx); errorValue != nil {
		return Response{}, errorValue
	}
	return backend.Backend.CompleteText(ctx, request)
}

func (backend ManagedLlamaCppBackend) CompleteChat(ctx context.Context, request ChatRequest) (ChatResponse, error) {
	response, errorValue := backend.Backend.CompleteChat(ctx, request)
	if errorValue == nil || !isLlamaCppServerUnavailable(errorValue) {
		return response, errorValue
	}
	if errorValue := backend.startAndWait(ctx); errorValue != nil {
		return ChatResponse{}, errorValue
	}
	return backend.Backend.CompleteChat(ctx, request)
}

func (backend ManagedLlamaCppBackend) startAndWait(ctx context.Context) error {
	if backend.RunCommand == nil {
		return errors.New("llama.cpp service manager is not configured")
	}
	if _, errorValue := backend.RunCommand(ctx, "systemctl", []string{"start", backend.ServiceName}, nil); errorValue != nil {
		return fmt.Errorf("start llama.cpp service: %w", errorValue)
	}
	return backend.waitUntilReady(ctx)
}

func (backend ManagedLlamaCppBackend) waitUntilReady(ctx context.Context) error {
	waitContext, cancel := context.WithTimeout(ctx, backend.startTimeout())
	defer cancel()

	ticker := time.NewTicker(backend.pollInterval())
	defer ticker.Stop()

	for {
		if errorValue := backend.Backend.Ping(waitContext); errorValue == nil {
			return nil
		}
		select {
		case <-waitContext.Done():
			return fmt.Errorf("wait for llama.cpp service readiness: %w", waitContext.Err())
		case <-ticker.C:
		}
	}
}

func (backend ManagedLlamaCppBackend) startTimeout() time.Duration {
	if backend.StartTimeout > 0 {
		return backend.StartTimeout
	}
	return 30 * time.Second
}

func (backend ManagedLlamaCppBackend) pollInterval() time.Duration {
	if backend.PollInterval > 0 {
		return backend.PollInterval
	}
	return 500 * time.Millisecond
}

func isLlamaCppServerUnavailable(errorValue error) bool {
	if errorValue == nil || errors.Is(errorValue, context.Canceled) {
		return false
	}
	if errors.Is(errorValue, context.DeadlineExceeded) {
		return true
	}
	var urlError *url.Error
	if errors.As(errorValue, &urlError) {
		return true
	}
	var netError net.Error
	if errors.As(errorValue, &netError) && netError.Timeout() {
		return true
	}
	message := strings.ToLower(errorValue.Error())
	return strings.Contains(message, "connection refused") ||
		strings.Contains(message, "connection reset") ||
		strings.Contains(message, "no route to host")
}
