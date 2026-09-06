package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type blueclawHTTPError struct {
	statusCode int
}

func (errorValue blueclawHTTPError) Error() string {
	return fmt.Sprintf("Blueclaw returned HTTP %d", errorValue.statusCode)
}

func (service *Service) blueclawSignedRequest(ctx context.Context, method string, path string, body []byte, readerPersonID string, responseValue any) error {
	key := strings.TrimSpace(readTrimmedFile(service.Configuration.CentralPlaneAgentKeyPath))
	if key == "" {
		return fmt.Errorf("central plane agent key is missing")
	}
	request, errorValue := http.NewRequestWithContext(ctx, method, strings.TrimRight(service.Configuration.BlueclawBaseURL, "/")+path, bytes.NewReader(body))
	if errorValue != nil {
		return errorValue
	}
	header, errorValue := signRequestAssertion(method, request.URL.RequestURI(), body, readerPersonID, time.Now().Add(memoryAssertionLifetime).Unix(), key)
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(memoryAssertionHeader, header)
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return blueclawHTTPError{statusCode: response.StatusCode}
	}
	if responseValue == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(responseValue)
}

func (service *Service) registerLearningRoutes(multiplexer *http.ServeMux) {
	multiplexer.HandleFunc("/agent-learning/api/", service.handleAgentLearning)
}

func (service *Service) handleAgentLearning(responseWriter http.ResponseWriter, request *http.Request) {
	path := strings.TrimPrefix(request.URL.Path, "/agent-learning/api")
	if path == "/soul" || path == "/soul/history" {
		if request.Method != http.MethodGet {
			http.Error(responseWriter, "working principles are read-only", http.StatusMethodNotAllowed)
			return
		}
		service.forwardLearning(responseWriter, request, "/admin/api/agent-learning"+path)
		return
	}
	if path == "/skills" || path == "/settings" || strings.HasPrefix(path, "/skills/") {
		upstreamPath := "/admin/api/agent-learning" + path
		if request.URL.RawQuery != "" {
			upstreamPath += "?" + request.URL.RawQuery
		}
		service.forwardLearning(responseWriter, request, upstreamPath)
		return
	}
	http.NotFound(responseWriter, request)
}

func (service *Service) forwardLearning(responseWriter http.ResponseWriter, request *http.Request, path string) {
	actorEmail := service.memoryActorEmail(request)
	if actorEmail == "" {
		http.Error(responseWriter, "learning access required", http.StatusForbidden)
		return
	}
	personID, errorValue := service.resolveMemoryPersonID(request.Context(), actorEmail)
	if errorValue != nil || personID == "" {
		http.Error(responseWriter, "learning identity unavailable", http.StatusBadGateway)
		return
	}
	requestBody := request.Body
	if requestBody == nil {
		requestBody = io.NopCloser(bytes.NewReader(nil))
	}
	body, errorValue := io.ReadAll(io.LimitReader(requestBody, 16*1024+1))
	if errorValue != nil || len(body) > 16*1024 {
		http.Error(responseWriter, "invalid learning request", http.StatusBadRequest)
		return
	}
	var response any
	if errorValue := service.blueclawSignedRequest(request.Context(), request.Method, path, body, personID, &response); errorValue != nil {
		var httpError blueclawHTTPError
		if errors.As(errorValue, &httpError) && httpError.statusCode >= 400 && httpError.statusCode < 500 {
			http.Error(responseWriter, "learning request denied", httpError.statusCode)
			return
		}
		http.Error(responseWriter, "learning service unavailable", http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, response)
}
