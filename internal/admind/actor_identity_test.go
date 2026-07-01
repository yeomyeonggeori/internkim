package admind

import "testing"

func TestUserActorFromAdminUserRecordPreservesOperationsAdminRole(t *testing.T) {
	actor := userActorFromAdminUserRecord(adminUserMutation{
		Email: "operator@example.com",
		Role:  "operationsAdmin",
	})

	if actor.Role != "operationsAdmin" {
		t.Fatalf("role = %q, want operationsAdmin", actor.Role)
	}
	if actor.isAdmin() {
		t.Fatal("operations admin should not be full admin")
	}
}
