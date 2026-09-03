package admind

import "testing"

func TestUserActorCarriesTheRoleTheDirectoryGaveIt(t *testing.T) {
	colleague := userActorFromAdminUserRecord(adminUserMutation{
		Email: "colleague@example.com",
		Role:  adminUserRoleMember,
	})
	if colleague.Role != adminUserRoleMember {
		t.Fatalf("role = %q, want %q", colleague.Role, adminUserRoleMember)
	}
	if colleague.isAdmin() {
		t.Fatal("a member should not be an admin")
	}

	administrator := userActorFromAdminUserRecord(adminUserMutation{
		Email: "admin@example.com",
		Role:  adminUserRoleAdmin,
	})
	if administrator.Role != adminUserRoleAdmin {
		t.Fatalf("role = %q, want %q", administrator.Role, adminUserRoleAdmin)
	}
	if !administrator.isAdmin() {
		t.Fatal("an admin should be an admin")
	}
}
