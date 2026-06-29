package admind

import (
	"context"
	"strings"
	"testing"
)

func TestOrgchartStoreDeduplicatesGroupsByName(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	if errorValue := service.writeOrgchartGroups(ctx, []orgGroupRecord{
		{ID: "engineering", Name: "Engineering"},
		{ID: "engineering-duplicate", Name: " engineering "},
		{ID: "engineering-uppercase", Name: "ENGINEERING"},
		{ID: "operations", Name: "Operations"},
		{ID: "operations", Name: "Operations"},
		{ID: "", Name: "No ID"},
		{ID: "no-name", Name: " "},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}

	groups, errorValue := service.readOrgchartGroups(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(groups) != 2 || groups[0].ID != "engineering" || groups[1].ID != "operations" {
		t.Fatalf("groups = %#v; want engineering and operations", groups)
	}
	if groups[0].Name != "Engineering" || groups[1].Name != "Operations" {
		t.Fatalf("group names = %#v; want trimmed first names", groups)
	}
}

func TestOrgchartStoreRewritesDuplicateGroupReferences(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	if errorValue := service.writeOrgchartProfiles(ctx, []orgchartProfile{{
		UserID:            "user-member",
		Email:             "member@example.com",
		PrimaryGroupID:    "engineering-duplicate",
		GroupIDs:          []string{"engineering-duplicate", "operations-duplicate", "external"},
		EmploymentStatus:  orgchartEmploymentStatusActive,
		IsOrgchartVisible: true,
	}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeOrgchartGroups(ctx, []orgGroupRecord{
		{ID: "engineering", Name: "Engineering"},
		{ID: "engineering-duplicate", Name: " engineering "},
		{ID: "operations", Name: "Operations"},
		{ID: "operations-duplicate", Name: "OPERATIONS"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}

	profilesByEmail, errorValue := service.readOrgchartProfilesByEmail(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	profile := profilesByEmail["member@example.com"]
	if profile.PrimaryGroupID != "engineering" {
		t.Fatalf("primary group ID = %q; want engineering", profile.PrimaryGroupID)
	}
	if strings.Join(profile.GroupIDs, ",") != "engineering,operations,external" {
		t.Fatalf("group IDs = %#v; want canonical groups with external preserved", profile.GroupIDs)
	}
}

func TestImportOrgchartGroupsIfUninitializedDoesNotOverwriteInitializedGroups(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	if errorValue := service.writeOrgchartGroups(ctx, []orgGroupRecord{{ID: "current", Name: "현재"}}); errorValue != nil {
		t.Fatal(errorValue)
	}

	groups, isImported, errorValue := service.importOrgchartGroupsIfUninitialized(ctx, []orgGroupRecord{{ID: "legacy", Name: "Legacy"}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if isImported {
		t.Fatal("isImported = true; want false")
	}
	if len(groups) != 1 || groups[0].ID != "current" {
		t.Fatalf("groups = %#v; want current group", groups)
	}
	storedGroups, errorValue := service.readOrgchartGroups(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(storedGroups) != 1 || storedGroups[0].ID != "current" {
		t.Fatalf("stored groups = %#v; want current group", storedGroups)
	}
}
