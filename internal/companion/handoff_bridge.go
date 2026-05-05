package companion

import (
	"encoding/json"
	"net/http"
)

type HandoffBridgeHandler struct {
	Store *BrowserHandoffStore
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
	if errorValue := handler.Store.Complete(completion); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusForbidden)
		return
	}
	writeHandoffJSON(responseWriter, map[string]bool{"ok": true})
}

func writeHandoffJSON(responseWriter http.ResponseWriter, response any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode(response)
}
