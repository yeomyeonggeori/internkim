package companion

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
)

type HandoffBridgeHandler struct {
	Store             *BrowserHandoffStore
	BrowserRuntime    browserruntime.Runtime
	CompletionHandler func(context.Context, HandoffCompletion) error
}

func (handler HandoffBridgeHandler) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	if handler.Store == nil {
		http.Error(responseWriter, "browser handoff bridge is unavailable", http.StatusServiceUnavailable)
		return
	}
	switch {
	case request.Method == http.MethodGet && request.URL.Path == "/v1/browser/handoff":
		writeHandoffJSON(responseWriter, handler.Store.Snapshot())
	case request.Method == http.MethodPost && request.URL.Path == "/v1/browser/handoff/complete":
		handler.complete(responseWriter, request)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (handler HandoffBridgeHandler) complete(responseWriter http.ResponseWriter, request *http.Request) {
	var completion HandoffCompletion
	if errorValue := json.NewDecoder(request.Body).Decode(&completion); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	capturedCompletion, errorValue := handler.captureCompletion(request.Context(), completion)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if errorValue := handler.Store.ValidateCompletion(capturedCompletion); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusForbidden)
		return
	}
	if handler.CompletionHandler != nil {
		if errorValue := handler.CompletionHandler(request.Context(), capturedCompletion); errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
			return
		}
	}
	if errorValue := handler.Store.Complete(capturedCompletion); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusForbidden)
		return
	}
	writeHandoffJSON(responseWriter, map[string]bool{"ok": true})
}

func (handler HandoffBridgeHandler) captureCompletion(ctx context.Context, completion HandoffCompletion) (HandoffCompletion, error) {
	if handler.BrowserRuntime == nil {
		if completion.CapturedAt == "" {
			completion.CapturedAt = time.Now().UTC().Format(time.RFC3339)
		}
		return completion, nil
	}
	observation, errorValue := handler.BrowserRuntime.Observe(ctx, browserruntime.ObserveRequest{})
	if errorValue != nil {
		return HandoffCompletion{}, errorValue
	}
	completion.URL = firstNonEmpty(observation.URL, completion.URL)
	completion.Title = firstNonEmpty(observation.Title, completion.Title)
	completion.SnapshotText = observation.SnapshotText
	completion.InteractiveRefs = observation.InteractiveRefs
	completion.CapturedAt = firstNonEmpty(observation.CapturedAt, time.Now().UTC().Format(time.RFC3339))
	return completion, nil
}

func writeHandoffJSON(responseWriter http.ResponseWriter, response any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode(response)
}
