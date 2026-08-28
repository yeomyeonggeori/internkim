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

func TestTaskCallsReachAdmindWhereItHonoursTheRequester(t *testing.T) {
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

	for _, call := range taskCallsCarryingARequester(service) {
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

func TestTaskCallsRefuseRatherThanSendARequesterNobodyHonours(t *testing.T) {
	service := Service{Configuration: Configuration{AdmindBaseURL: admindOnLoopbackThatFailsTheTest(t)}}

	for _, call := range taskCallsCarryingARequester(service) {
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

type taskCallCarryingARequester struct {
	name   string
	call   string
	invoke func(context.Context) error
}

func taskCallsCarryingARequester(service Service) []taskCallCarryingARequester {
	return []taskCallCarryingARequester{
		{name: "read", call: "GET /task/api/state", invoke: func(ctx context.Context) error {
			_, errorValue := service.getTask(ctx, "/task/api/state", "staff@example.com")
			return errorValue
		}},
		{name: "add", call: "POST /task/api/tasks", invoke: func(ctx context.Context) error {
			_, errorValue := service.postTask(ctx, taskCreatePayload{OwnerID: "staff", Content: "업무"}, "staff@example.com")
			return errorValue
		}},
		{name: "update", call: "PUT /task/api/tasks/task-1", invoke: func(ctx context.Context) error {
			_, errorValue := service.putTask(ctx, taskForTool{ID: "task-1"}, "staff@example.com")
			return errorValue
		}},
		{name: "delete", call: "DELETE /task/api/tasks/task-1", invoke: func(ctx context.Context) error {
			_, errorValue := service.deleteTask(ctx, "task-1", "staff@example.com")
			return errorValue
		}},
	}
}
