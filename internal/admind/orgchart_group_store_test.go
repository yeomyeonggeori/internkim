package admind

import (
	"context"
	"database/sql"
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

func TestOrgchartStorePersistsGroupHierarchyAndPreorder(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	groups := []orgGroupRecord{
		{ID: "product", Name: "Product"},
		{ID: "engineering", Name: "Engineering", ParentID: "product"},
		{ID: "design", Name: "Design", ParentID: "product"},
		{ID: "sales", Name: "Sales"},
	}

	if errorValue := service.writeOrgchartGroups(ctx, groups); errorValue != nil {
		t.Fatal(errorValue)
	}
	reloaded, errorValue := service.readOrgchartGroups(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(reloaded) != len(groups) {
		t.Fatalf("groups = %#v; want %#v", reloaded, groups)
	}
	for index := range groups {
		if reloaded[index] != groups[index] {
			t.Fatalf("group %d = %#v; want %#v", index, reloaded[index], groups[index])
		}
	}
}

func TestOrgchartStoreRejectsUnknownGroupParent(t *testing.T) {
	service := newLocalUsersTestService(t)
	errorValue := service.writeOrgchartGroups(context.Background(), []orgGroupRecord{{ID: "engineering", Name: "Engineering", ParentID: "missing"}})
	if errorValue == nil {
		t.Fatal("error = nil; want unknown parent rejection")
	}
}

func TestOrgchartStoreRejectsGroupHierarchyCycle(t *testing.T) {
	service := newLocalUsersTestService(t)
	errorValue := service.writeOrgchartGroups(context.Background(), []orgGroupRecord{
		{ID: "product", Name: "Product", ParentID: "engineering"},
		{ID: "engineering", Name: "Engineering", ParentID: "product"},
	})
	if errorValue == nil {
		t.Fatal("error = nil; want hierarchy cycle rejection")
	}
}

func TestOrgchartStoreMigratesLegacyGroupsToRoot(t *testing.T) {
	service := newLocalUsersTestService(t)
	database, errorValue := sql.Open("sqlite", sqliteDatabaseDSN(service.orgchartDatabasePath()))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.Exec(`CREATE TABLE orgchart_groups (id TEXT PRIMARY KEY, name TEXT NOT NULL, position INTEGER NOT NULL)`); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.Exec(`INSERT INTO orgchart_groups(id, name, position) VALUES('legacy', 'Legacy', 0)`); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	groups, errorValue := service.readOrgchartGroups(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(groups) != 1 || groups[0] != (orgGroupRecord{ID: "legacy", Name: "Legacy"}) {
		t.Fatalf("groups = %#v; want migrated legacy root group", groups)
	}
}
