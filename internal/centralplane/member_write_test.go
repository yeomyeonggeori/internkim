package centralplane

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type memberDoorCall struct {
	method        string
	path          string
	query         string
	authorization string
	body          map[string]any
}

func memberDoorForTest(t *testing.T, answer string) (*Client, *memberDoorCall) {
	t.Helper()
	call := &memberDoorCall{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		call.method = request.Method
		call.path = request.URL.Path
		call.query = request.URL.RawQuery
		call.authorization = request.Header.Get("Authorization")
		document, _ := io.ReadAll(request.Body)
		if len(document) > 0 {
			_ = json.Unmarshal(document, &call.body)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(answer))
	}))
	t.Cleanup(server.Close)
	client := New(Settings{AppURL: server.URL, AgentAPIKey: "agent-key", ProjectURL: server.URL, PublishableKey: "publishable"})
	return client, call
}

func TestSaveMemberPostsTheAccountWriteThroughTheMemberDoor(t *testing.T) {
	client, call := memberDoorForTest(t, `{"member":{"memberID":"m1","email":"lead@example.com","name":"이샘플","role":"admin","status":"active"}}`)

	saved, errorValue := client.SaveMember(context.Background(), MemberWrite{Email: " Lead@Example.com ", Name: "이샘플", Role: "admin"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if call.method != http.MethodPost || call.path != "/api/agent/member" {
		t.Fatalf("the write went to %s %s", call.method, call.path)
	}
	if call.authorization != "Bearer agent-key" {
		t.Fatalf("the write named itself with %q", call.authorization)
	}
	if call.body["email"] != "lead@example.com" || call.body["name"] != "이샘플" || call.body["role"] != "admin" {
		t.Fatalf("the write carried %v", call.body)
	}
	if _, carried := call.body["note"]; carried {
		t.Fatalf("a note nobody offered was written anyway: %v", call.body)
	}
	if saved.MemberID != "m1" || saved.Role != "admin" || saved.Status != "active" {
		t.Fatalf("the answer arrived reshaped: %+v", saved)
	}
}

func TestSaveMemberCarriesTheNoteWhenOneIsOffered(t *testing.T) {
	client, call := memberDoorForTest(t, `{"member":{"memberID":"m1","email":"lead@example.com","note":"HR follow-up","role":"member","status":"active"}}`)

	saved, errorValue := client.SaveMember(context.Background(), MemberWrite{Email: "lead@example.com", Note: "HR follow-up"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if call.body["note"] != "HR follow-up" {
		t.Fatalf("the note was not written: %v", call.body)
	}
	if _, carried := call.body["name"]; carried {
		t.Fatalf("a name nobody offered was written anyway: %v", call.body)
	}
	if saved.Note != "HR follow-up" {
		t.Fatalf("the note the company answered with was dropped: %+v", saved)
	}
}

func TestSaveMemberRefusesAnAnswerThatNamesNobody(t *testing.T) {
	client, _ := memberDoorForTest(t, `{}`)
	if _, errorValue := client.SaveMember(context.Background(), MemberWrite{Email: "lead@example.com"}); errorValue == nil {
		t.Fatal("an answer with no member was accepted")
	}
}

func TestWithdrawMemberDeletesThroughTheMemberDoor(t *testing.T) {
	client, call := memberDoorForTest(t, `{"member":{"memberID":"m1","email":"gone@example.com","status":"withdrawn"}}`)

	if errorValue := client.WithdrawMember(context.Background(), " Gone@Example.com "); errorValue != nil {
		t.Fatal(errorValue)
	}
	if call.method != http.MethodDelete || call.path != "/api/agent/member" || call.query != "email=gone%40example.com" {
		t.Fatalf("the withdrawal went to %s %s?%s", call.method, call.path, call.query)
	}
	if call.authorization != "Bearer agent-key" {
		t.Fatalf("the withdrawal named itself with %q", call.authorization)
	}
}

func TestAMemberWriteNeedsAnAddress(t *testing.T) {
	client, call := memberDoorForTest(t, `{}`)
	if _, errorValue := client.SaveMember(context.Background(), MemberWrite{Name: "이샘플"}); errorValue == nil {
		t.Fatal("a write with no address was sent")
	}
	if errorValue := client.WithdrawMember(context.Background(), ""); errorValue == nil {
		t.Fatal("a withdrawal with no address was sent")
	}
	if call.method != "" {
		t.Fatalf("the company was asked anyway: %s %s", call.method, call.path)
	}
}
