package admind

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// A plane that answers the two calls this seam makes, in the shapes the real
// routes accept: an agent key buys a member session, and that session invokes
// the tool. Nothing between admind and the plane is stubbed, so a change to
// either request shape fails here rather than on a device.
type planeUnderTest struct {
	server *httptest.Server

	mutex        sync.Mutex
	sessionAsked []map[string]string
	invoked      []invokedOnThePlane
	answer       func(string) (int, string)
}

type invokedOnThePlane struct {
	Path          string
	Authorization string
	Body          string
}

const sessionToken = "session-token-for-the-member"

func planeAnswering(t *testing.T, answer func(toolName string) (int, string)) *planeUnderTest {
	t.Helper()
	plane := &planeUnderTest{answer: answer}
	plane.server = httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		switch {
		case request.URL.Path == "/api/agent/session":
			if request.Header.Get("Authorization") != "Bearer agent-key" {
				http.Error(responseWriter, "no agent key", http.StatusUnauthorized)
				return
			}
			asked := map[string]string{}
			json.Unmarshal(body, &asked)
			plane.mutex.Lock()
			plane.sessionAsked = append(plane.sessionAsked, asked)
			plane.mutex.Unlock()
			json.NewEncoder(responseWriter).Encode(map[string]any{
				"memberID": "member-1", "accessToken": sessionToken, "expiresAt": 4102444800,
			})
		case strings.HasPrefix(request.URL.Path, "/api/v1/tools/"):
			plane.mutex.Lock()
			plane.invoked = append(plane.invoked, invokedOnThePlane{
				Path:          request.URL.Path,
				Authorization: request.Header.Get("Authorization"),
				Body:          string(body),
			})
			answerFor := plane.answer
			plane.mutex.Unlock()
			toolName := strings.TrimSuffix(strings.TrimPrefix(request.URL.Path, "/api/v1/tools/"), "/invoke")
			status, document := answerFor(toolName)
			responseWriter.Header().Set("Content-Type", "application/json")
			responseWriter.WriteHeader(status)
			responseWriter.Write([]byte(document))
		default:
			http.Error(responseWriter, "the plane serves no "+request.URL.Path, http.StatusNotFound)
		}
	}))
	t.Cleanup(plane.server.Close)
	return plane
}

func serviceReachingThePlane(t *testing.T, plane *planeUnderTest) *Service {
	t.Helper()
	directory := t.TempDir()
	keyPath := filepath.Join(directory, "agent-key")
	if errorValue := os.WriteFile(keyPath, []byte("agent-key\n"), 0o600); errorValue != nil {
		t.Fatalf("write the agent key: %v", errorValue)
	}
	return &Service{Configuration: Configuration{
		CentralPlaneAppURL:         plane.server.URL,
		CentralPlaneProjectURL:     plane.server.URL,
		CentralPlanePublishableKey: "publishable",
		CentralPlaneAgentKeyPath:   keyPath,
	}}
}

func askedOnTheSocket(service *Service, toolName string, input string, requesterEmail string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/record/api/tools/"+toolName+"/invoke", strings.NewReader(input))
	if requesterEmail != "" {
		request.Header.Set(requesterEmailHeader, requesterEmail)
	}
	recorder := httptest.NewRecorder()
	markRequestsAsAssertedByTheListener(http.HandlerFunc(service.handleRecordTool)).ServeHTTP(recorder, request)
	return recorder
}

func TestTheSeamCarriesACallToThePlaneAsTheRequester(t *testing.T) {
	plane := planeAnswering(t, func(string) (int, string) {
		return http.StatusOK, `{"tool":"leave_balance","result":{"remainingDays":13}}`
	})
	service := serviceReachingThePlane(t, plane)

	recorder := askedOnTheSocket(service, "leave_balance", `{"year":2026}`, "member@example.com")

	if recorder.Code != http.StatusOK {
		t.Fatalf("the seam answered %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"remainingDays":13`) {
		t.Fatalf("the plane's answer did not come back: %s", recorder.Body.String())
	}

	if len(plane.sessionAsked) != 1 {
		t.Fatalf("the plane was asked for %d sessions", len(plane.sessionAsked))
	}
	if plane.sessionAsked[0]["kind"] != "email" || plane.sessionAsked[0]["externalID"] != "member@example.com" {
		t.Fatalf("the session was asked for %v", plane.sessionAsked[0])
	}

	if len(plane.invoked) != 1 {
		t.Fatalf("the plane was invoked %d times", len(plane.invoked))
	}
	invoked := plane.invoked[0]
	if invoked.Path != "/api/v1/tools/leave_balance/invoke" {
		t.Fatalf("the call went to %s", invoked.Path)
	}
	if invoked.Authorization != "Bearer "+sessionToken {
		t.Fatalf("the call carried %q rather than the member's own session", invoked.Authorization)
	}
	if invoked.Body != `{"input":{"year":2026}}` {
		t.Fatalf("the plane was sent %s", invoked.Body)
	}
}

// The member's session is what the plane runs the tool as, so it must be
// borrowed per person rather than shared. Two requesters, two sessions.
func TestTheSeamBorrowsASessionForEachRequester(t *testing.T) {
	plane := planeAnswering(t, func(string) (int, string) {
		return http.StatusOK, `{"tool":"leave_list","result":{"count":0}}`
	})
	service := serviceReachingThePlane(t, plane)

	askedOnTheSocket(service, "leave_list", `{}`, "one@example.com")
	askedOnTheSocket(service, "leave_list", `{}`, "two@example.com")

	asked := []string{}
	for _, session := range plane.sessionAsked {
		asked = append(asked, session["externalID"])
	}
	if len(asked) != 2 || asked[0] != "one@example.com" || asked[1] != "two@example.com" {
		t.Fatalf("the plane was asked for sessions %v", asked)
	}
}

// A refusal is the record's to explain, so its status and its words reach the
// caller rather than being turned into a gateway error.
func TestTheSeamCarriesTheRecordsRefusalBack(t *testing.T) {
	plane := planeAnswering(t, func(string) (int, string) {
		return http.StatusConflict, `{"error":"연차 could be two of them","candidates":["a","b"]}`
	})
	service := serviceReachingThePlane(t, plane)

	recorder := askedOnTheSocket(service, "leave_decide", `{"leaveHint":"연차"}`, "member@example.com")

	if recorder.Code != http.StatusConflict {
		t.Fatalf("a 409 from the record answered %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "could be two of them") {
		t.Fatalf("the record's words did not survive: %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"candidates"`) {
		t.Fatalf("the candidates did not survive: %s", recorder.Body.String())
	}
}

func TestTheSeamSaysSoWhenThereIsNoPlaneToReach(t *testing.T) {
	service := &Service{}

	recorder := askedOnTheSocket(service, "leave_list", `{}`, "member@example.com")

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("a device with no plane answered %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "out of reach") {
		t.Fatalf("the refusal does not say why: %s", recorder.Body.String())
	}
}

func TestAnEmptyBodyReachesThePlaneAsAnEmptyInput(t *testing.T) {
	plane := planeAnswering(t, func(string) (int, string) {
		return http.StatusOK, `{"tool":"leave_list","result":{"count":0}}`
	})
	service := serviceReachingThePlane(t, plane)

	askedOnTheSocket(service, "leave_list", "", "member@example.com")

	if len(plane.invoked) != 1 || plane.invoked[0].Body != `{"input":{}}` {
		t.Fatalf("the plane was sent %v", plane.invoked)
	}
}
