package workqueue

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func postingTo(t *testing.T, answer func(http.ResponseWriter, *http.Request)) (Address, Queue) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(answer))
	t.Cleanup(server.Close)
	address := Address{GatewayURL: "ws" + server.URL[len("http"):], CompanyID: "company-1"}
	return address, Queue{Token: func() (string, error) { return "host.session.token", nil }}
}

func TestTheAddressIsReadAsWrittenAndAMissingOneIsNotRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "work-queue.json")
	if _, isRead := Read(path); isRead {
		t.Fatal("an address that was never written was read")
	}
	written := Address{GatewayURL: "wss://gateway.example.test", CompanyID: "company-1"}
	if errorValue := Write(path, written); errorValue != nil {
		t.Fatal(errorValue)
	}
	if read, isRead := Read(path); !isRead || read != written {
		t.Fatalf("read %+v (%v)", read, isRead)
	}
	if errorValue := Forget(path); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, isRead := Read(path); isRead {
		t.Fatal("a forgotten address was still read")
	}
}

func TestWorkGoesIntoTheQueueNamingNoBoxAndItsAnswerComesBack(t *testing.T) {
	address, queue := postingTo(t, func(writer http.ResponseWriter, request *http.Request) {
		var posted map[string]json.RawMessage
		body, _ := io.ReadAll(request.Body)
		json.Unmarshal(body, &posted)
		_, namesABox := posted["boxKey"]
		if request.URL.Path != "/company/company-1/work" || request.Header.Get("Authorization") != "Bearer host.session.token" || namesABox || string(posted["affinityKey"]) != `"person-1"` {
			t.Errorf("the gateway was asked %s with %s", request.URL.Path, body)
		}
		writer.Write([]byte(`{"requestID":"work-1","status":200,"body":{"toolName":"browser_open"}}`))
	})

	answer, errorValue := queue.Post(context.Background(), address, "work-1", "person-1", json.RawMessage(`{"toolName":"browser_open"}`))

	if errorValue != nil || string(answer) != `{"toolName":"browser_open"}` {
		t.Fatalf("answered %s, %v", answer, errorValue)
	}
}

func TestWorkNoBoxHadRoomForIsToldApartFromWorkABoxMayHaveDone(t *testing.T) {
	address, queue := postingTo(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Write([]byte(`{"requestID":"work-1","status":503,"body":{"error":"no box had room for this work"}}`))
	})
	if _, errorValue := queue.Post(context.Background(), address, "work-1", "person-1", json.RawMessage(`{}`)); !errors.Is(errorValue, ErrNoBoxHadRoom) {
		t.Fatalf("work no box took answered %v", errorValue)
	}

	address, queue = postingTo(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Write([]byte(`{"requestID":"work-1","status":504,"body":{"error":"call_timed_out"}}`))
	})
	_, errorValue := queue.Post(context.Background(), address, "work-1", "person-1", json.RawMessage(`{}`))
	if errorValue == nil || errors.Is(errorValue, ErrNoBoxHadRoom) {
		t.Fatalf("work a box may have done was treated as never taken: %v", errorValue)
	}
}

func TestTheGatewayIsReachedOverHTTPAndASocketOverItsSocketAddress(t *testing.T) {
	if HTTPAddressOf("wss://gateway.example.test/") != "https://gateway.example.test" || SocketAddressOf("https://gateway.example.test") != "wss://gateway.example.test" {
		t.Fatal("the gateway's addresses were not converted")
	}
}
