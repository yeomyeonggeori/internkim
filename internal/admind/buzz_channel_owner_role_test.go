package admind

import "testing"

func TestAnAdminInARoomIsGivenTheOwnerRole(t *testing.T) {
	adminEmails := map[string]bool{"lee@example.com": true}

	if role := buzzChannelRoleFor(adminEmails, "Lee@Example.com "); role != buzzChannelOwnerRole {
		t.Fatalf("role = %q, want %q", role, buzzChannelOwnerRole)
	}
}

func TestEveryoneElseIsAddedWithoutNamingARole(t *testing.T) {
	adminEmails := map[string]bool{"lee@example.com": true}

	if role := buzzChannelRoleFor(adminEmails, "park@example.com"); role != "" {
		t.Fatalf("role = %q, want the room to keep whatever role they hold", role)
	}
}

func TestARoomWithNoAdminDirectoryNamesNoRole(t *testing.T) {
	if role := buzzChannelRoleFor(map[string]bool{}, "lee@example.com"); role != "" {
		t.Fatalf("role = %q, want no role when the directory answered nothing", role)
	}
}
