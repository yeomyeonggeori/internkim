package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

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

func TestUserActorRendersTheCompanyLanguageForACompanyMember(t *testing.T) {
	company := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/agent/member" {
			t.Fatalf("unexpected company path %s", request.URL.Path)
		}
		responseWriter.Write([]byte(`{"members":[{"memberID":"member-1","email":"member@example.com","name":"샘플 이","role":"member","status":"active"}]}`))
	}))
	defer company.Close()

	service := companyDeviceForTest(t, company.URL, "", "the-company-key")
	holdWorkspaceSettingsForTest(service, "Asia/Seoul", workspaceLanguageKorean)
	actor, found, errorValue := service.resolveUserActorFromUserRecords(context.Background(), "member@example.com")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatal("expected the active company member")
	}
	if actor.Name != "이샘플" {
		t.Fatalf("actor name = %q, want the rendered Korean name", actor.Name)
	}
}
