package capabilityd

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func admindOnASocket(t *testing.T, handler http.Handler) string {
	t.Helper()
	directory, errorValue := os.MkdirTemp("", "admind")
	if errorValue != nil {
		t.Fatalf("make the socket directory: %v", errorValue)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	socketPath := filepath.Join(directory, "admind.sock")
	listener, errorValue := net.Listen("unix", socketPath)
	if errorValue != nil {
		t.Fatalf("listen on %s: %v", socketPath, errorValue)
	}
	server := httptest.NewUnstartedServer(handler)
	_ = server.Listener.Close()
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	return socketPath
}

func admindOnLoopbackThatFailsTheTest(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		t.Errorf("%s %s reached admind over TCP, where the requester header is ignored", request.Method, request.URL.Path)
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func TestFlowCallsReachAdmindWhereItHonoursTheRequester(t *testing.T) {
	requesterOfCall := map[string]string{}
	socketPath := admindOnASocket(t, http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		requesterEmail := request.Header.Get(admindRequesterEmailHeader)
		if requesterEmail == "" {
			http.Error(responseWriter, "flow refuses a caller it cannot name", http.StatusForbidden)
			return
		}
		requesterOfCall[request.Method+" "+request.URL.Path] = requesterEmail
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"id":"task-1"}`))
	}))
	service := Service{Configuration: Configuration{
		AdmindBaseURL:    admindOnLoopbackThatFailsTheTest(t),
		AdmindSocketPath: socketPath,
	}}

	for _, call := range flowCallsCarryingARequester(service) {
		t.Run(call.name, func(t *testing.T) {
			if errorValue := call.invoke(context.Background()); errorValue != nil {
				t.Fatalf("%s: %v", call.name, errorValue)
			}
			if requesterOfCall[call.call] != "staff@example.com" {
				t.Fatalf("admind served %s as %q", call.call, requesterOfCall[call.call])
			}
		})
	}
}

func TestFlowCallsRefuseRatherThanSendARequesterNobodyHonours(t *testing.T) {
	service := Service{Configuration: Configuration{AdmindBaseURL: admindOnLoopbackThatFailsTheTest(t)}}

	for _, call := range flowCallsCarryingARequester(service) {
		t.Run(call.name, func(t *testing.T) {
			errorValue := call.invoke(context.Background())
			if errorValue == nil {
				t.Fatalf("%s went somewhere without an admind socket to go to", call.name)
			}
			if !strings.Contains(errorValue.Error(), "socket") {
				t.Fatalf("the failure does not say the socket is missing: %v", errorValue)
			}
		})
	}
}

type flowCallCarryingARequester struct {
	name   string
	call   string
	invoke func(context.Context) error
}

func flowCallsCarryingARequester(service Service) []flowCallCarryingARequester {
	return []flowCallCarryingARequester{
		{name: "read", call: "GET /flow/api/state", invoke: func(ctx context.Context) error {
			_, errorValue := service.getFlow(ctx, "/flow/api/state", "staff@example.com")
			return errorValue
		}},
		{name: "add", call: "POST /flow/api/tasks", invoke: func(ctx context.Context) error {
			_, errorValue := service.postFlowTask(ctx, flowTaskCreatePayload{OwnerID: "staff", Content: "업무"}, "staff@example.com")
			return errorValue
		}},
		{name: "update", call: "PUT /flow/api/tasks/task-1", invoke: func(ctx context.Context) error {
			_, errorValue := service.putFlowTask(ctx, flowTaskForTool{ID: "task-1"}, "staff@example.com")
			return errorValue
		}},
		{name: "delete", call: "DELETE /flow/api/tasks/task-1", invoke: func(ctx context.Context) error {
			_, errorValue := service.deleteFlowTask(ctx, "task-1", "staff@example.com")
			return errorValue
		}},
	}
}
