package admind

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestOrganizationPeopleCacheRebuildsLegacyUnsafePayloads(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	legacyRecordJSON := `{"userID":"user-1","email":"one@example.com","name":"Legacy","role":"admin","circles":["staff","admin"],"note":"legacy note","mattermostUserID":"mattermost-1","mattermostUsername":"legacy"}`
	seedLegacyOrganizationPeopleCacheEntry(t, service, organizationPeopleCacheKey{Kind: organizationPeopleCacheList, Key: organizationPeopleCacheSingletonKey}, `{"records":[`+legacyRecordJSON+`]}`)
	seedLegacyOrganizationPeopleCacheEntry(t, service, organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: "user-1"}, `{"record":`+legacyRecordJSON+`}`)
	loadCount := 0
	users, cachePolicy, errorValue := service.readCachedOrganizationUserList(ctx, func(context.Context) (pagesUsersResponse, error) {
		loadCount++
		return pagesUsersResponse{Records: []adminUserMutation{{
			UserID:             "user-1",
			Email:              "one@example.com",
			Name:               "Fresh",
			Role:               "member",
			Circles:            []string{"staff"},
			Note:               "fresh note",
			MattermostUserID:   "mattermost-1",
			MattermostUsername: "fresh",
		}}}, nil
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	response, errorValue := service.applyOrganizationPeople(ctx, users, cachePolicy)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if loadCount != 1 || response.response.Records[0].Name != "Fresh" {
		t.Fatalf("load count = %d response = %#v", loadCount, response.response)
	}

	keys := []organizationPeopleCacheKey{
		{Kind: organizationPeopleCacheList, Key: organizationPeopleCacheSingletonKey},
		{Kind: organizationPeopleCachePerson, Key: "user-1"},
	}
	snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(ctx, keys)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, key := range keys {
		snapshot := snapshots[key]
		if !snapshot.Found || snapshot.SchemaVersion != organizationPeopleCacheSchemaVersion {
			t.Fatalf("cache snapshot for %#v = %#v", key, snapshot)
		}
		assertOrganizationCachePayloadExcludesForbiddenFields(t, snapshot.PayloadJSON)
	}
}

func TestOrganizationPeopleCacheListMissAndHitReturnSameSafeProjection(t *testing.T) {
	service := newLocalUsersTestService(t)
	loadCount := 0
	loader := func(context.Context) (pagesUsersResponse, error) {
		loadCount++
		return pagesUsersResponse{
			Records: []adminUserMutation{{
				UserID:                 "user-1",
				Handle:                 "user-one",
				Name:                   "User One",
				Email:                  "one@example.com",
				Role:                   "admin",
				Circles:                []string{"staff", "admin"},
				Note:                   "private note",
				MattermostUserID:       "mattermost-1",
				MattermostUsername:     "user-one",
				Status:                 "active",
				TemporaryPassword:      "temporary-secret",
				TemporaryPasswordEmail: "temporary@example.com",
				JobTitle:               "Engineer",
			}},
			AvailableCircles: []adminCircleRecord{{CircleID: "staff", DisplayName: "Staff"}},
			AvailableGroups:  []orgGroupRecord{{ID: "engineering", Name: "Engineering"}},
		}, nil
	}
	first, _, errorValue := service.readCachedOrganizationUserList(context.Background(), loader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	second, _, errorValue := service.readCachedOrganizationUserList(context.Background(), loader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if loadCount != 1 {
		t.Fatalf("load count = %d; want 1", loadCount)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("miss = %#v hit = %#v", first, second)
	}
	firstJSON, errorValue := json.Marshal(first)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	secondJSON, errorValue := json.Marshal(second)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("miss JSON = %s hit JSON = %s", firstJSON, secondJSON)
	}
	record := first.Records[0]
	if record.Role != "" || len(record.Circles) != 0 || record.Note != "" || record.MattermostUserID != "" || record.MattermostUsername != "" || record.Status != "" || record.TemporaryPassword != "" || record.TemporaryPasswordEmail != "" {
		t.Fatalf("safe record = %#v", record)
	}
	if len(first.AvailableCircles) != 0 || len(first.AvailableGroups) != 1 {
		t.Fatalf("safe response = %#v", first)
	}
}

func TestOrganizationPeopleCacheExcludesForbiddenFields(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	record := adminUserMutation{
		UserID:                 "user-1",
		Handle:                 "user-one",
		Name:                   "User One",
		Email:                  "one@example.com",
		Image:                  "/images/user-1.png",
		HireDate:               "2026-01-01",
		Note:                   "private note",
		Role:                   "admin",
		Circles:                []string{"staff", "admin"},
		JobTitle:               "Engineer",
		Group:                  "engineering",
		PositionLevel:          3,
		PrimaryGroupID:         "engineering",
		GroupIDs:               []string{"engineering"},
		SupervisorID:           "user-2",
		ProjectIDs:             []string{"project-1"},
		TeamRole:               "Backend",
		EmploymentStatus:       organizationEmploymentStatusActive,
		IsOrganizationVisible:      true,
		MattermostUserID:       "mattermost-user-1",
		MattermostUsername:     "user-one",
		Status:                 "active",
		TemporaryPassword:      "temporary-secret",
		TemporaryPasswordEmail: "temporary@example.com",
	}
	users, cachePolicy, errorValue := service.readCachedOrganizationUserList(ctx, func(context.Context) (pagesUsersResponse, error) {
		return pagesUsersResponse{Records: []adminUserMutation{record}}, nil
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.applyOrganizationPeople(ctx, users, cachePolicy); errorValue != nil {
		t.Fatal(errorValue)
	}

	keys := []organizationPeopleCacheKey{
		{Kind: organizationPeopleCacheList, Key: organizationPeopleCacheSingletonKey},
		{Kind: organizationPeopleCachePerson, Key: "user-1"},
	}
	snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(ctx, keys)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, key := range keys {
		payloadJSON := snapshots[key].PayloadJSON
		if !snapshots[key].Found {
			t.Fatalf("cache entry not found for %#v", key)
		}
		assertOrganizationCachePayloadExcludesForbiddenFields(t, payloadJSON)
	}
}

func seedLegacyOrganizationPeopleCacheEntry(t *testing.T, service *Service, key organizationPeopleCacheKey, payloadJSON string) {
	t.Helper()
	database, errorValue := service.openOrganizationDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.Exec(`INSERT INTO organization_people_cache_entries(cache_kind, cache_key, source_revision, schema_version, payload_json, cached_at) VALUES(?, ?, '', 1, ?, '2026-07-14T00:00:00Z')`, string(key.Kind), key.Key, payloadJSON); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func assertOrganizationCachePayloadExcludesForbiddenFields(t *testing.T, payloadJSON []byte) {
	t.Helper()
	for _, forbiddenKey := range []string{
		"role", "circles", "note", "mattermostUserID", "mattermostUsername",
		"status", "temporaryPassword", "temporaryPasswordEmail",
	} {
		if strings.Contains(string(payloadJSON), `"`+forbiddenKey+`"`) {
			t.Fatalf("cache payload contains %s: %s", forbiddenKey, payloadJSON)
		}
	}
}
