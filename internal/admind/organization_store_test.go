package admind

import (
	"context"
	"strings"
	"testing"
)

func TestOrganizationStorePersistsProfilesAndGroups(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	profile := organizationProfile{
		UserID:            "user-member",
		Email:             "member@example.com",
		JobTitle:          "Product Manager",
		PositionLevel:     2,
		PrimaryGroupID:    "product",
		GroupIDs:          []string{"product", "growth"},
		SupervisorID:      "user-admin",
		ProjectIDs:        []string{"new-business", "retention"},
		TeamRole:          "제품 일정과 우선순위 관리",
		EmploymentStatus:  organizationEmploymentStatusLeave,
		IsOrganizationVisible: false,
	}

	if errorValue := service.writeOrganizationGroups(ctx, []orgGroupRecord{{ID: "product", Name: "제품"}, {ID: "growth", Name: "성장"}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeOrganizationProfiles(ctx, []organizationProfile{profile}); errorValue != nil {
		t.Fatal(errorValue)
	}

	groups, errorValue := service.readOrganizationGroups(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(groups) != 2 || groups[0].ID != "product" || groups[1].ID != "growth" {
		t.Fatalf("groups = %#v; want product and growth", groups)
	}
	profilesByEmail, errorValue := service.readOrganizationProfilesByEmail(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	reloaded := profilesByEmail["member@example.com"]
	if reloaded.JobTitle != profile.JobTitle || reloaded.PositionLevel != 2 || reloaded.PrimaryGroupID != "product" {
		t.Fatalf("reloaded profile = %#v", reloaded)
	}
	if strings.Join(reloaded.GroupIDs, ",") != "product,growth" {
		t.Fatalf("group ids = %#v", reloaded.GroupIDs)
	}
	if strings.Join(reloaded.ProjectIDs, ",") != "new-business,retention" {
		t.Fatalf("project ids = %#v", reloaded.ProjectIDs)
	}
	if reloaded.IsOrganizationVisible {
		t.Fatal("isOrganizationVisible = true; want false")
	}
}

func TestOrganizationStoreKeepsPrimaryGroupInGroupIDs(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	if errorValue := service.writeOrganizationProfiles(ctx, []organizationProfile{{
		UserID:            "user-member",
		Email:             "member@example.com",
		PrimaryGroupID:    "product",
		GroupIDs:          []string{"growth"},
		EmploymentStatus:  organizationEmploymentStatusActive,
		IsOrganizationVisible: true,
	}}); errorValue != nil {
		t.Fatal(errorValue)
	}

	profilesByEmail, errorValue := service.readOrganizationProfilesByEmail(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	profile := profilesByEmail["member@example.com"]
	if strings.Join(profile.GroupIDs, ",") != "product,growth" {
		t.Fatalf("group ids = %#v; want product and growth", profile.GroupIDs)
	}
}

func TestOrganizationSchemaRejectsInvalidEmploymentStatus(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	database, errorValue := service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()

	_, errorValue = database.ExecContext(ctx, `
	INSERT INTO organization_profiles(
		profile_key,
		user_id,
		email,
		job_title,
		position_level,
		primary_group_id,
		group_ids,
		supervisor_id,
		project_ids,
		team_role,
		employment_status,
		is_organization_visible,
		updated_at
	) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"user:user-member",
		"user-member",
		"member@example.com",
		"Product Manager",
		2,
		"product",
		`["product"]`,
		"",
		`[]`,
		"",
		"paused",
		1,
		"2026-06-25T00:00:00Z",
	)
	if errorValue == nil {
		t.Fatal("expected invalid employment status to be rejected")
	}
}

func TestOrganizationSchemaMigratesLegacyProfileConstraints(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	database, errorValue := service.openSQLiteDatabase(ctx, service.organizationDatabasePath(), nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `
CREATE TABLE organization_profiles (
	profile_key TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	email TEXT NOT NULL,
	job_title TEXT NOT NULL,
	position_level INTEGER NOT NULL,
	primary_group_id TEXT NOT NULL,
	group_ids TEXT NOT NULL,
	supervisor_id TEXT NOT NULL,
	project_ids TEXT NOT NULL,
	team_role TEXT NOT NULL,
	employment_status TEXT NOT NULL,
	is_organization_visible INTEGER NOT NULL,
	updated_at TEXT NOT NULL
)`); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `
INSERT INTO organization_profiles(
	profile_key,
	user_id,
	email,
	job_title,
	position_level,
	primary_group_id,
	group_ids,
	supervisor_id,
	project_ids,
	team_role,
	employment_status,
	is_organization_visible,
	updated_at
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"user:user-member",
		"user-member",
		"MEMBER@EXAMPLE.COM",
		" Product Manager ",
		-3,
		" product ",
		`["product"]`,
		" user-admin ",
		`["new-business"]`,
		" 제품 일정 관리 ",
		"paused",
		7,
		"2026-06-25T00:00:00Z",
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	database, errorValue = service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	schema, errorValue := readSQLiteTableSchema(ctx, database, "organization_profiles")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !hasOrganizationProfileConstraints(schema) {
		t.Fatalf("schema = %s; want organization profile constraints", schema)
	}
	profiles, errorValue := service.readOrganizationProfiles(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(profiles) != 1 {
		t.Fatalf("profiles = %#v; want one migrated profile", profiles)
	}
	profile := profiles[0]
	if profile.Email != "member@example.com" || profile.PositionLevel != 0 || profile.EmploymentStatus != organizationEmploymentStatusActive {
		t.Fatalf("profile = %#v; want normalized migrated profile", profile)
	}
	if !profile.IsOrganizationVisible {
		t.Fatal("isOrganizationVisible = false; want invalid legacy value normalized to visible")
	}
	_, errorValue = database.ExecContext(ctx, `
INSERT INTO organization_profiles(
	profile_key,
	user_id,
	email,
	job_title,
	position_level,
	primary_group_id,
	group_ids,
	supervisor_id,
	project_ids,
	team_role,
	employment_status,
	is_organization_visible,
	updated_at
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"user:invalid-member",
		"invalid-member",
		"invalid@example.com",
		"Product Manager",
		2,
		"product",
		`["product"]`,
		"",
		`[]`,
		"",
		"paused",
		1,
		"2026-06-25T00:00:00Z",
	)
	if errorValue == nil {
		t.Fatal("expected migrated schema to reject invalid employment status")
	}
}

func TestOrganizationProfileRequestClearsPrimaryGroupWithoutKeepingOldGroupID(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	if errorValue := service.writeOrganizationProfiles(ctx, []organizationProfile{{
		UserID:            "user-member",
		Email:             "member@example.com",
		JobTitle:          "Product Manager",
		PositionLevel:     2,
		PrimaryGroupID:    "product",
		GroupIDs:          []string{"product", "growth"},
		SupervisorID:      "user-admin",
		ProjectIDs:        []string{"new-business"},
		TeamRole:          "제품 일정 관리",
		EmploymentStatus:  organizationEmploymentStatusActive,
		IsOrganizationVisible: true,
	}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	emptyGroup := ""
	profiles, errorValue := service.organizationProfilesFromRequest(ctx, []organizationProfileRequest{{
		UserID: "user-member",
		Email:  "member@example.com",
		Group:  &emptyGroup,
	}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeOrganizationProfiles(ctx, profiles); errorValue != nil {
		t.Fatal(errorValue)
	}

	profilesByEmail, errorValue := service.readOrganizationProfilesByEmail(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	profile := profilesByEmail["member@example.com"]
	if profile.PrimaryGroupID != "" {
		t.Fatalf("primary group = %q; want empty", profile.PrimaryGroupID)
	}
	if strings.Join(profile.GroupIDs, ",") != "growth" {
		t.Fatalf("group ids = %#v; want growth only", profile.GroupIDs)
	}
}

func TestOrganizationProfileRequestReplacesPrimaryGroupWithoutKeepingOldPrimaryID(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	if errorValue := service.writeOrganizationProfiles(ctx, []organizationProfile{{
		UserID:            "user-member",
		Email:             "member@example.com",
		JobTitle:          "Product Manager",
		PositionLevel:     2,
		PrimaryGroupID:    "product",
		GroupIDs:          []string{"product", "growth"},
		SupervisorID:      "user-admin",
		ProjectIDs:        []string{"new-business"},
		TeamRole:          "제품 일정 관리",
		EmploymentStatus:  organizationEmploymentStatusActive,
		IsOrganizationVisible: true,
	}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	designGroup := "design"
	profiles, errorValue := service.organizationProfilesFromRequest(ctx, []organizationProfileRequest{{
		UserID: "user-member",
		Email:  "member@example.com",
		Group:  &designGroup,
	}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeOrganizationProfiles(ctx, profiles); errorValue != nil {
		t.Fatal(errorValue)
	}

	profilesByEmail, errorValue := service.readOrganizationProfilesByEmail(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	profile := profilesByEmail["member@example.com"]
	if profile.PrimaryGroupID != "design" {
		t.Fatalf("primary group = %q; want design", profile.PrimaryGroupID)
	}
	if strings.Join(profile.GroupIDs, ",") != "design,growth" {
		t.Fatalf("group ids = %#v; want design and growth", profile.GroupIDs)
	}
}

func TestOrganizationProfileRequestKeepsExplicitPrimaryGroupInGroupIDs(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	productGroup := "product"
	growthGroups := []string{"growth"}
	profiles, errorValue := service.organizationProfilesFromRequest(ctx, []organizationProfileRequest{{
		UserID:         "user-member",
		Email:          "member@example.com",
		PrimaryGroupID: &productGroup,
		GroupIDs:       &growthGroups,
	}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeOrganizationProfiles(ctx, profiles); errorValue != nil {
		t.Fatal(errorValue)
	}

	profilesByEmail, errorValue := service.readOrganizationProfilesByEmail(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	profile := profilesByEmail["member@example.com"]
	if profile.PrimaryGroupID != "product" {
		t.Fatalf("primary group = %q; want product", profile.PrimaryGroupID)
	}
	if strings.Join(profile.GroupIDs, ",") != "product,growth" {
		t.Fatalf("group ids = %#v; want product and growth", profile.GroupIDs)
	}
}
