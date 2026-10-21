package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	nostr "github.com/nbd-wtf/go-nostr"
)

type profilesHeardByTheRelay struct {
	mutex  sync.Mutex
	byName map[string]string
}

func (heard *profilesHeardByTheRelay) nameOf(pubkey string) string {
	heard.mutex.Lock()
	defer heard.mutex.Unlock()
	return heard.byName[pubkey]
}

func relayHearingProfiles(t *testing.T) (*httptest.Server, *profilesHeardByTheRelay) {
	t.Helper()
	heard := &profilesHeardByTheRelay{byName: map[string]string{}}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		connection, errorValue := websocket.Accept(writer, request, nil)
		if errorValue != nil {
			return
		}
		defer connection.CloseNow()
		ctx := request.Context()
		_ = connection.Write(ctx, websocket.MessageText, []byte(`["AUTH","challenge-1"]`))
		for {
			_, message, errorValue := connection.Read(ctx)
			if errorValue != nil {
				return
			}
			var event nostr.Event
			switch received := nostr.ParseMessage(string(message)).(type) {
			case *nostr.AuthEnvelope:
				event = received.Event
			case *nostr.EventEnvelope:
				event = received.Event
			default:
				continue
			}
			if event.Kind == nostr.KindProfileMetadata {
				var profile struct {
					Name string `json:"name"`
				}
				_ = json.Unmarshal([]byte(event.Content), &profile)
				heard.mutex.Lock()
				heard.byName[event.PubKey] = profile.Name
				heard.mutex.Unlock()
			}
			answer, _ := nostr.OKEnvelope{EventID: event.ID, OK: true}.MarshalJSON()
			_ = connection.Write(ctx, websocket.MessageText, answer)
		}
	}))
	t.Cleanup(server.Close)
	return server, heard
}

func serviceNamingTheSampleWithNoOpenRoom(t *testing.T) (*Service, *profilesHeardByTheRelay) {
	t.Helper()
	relay := disposableRelayDatabase(t)
	if _, errorValue := relay.Exec(roomShapeCreateStatement); errorValue != nil {
		t.Fatalf("create the room tables: %v", errorValue)
	}
	if _, errorValue := relay.Exec(`CREATE TABLE events (pubkey bytea NOT NULL, kind integer NOT NULL, content text NOT NULL, created_at bigint NOT NULL)`); errorValue != nil {
		t.Fatalf("create the event table: %v", errorValue)
	}
	relayServer, heard := relayHearingProfiles(t)
	service, _ := serviceRecordingRelayMembershipGrants(t)
	service.Configuration.BuzzRelayURL = strings.Replace(relayServer.URL, "http://", "ws://", 1)
	service.Configuration.BuzzDatabaseURL = "postgres://the-disposable-relay"
	service.buzzDatabaseOwner.database = relay
	service.buzzDatabaseOwner.connectionURL = service.Configuration.BuzzDatabaseURL
	pointAtACompanyDirectory(t, service)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isTheDirectoryAsked(request) {
			return jsonResponse(http.StatusOK, `{"members":[{"memberID":"member-1","email":"sample@example.com","name":"샘플 이","status":"active"}]}`, nil), nil
		}
		if isBlueclawPolicyGet(request) {
			return jsonResponse(http.StatusOK, `{"people":[{"personID":"p-1","emails":["sample@example.com"]}]}`, nil), nil
		}
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	})}
	return service, heard
}

func requireTheSampleNamed(t *testing.T, service *Service, heard *profilesHeardByTheRelay) {
	t.Helper()
	pubkey, errorValue := buzzPublicKey(service.buzzSecretForEmail(context.Background(), "sample@example.com"))
	if errorValue != nil {
		t.Fatalf("sample pubkey: %v", errorValue)
	}
	if name := heard.nameOf(pubkey); name != "이샘플" {
		t.Fatalf("a member of a company with no open room must still be named, the relay heard %q", name)
	}
}

func TestADirectoryChangeNamesMembersWhenTheCompanyHasNoOpenRoom(t *testing.T) {
	service, heard := serviceNamingTheSampleWithNoOpenRoom(t)

	service.handleDirectoryChanged(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/admin/api/directory/changed", nil))

	requireTheSampleNamed(t, service, heard)
}

func TestTheMembershipPassNamesMembersWhenTheCompanyHasNoOpenRoom(t *testing.T) {
	service, heard := serviceNamingTheSampleWithNoOpenRoom(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	service.runMembershipPass(ctx)

	requireTheSampleNamed(t, service, heard)
}
