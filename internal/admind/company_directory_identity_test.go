package admind

import (
	"context"
	"testing"
)

func TestAHostWithNoDeviceFilesReadsWhoIsAskingFromTheCompanyDirectory(t *testing.T) {
	service := NewService(Configuration{})
	seatAdministratorInTheCompanyForTest(t, service, "lead@example.com", memberForTest("colleague@example.com", "박예시", "member"))
	ctx := context.Background()

	if !service.isTaskAdminEmail(ctx, "lead@example.com") {
		t.Fatal("an admin the company directory lists was not recognised as an admin")
	}
	if service.isTaskAdminEmail(ctx, "colleague@example.com") {
		t.Fatal("a member the company directory lists was recognised as an admin")
	}
	actor, found, errorValue := service.resolveUserActorByEmail(ctx, "lead@example.com")
	if errorValue != nil || !found || !actor.isAdmin() {
		t.Fatalf("the directory's admin resolved to %+v, found %v, error %v", actor, found, errorValue)
	}
	actor, found, errorValue = service.resolveUserActorByEmail(ctx, "colleague@example.com")
	if errorValue != nil || !found || actor.isAdmin() {
		t.Fatalf("the directory's member resolved to %+v, found %v, error %v", actor, found, errorValue)
	}
}
