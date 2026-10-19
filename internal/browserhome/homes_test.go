package browserhome

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

func sampleHomes(gatewayURL string) Homes {
	return Homes{
		ThisBox:    "head-office",
		GatewayURL: gatewayURL,
		CompanyID:  "company-1",
		Members:    map[string]string{"sample@example.com": "annex", "local@example.com": "head-office"},
	}
}

func TestAPersonsBrowserIsElsewhereOnlyWhenPlacedOnAnotherBox(t *testing.T) {
	homes := sampleHomes("wss://gateway.example.test")

	if box, isElsewhere := homes.BoxOf(" Sample@Example.com "); !isElsewhere || box != "annex" {
		t.Fatalf("placed on %q (%v)", box, isElsewhere)
	}
	if _, isElsewhere := homes.BoxOf("local@example.com"); isElsewhere {
		t.Fatal("a browser placed on this box was handed elsewhere")
	}
	if _, isElsewhere := homes.BoxOf("unplaced@example.com"); isElsewhere {
		t.Fatal("a person with no placement was handed elsewhere")
	}
}

func TestHomesAreReadAsWrittenAndAnEmptyPlacementLeavesNoFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "browser-homes.json")
	if errorValue := Write(path, sampleHomes("wss://gateway.example.test")); errorValue != nil {
		t.Fatal(errorValue)
	}
	read, isRead := Read(path)
	if !isRead || read.Members["sample@example.com"] != "annex" {
		t.Fatalf("read %+v (%v)", read, isRead)
	}

	if errorValue := Write(path, Homes{ThisBox: "head-office"}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, isRead := Read(path); isRead {
		t.Fatal("a company whose browsers all run here kept a placement")
	}
}

func handingTo(t *testing.T, answer func(http.ResponseWriter, *http.Request)) (Homes, Hand) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(answer))
	t.Cleanup(server.Close)
	homes := sampleHomes("ws" + server.URL[len("http"):])
	return homes, Hand{Token: func() (string, error) { return "host.session.token", nil }}
}

func TestWorkGoesThroughTheGatewayToTheBoxItNamesAndItsAnswerComesBack(t *testing.T) {
	homes, hand := handingTo(t, func(writer http.ResponseWriter, request *http.Request) {
		var call struct {
			BoxKey    string          `json:"boxKey"`
			RequestID string          `json:"requestID"`
			Work      json.RawMessage `json:"work"`
		}
		body, _ := io.ReadAll(request.Body)
		json.Unmarshal(body, &call)
		if request.URL.Path != "/company/company-1/worker-call" || request.Header.Get("Authorization") != "Bearer host.session.token" || call.BoxKey != "annex" || call.RequestID != "work-1" {
			t.Errorf("the gateway was asked %s with %s", request.URL.Path, body)
		}
		writer.Write([]byte(`{"requestID":"work-1","status":200,"body":{"toolName":"browser_open"}}`))
	})

	answer, errorValue := hand.Give(context.Background(), homes, "annex", "work-1", json.RawMessage(`{"toolName":"browser_open"}`))

	if errorValue != nil || string(answer) != `{"toolName":"browser_open"}` {
		t.Fatalf("answered %s, %v", answer, errorValue)
	}
}

func TestABoxThatIsNotConnectedIsAwayAndOneThatTookTheWorkIsNot(t *testing.T) {
	homes, hand := handingTo(t, func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, `{"error":"that box is not connected to take work"}`, http.StatusServiceUnavailable)
	})
	if _, errorValue := hand.Give(context.Background(), homes, "annex", "work-1", json.RawMessage(`{}`)); !errors.Is(errorValue, ErrBoxAway) {
		t.Fatalf("a box that was not connected answered %v", errorValue)
	}

	homes, hand = handingTo(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Write([]byte(`{"requestID":"work-1","status":504,"body":{"error":"call_timed_out"}}`))
	})
	_, errorValue := hand.Give(context.Background(), homes, "annex", "work-1", json.RawMessage(`{}`))
	if errorValue == nil || errors.Is(errorValue, ErrBoxAway) {
		t.Fatalf("work a box may have done was treated as never handed on: %v", errorValue)
	}
}

func TestTheGatewayIsReachedOverHTTPAndASocketOverItsSocketAddress(t *testing.T) {
	if HTTPAddressOf("wss://gateway.example.test/") != "https://gateway.example.test" || SocketAddressOf("https://gateway.example.test") != "wss://gateway.example.test" {
		t.Fatal("the gateway's addresses were not converted")
	}
}
