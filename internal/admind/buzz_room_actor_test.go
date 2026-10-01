package admind

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestTheAgentSignsARoomItAdministersEvenWithAnAdminAlsoElevated(t *testing.T) {
	service := &Service{}
	heldRoles := map[string]string{"admin-pubkey": "owner", "agent-pubkey": "owner"}

	secret := service.pickBuzzRoomActor(heldRoles, map[string]string{"admin-pubkey": "admin-secret"}, "agent-secret", "agent-pubkey", "bootstrap-secret")

	if secret != "agent-secret" {
		t.Fatalf("actor = %q, want the agent to sign a room it administers rather than an admin who did not act", secret)
	}
}

func TestAnAdminSignsOnlyAsALastResortAndTheFallbackIsLogged(t *testing.T) {
	service := &Service{}
	heldRoles := map[string]string{"admin-pubkey": "owner", "someone-pubkey": "member"}
	output := captureLogs(t)

	secret := service.pickBuzzRoomActor(heldRoles, map[string]string{"admin-pubkey": "admin-secret"}, "agent-secret", "agent-pubkey", "bootstrap-secret")

	if secret != "admin-secret" {
		t.Fatalf("actor = %q, want the admin in the room while the agent does not administer it yet", secret)
	}
	if !strings.Contains(output.String(), "the agent does not administer this room yet") {
		t.Fatalf("log = %q, want one line naming the fallback to an admin", output.String())
	}
}

func TestARoomWithNoAdminIsAdministeredByTheAgent(t *testing.T) {
	service := &Service{}
	heldRoles := map[string]string{"agent-pubkey": "owner", "someone-pubkey": "member"}

	secret := service.pickBuzzRoomActor(heldRoles, map[string]string{"admin-pubkey": "admin-secret"}, "agent-secret", "agent-pubkey", "bootstrap-secret")

	if secret != "agent-secret" {
		t.Fatalf("actor = %q, want the agent", secret)
	}
}

func TestAnAdminWhoIsOnlyAMemberDoesNotSign(t *testing.T) {
	service := &Service{}
	heldRoles := map[string]string{"admin-pubkey": "member"}

	secret := service.pickBuzzRoomActor(heldRoles, map[string]string{"admin-pubkey": "admin-secret"}, "agent-secret", "agent-pubkey", "bootstrap-secret")

	if secret != "bootstrap-secret" {
		t.Fatalf("actor = %q, want the bootstrap key while nobody else administers the room", secret)
	}
}

func TestTheCompanyAccountLeavesOnlyARoomSomebodyElseAdministers(t *testing.T) {
	withAdmin := map[string]string{"bootstrap-pubkey": "owner", "admin-pubkey": "owner"}
	if !holdsAnotherAdministrator(withAdmin, "bootstrap-pubkey") {
		t.Fatal("a room with an admin owner is administered without the company account")
	}

	alone := map[string]string{"bootstrap-pubkey": "owner", "someone-pubkey": "member"}
	if holdsAnotherAdministrator(alone, "bootstrap-pubkey") {
		t.Fatal("a room the company account alone administers must keep it")
	}
}

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var output bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() {
		log.SetOutput(previousWriter)
	})
	return &output
}
