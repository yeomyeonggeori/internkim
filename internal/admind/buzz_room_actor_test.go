package admind

import "testing"

func TestARoomIsAdministeredByAnAdminStandingInIt(t *testing.T) {
	service := &Service{}
	heldRoles := map[string]string{"admin-pubkey": "owner", "someone-pubkey": "member"}

	secret := service.pickBuzzRoomActor(heldRoles, map[string]string{"admin-pubkey": "admin-secret"}, "agent-secret", "agent-pubkey", "bootstrap-secret")

	if secret != "admin-secret" {
		t.Fatalf("actor = %q, want the admin in the room", secret)
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
