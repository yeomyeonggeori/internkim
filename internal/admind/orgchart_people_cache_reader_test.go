package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestOrgchartPeopleCacheRebuildsLegacyUnsafePayloads(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	legacyRecordJSON := `{"userID":"user-1","email":"one@example.com","name":"Legacy","role":"admin","circles":["staff","admin"],"note":"legacy note","mattermostUserID":"mattermost-1","mattermostUsername":"legacy"}`
	seedLegacyOrgchartPeopleCacheEntry(t, service, orgchartPeopleCacheKey{Kind: orgchartPeopleCacheList, Key: orgchartPeopleCacheSingletonKey}, `{"records":[`+legacyRecordJSON+`]}`)
	seedLegacyOrgchartPeopleCacheEntry(t, service, orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: "user-1"}, `{"record":`+legacyRecordJSON+`}`)
	loadCount := 0
	users, cachePolicy, errorValue := service.readCachedOrgchartUserList(ctx, func(context.Context) (pagesUsersResponse, error) {
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
	response, errorValue := service.applyOrgchartPeople(ctx, users, cachePolicy)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if loadCount != 1 || response.response.Records[0].Name != "Fresh" {
		t.Fatalf("load count = %d response = %#v", loadCount, response.response)
	}

	keys := []orgchartPeopleCacheKey{
		{Kind: orgchartPeopleCacheList, Key: orgchartPeopleCacheSingletonKey},
		{Kind: orgchartPeopleCachePerson, Key: "user-1"},
	}
	snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(ctx, keys)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, key := range keys {
		snapshot := snapshots[key]
		if !snapshot.Found || snapshot.SchemaVersion != orgchartPeopleCacheSchemaVersion {
			t.Fatalf("cache snapshot for %#v = %#v", key, snapshot)
		}
		assertOrgchartCachePayloadExcludesForbiddenFields(t, snapshot.PayloadJSON)
	}
}

func TestOrgchartPeopleCacheListMissAndHitReturnSameSafeProjection(t *testing.T) {
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
	first, _, errorValue := service.readCachedOrgchartUserList(context.Background(), loader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	second, _, errorValue := service.readCachedOrgchartUserList(context.Background(), loader)
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

func TestOrgchartPeopleCacheExcludesForbiddenFields(t *testing.T) {
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
		EmploymentStatus:       orgchartEmploymentStatusActive,
		IsOrgchartVisible:      true,
		MattermostUserID:       "mattermost-user-1",
		MattermostUsername:     "user-one",
		Status:                 "active",
		TemporaryPassword:      "temporary-secret",
		TemporaryPasswordEmail: "temporary@example.com",
	}
	users, cachePolicy, errorValue := service.readCachedOrgchartUserList(ctx, func(context.Context) (pagesUsersResponse, error) {
		return pagesUsersResponse{Records: []adminUserMutation{record}}, nil
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.applyOrgchartPeople(ctx, users, cachePolicy); errorValue != nil {
		t.Fatal(errorValue)
	}

	keys := []orgchartPeopleCacheKey{
		{Kind: orgchartPeopleCacheList, Key: orgchartPeopleCacheSingletonKey},
		{Kind: orgchartPeopleCachePerson, Key: "user-1"},
	}
	snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(ctx, keys)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, key := range keys {
		payloadJSON := snapshots[key].PayloadJSON
		if !snapshots[key].Found {
			t.Fatalf("cache entry not found for %#v", key)
		}
		assertOrgchartCachePayloadExcludesForbiddenFields(t, payloadJSON)
	}
}

func seedLegacyOrgchartPeopleCacheEntry(t *testing.T, service *Service, key orgchartPeopleCacheKey, payloadJSON string) {
	t.Helper()
	database, errorValue := service.openOrgchartDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.Exec(`INSERT INTO orgchart_people_cache_entries(cache_kind, cache_key, source_revision, schema_version, payload_json, cached_at) VALUES(?, ?, '', 1, ?, '2026-07-14T00:00:00Z')`, string(key.Kind), key.Key, payloadJSON); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func assertOrgchartCachePayloadExcludesForbiddenFields(t *testing.T, payloadJSON []byte) {
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

func TestOrgchartPeopleCacheReusesUserList(t *testing.T) {
	service := newLocalUsersTestService(t)
	loadCount := 0
	loader := func(context.Context) (pagesUsersResponse, error) {
		loadCount++
		return pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Role: "member"}}}, nil
	}

	for range 2 {
		response, _, errorValue := service.readCachedOrgchartUserList(context.Background(), loader)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if len(response.Records) != 1 || response.Records[0].UserID != "user-1" {
			t.Fatalf("response = %#v", response)
		}
	}
	if loadCount != 1 {
		t.Fatalf("load count = %d; want 1", loadCount)
	}
}

func TestOrgchartPeopleCacheRefreshesUserListWhenSourceRevisionChanges(t *testing.T) {
	service := newLocalUsersTestService(t)
	statePath := filepath.Join(service.Configuration.StateDirectory, "users-sync.json")
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"1","users":["one@example.com"]}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	loadCount := 0
	loader := func(context.Context) (pagesUsersResponse, error) {
		loadCount++
		return pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-" + string(rune('0'+loadCount)), Email: "one@example.com", Role: "member"}}}, nil
	}
	first, _, errorValue := service.readCachedOrgchartUserList(context.Background(), loader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"2","users":["one@example.com"]}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	second, _, errorValue := service.readCachedOrgchartUserList(context.Background(), loader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if first.Records[0].UserID == second.Records[0].UserID || loadCount != 2 {
		t.Fatalf("first = %#v second = %#v load count = %d", first, second, loadCount)
	}
}

func TestOrgchartPeopleCacheInvalidatesPersonWhenPreviousListIsCorrupt(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	statePath := filepath.Join(service.Configuration.StateDirectory, "users-sync.json")
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"1"}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	listKey := orgchartPeopleCacheKey{Kind: orgchartPeopleCacheList, Key: orgchartPeopleCacheSingletonKey}
	if written, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(ctx, listKey, 0, "1", []byte(`{`)); errorValue != nil || !written {
		t.Fatalf("write corrupt list cache: written = %t error = %v", written, errorValue)
	}
	personKey := orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: "user-1"}
	personPayload, errorValue := json.Marshal(orgchartCachedPerson{Record: newOrgchartCachedUserRecord(adminUserMutation{UserID: "user-1", Email: "one@example.com", Name: "Before", Role: "member"})})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if written, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(ctx, personKey, 0, "", personPayload); errorValue != nil || !written {
		t.Fatalf("write person cache: written = %t error = %v", written, errorValue)
	}
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"2"}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	users, cachePolicy, errorValue := service.readCachedOrgchartUserList(ctx, func(context.Context) (pagesUsersResponse, error) {
		return pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Name: "After", Role: "member"}}}, nil
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	response, errorValue := service.applyOrgchartPeople(ctx, users, cachePolicy)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.response.Records[0].Name != "After" {
		t.Fatalf("name = %q; want After", response.response.Records[0].Name)
	}
}

func TestOrgchartPeopleCacheDoesNotApplyPersonEntryWhenSourceChangesDuringLoad(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	statePath := filepath.Join(service.Configuration.StateDirectory, "users-sync.json")
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"1"}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	personKey := orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: "user-1"}
	personPayload, errorValue := json.Marshal(orgchartCachedPerson{Record: newOrgchartCachedUserRecord(adminUserMutation{UserID: "user-1", Email: "one@example.com", Name: "Before", Role: "member"})})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if written, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(ctx, personKey, 0, "", personPayload); errorValue != nil || !written {
		t.Fatalf("write person cache: written = %t error = %v", written, errorValue)
	}
	users, cachePolicy, errorValue := service.readCachedOrgchartUserList(ctx, func(context.Context) (pagesUsersResponse, error) {
		if errorValue := os.WriteFile(statePath, []byte(`{"revision":"2"}`), 0o600); errorValue != nil {
			return pagesUsersResponse{}, errorValue
		}
		return pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Name: "After", Role: "member"}}}, nil
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	response, errorValue := service.applyOrgchartPeople(ctx, users, cachePolicy)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.response.Records[0].Name != "After" {
		t.Fatalf("name = %q; want After", response.response.Records[0].Name)
	}
}

func TestOrgchartPeopleCacheBypassesPersonEntriesWhenListWriteLosesRevisionRace(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	listKey := orgchartPeopleCacheKey{Kind: orgchartPeopleCacheList, Key: orgchartPeopleCacheSingletonKey}
	_, cachePolicy, errorValue := service.readCachedOrgchartUserList(ctx, func(context.Context) (pagesUsersResponse, error) {
		if errorValue := service.beginOrgchartPeopleCacheMutation(ctx, []orgchartPeopleCacheKey{listKey}); errorValue != nil {
			return pagesUsersResponse{}, errorValue
		}
		if errorValue := service.completeOrgchartPeopleCacheMutation(ctx, []orgchartPeopleCacheKey{listKey}); errorValue != nil {
			return pagesUsersResponse{}, errorValue
		}
		return pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Role: "member"}}}, nil
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if cachePolicy != orgchartPeopleCacheBypassed {
		t.Fatalf("cache policy = %#v; want bypassed", cachePolicy)
	}
}

func TestOrgchartPeopleCacheDoesNotRebuildPersonAfterListReadRevisionChanges(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	loader := func(context.Context) (pagesUsersResponse, error) {
		return pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Name: "Before", Role: "member"}}}, nil
	}
	users, cachePolicy, errorValue := service.readCachedOrgchartUserList(ctx, loader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.applyOrgchartPeople(ctx, users, cachePolicy); errorValue != nil {
		t.Fatal(errorValue)
	}
	users, cachePolicy, errorValue = service.readCachedOrgchartUserList(ctx, loader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	identities := []orgchartPersonIdentity{{UserID: "user-1", Email: "one@example.com"}}
	if errorValue := service.beginOrgchartUserMutation(ctx, identities); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.completeOrgchartUserMutation(ctx, identities); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.applyOrgchartPeople(ctx, users, cachePolicy); errorValue != nil {
		t.Fatal(errorValue)
	}

	assertOrgchartPersonCacheFound(t, service, "user-1", false)
}

func TestOrgchartPeopleCacheDoesNotApplyPersonAfterSourceChangesFollowingListRead(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	statePath := filepath.Join(service.Configuration.StateDirectory, "users-sync.json")
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"1"}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	users, cachePolicy, errorValue := service.readCachedOrgchartUserList(ctx, func(context.Context) (pagesUsersResponse, error) {
		return pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Name: "Source", Role: "member"}}}, nil
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	personPayload, errorValue := json.Marshal(orgchartCachedPerson{Record: newOrgchartCachedUserRecord(adminUserMutation{UserID: "user-1", Email: "one@example.com", Name: "Cached", Role: "member"})})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	personKey := orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: "user-1"}
	snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(ctx, []orgchartPeopleCacheKey{personKey})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if written, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(ctx, personKey, snapshots[personKey].Revision, "", personPayload); errorValue != nil || !written {
		t.Fatalf("write person cache: written = %t error = %v", written, errorValue)
	}
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"2"}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	response, errorValue := service.applyOrgchartPeople(ctx, users, cachePolicy)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.response.Records[0].Name != "Source" {
		t.Fatalf("name = %q; want Source", response.response.Records[0].Name)
	}
}

func TestOrgchartPeopleCacheReusesPersonEntry(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	if errorValue := service.writeOrgchartProfiles(ctx, []orgchartProfile{{
		UserID:            "user-1",
		Email:             "one@example.com",
		JobTitle:          "Designer",
		EmploymentStatus:  orgchartEmploymentStatusActive,
		IsOrgchartVisible: true,
	}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	users := pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Role: "member"}}}
	first, errorValue := service.applyCachedOrgchartPeople(ctx, users)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openOrgchartDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE orgchart_profiles SET job_title = 'Engineer' WHERE user_id = 'user-1'`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	database.Close()
	second, errorValue := service.applyCachedOrgchartPeople(ctx, users)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if first.response.Records[0].JobTitle != "Designer" || second.response.Records[0].JobTitle != "Designer" {
		t.Fatalf("first = %#v second = %#v", first.response, second.response)
	}
}

func TestOrgchartPeopleCacheRebuildsCorruptPersonPayload(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	key := orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: "user-1"}
	written, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(ctx, key, 0, "", []byte(`{`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !written {
		t.Fatal("expected corrupt fixture write")
	}
	users := pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Role: "member"}}}

	response, errorValue := service.applyCachedOrgchartPeople(ctx, users)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(response.response.Records) != 1 || response.response.Records[0].UserID != "user-1" {
		t.Fatalf("response = %#v", response.response)
	}
}

func TestOrgchartPeopleCacheRejectsSemanticCorruption(t *testing.T) {
	t.Run("missing list records", func(t *testing.T) {
		service := newLocalUsersTestService(t)
		ctx := context.Background()
		key := orgchartPeopleCacheKey{Kind: orgchartPeopleCacheList, Key: orgchartPeopleCacheSingletonKey}
		if written, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(ctx, key, 0, "", []byte(`{}`)); errorValue != nil || !written {
			t.Fatalf("write corrupt list cache: written = %t error = %v", written, errorValue)
		}
		loadCount := 0
		response, _, errorValue := service.readCachedOrgchartUserList(ctx, func(context.Context) (pagesUsersResponse, error) {
			loadCount++
			return pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Name: "Fresh"}}}, nil
		})
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if loadCount != 1 || len(response.Records) != 1 || response.Records[0].Name != "Fresh" {
			t.Fatalf("load count = %d response = %#v", loadCount, response)
		}
	})

	personCases := []struct {
		name        string
		key         orgchartPeopleCacheKey
		payloadJSON string
	}{
		{name: "empty person", key: orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: "user-1"}, payloadJSON: `{}`},
		{name: "missing identity", key: orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: "user-1"}, payloadJSON: `{"record":{"name":"Corrupt"}}`},
		{name: "mismatched user ID", key: orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: "user-1"}, payloadJSON: `{"record":{"userID":"user-2","email":"two@example.com","name":"Corrupt"}}`},
		{name: "mismatched email", key: orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: "email:one@example.com"}, payloadJSON: `{"record":{"email":"two@example.com","name":"Corrupt"}}`},
	}
	for _, testCase := range personCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := newLocalUsersTestService(t)
			ctx := context.Background()
			if written, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(ctx, testCase.key, 0, "", []byte(testCase.payloadJSON)); errorValue != nil || !written {
				t.Fatalf("write corrupt person cache: written = %t error = %v", written, errorValue)
			}
			userID := "user-1"
			if strings.HasPrefix(testCase.key.Key, "email:") {
				userID = ""
			}
			users := pagesUsersResponse{Records: []adminUserMutation{{UserID: userID, Email: "one@example.com", Name: "Fresh"}}}
			response, errorValue := service.applyCachedOrgchartPeople(ctx, users)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if response.response.Records[0].Name != "Fresh" {
				t.Fatalf("response = %#v", response.response)
			}
			snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(ctx, []orgchartPeopleCacheKey{testCase.key})
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if !snapshots[testCase.key].Found || string(snapshots[testCase.key].PayloadJSON) == testCase.payloadJSON {
				t.Fatalf("rebuilt snapshot = %#v", snapshots[testCase.key])
			}
		})
	}

	profileCases := []struct {
		name        string
		profileJSON string
	}{
		{name: "mismatched profile identity", profileJSON: `{"userID":"user-2","email":"two@example.com","jobTitle":"Corrupt","employmentStatus":"active","isOrgchartVisible":true}`},
		{name: "invalid profile employment status", profileJSON: `{"userID":"user-1","email":"one@example.com","jobTitle":"Corrupt","employmentStatus":"unknown","isOrgchartVisible":true}`},
	}
	for _, testCase := range profileCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := newLocalUsersTestService(t)
			ctx := context.Background()
			if errorValue := service.writeOrgchartProfiles(ctx, []orgchartProfile{{
				UserID:            "user-1",
				Email:             "one@example.com",
				JobTitle:          "Authoritative",
				EmploymentStatus:  orgchartEmploymentStatusActive,
				IsOrgchartVisible: true,
			}}); errorValue != nil {
				t.Fatal(errorValue)
			}
			key := orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: "user-1"}
			payloadJSON := `{"record":{"userID":"user-1","email":"one@example.com","name":"Cached"},"profile":` + testCase.profileJSON + `}`
			snapshotsBeforeWrite, errorValue := service.readOrgchartPeopleCacheSnapshots(ctx, []orgchartPeopleCacheKey{key})
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if written, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(ctx, key, snapshotsBeforeWrite[key].Revision, "", []byte(payloadJSON)); errorValue != nil || !written {
				t.Fatalf("write corrupt person cache: written = %t error = %v", written, errorValue)
			}

			users := pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Name: "Fresh"}}}
			response, errorValue := service.applyCachedOrgchartPeople(ctx, users)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if record := response.response.Records[0]; record.Name != "Fresh" || record.JobTitle != "Authoritative" {
				t.Fatalf("response record = %#v", record)
			}

			snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(ctx, []orgchartPeopleCacheKey{key})
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			var rebuiltPerson orgchartCachedPerson
			if errorValue := json.Unmarshal(snapshots[key].PayloadJSON, &rebuiltPerson); errorValue != nil {
				t.Fatal(errorValue)
			}
			if rebuiltPerson.Profile == nil || rebuiltPerson.Profile.UserID != "user-1" || rebuiltPerson.Profile.Email != "one@example.com" || rebuiltPerson.Profile.JobTitle != "Authoritative" || rebuiltPerson.Profile.EmploymentStatus != orgchartEmploymentStatusActive {
				t.Fatalf("rebuilt person = %#v", rebuiltPerson)
			}
		})
	}
}

func TestOrgchartPeopleCacheAcceptsUserIDOnlyPersonPayload(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	key := orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: "user-1"}
	payloadJSON := []byte(`{"record":{"userID":"user-1","name":"Cached"}}`)
	if written, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(ctx, key, 0, "", payloadJSON); errorValue != nil || !written {
		t.Fatalf("write person cache: written = %t error = %v", written, errorValue)
	}
	users := pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Name: "Fresh"}}}
	response, errorValue := service.applyCachedOrgchartPeople(ctx, users)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.response.Records[0].Name != "Cached" {
		t.Fatalf("response = %#v", response.response)
	}
}

func TestOrgchartGroupCacheHitAndCorruptRebuild(t *testing.T) {
	t.Run("hit", func(t *testing.T) {
		service := newLocalUsersTestService(t)
		ctx := context.Background()
		if errorValue := service.writeOrgchartGroups(ctx, []orgGroupRecord{{ID: "engineering", Name: "Engineering"}}); errorValue != nil {
			t.Fatal(errorValue)
		}
		first, errorValue := service.readCachedOrgchartGroups(ctx, nil)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		database, errorValue := service.openOrgchartDatabase(ctx)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if _, errorValue := database.ExecContext(ctx, `UPDATE orgchart_groups SET name = 'Changed' WHERE id = 'engineering'`); errorValue != nil {
			database.Close()
			t.Fatal(errorValue)
		}
		if errorValue := database.Close(); errorValue != nil {
			t.Fatal(errorValue)
		}
		second, errorValue := service.readCachedOrgchartGroups(ctx, nil)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if !reflect.DeepEqual(first, second) || second[0].Name != "Engineering" {
			t.Fatalf("first = %#v second = %#v", first, second)
		}
	})

	t.Run("corrupt", func(t *testing.T) {
		service := newLocalUsersTestService(t)
		ctx := context.Background()
		if errorValue := service.writeOrgchartGroups(ctx, []orgGroupRecord{{ID: "engineering", Name: "Engineering"}}); errorValue != nil {
			t.Fatal(errorValue)
		}
		key := orgchartPeopleCacheKey{Kind: orgchartPeopleCacheGroups, Key: orgchartPeopleCacheSingletonKey}
		snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(ctx, []orgchartPeopleCacheKey{key})
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		corruptPayload := []byte(`[{"id":"","name":"Corrupt"}]`)
		if written, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(ctx, key, snapshots[key].Revision, "", corruptPayload); errorValue != nil || !written {
			t.Fatalf("write corrupt group cache: written = %t error = %v", written, errorValue)
		}
		groups, errorValue := service.readCachedOrgchartGroups(ctx, nil)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if len(groups) != 1 || groups[0].ID != "engineering" || groups[0].Name != "Engineering" {
			t.Fatalf("groups = %#v", groups)
		}
	})
}

func TestOrgchartPeopleCacheHasNoTTL(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	if errorValue := service.writeOrgchartProfiles(ctx, []orgchartProfile{{
		UserID:            "user-1",
		Email:             "one@example.com",
		JobTitle:          "Designer",
		EmploymentStatus:  orgchartEmploymentStatusActive,
		IsOrgchartVisible: true,
	}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	users := pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com"}}}
	if _, errorValue := service.applyCachedOrgchartPeople(ctx, users); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openOrgchartDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE orgchart_people_cache_entries SET cached_at = '2000-01-01T00:00:00Z' WHERE cache_kind = 'person' AND cache_key = 'user-1'`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE orgchart_profiles SET job_title = 'Engineer' WHERE user_id = 'user-1'`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	response, errorValue := service.applyCachedOrgchartPeople(ctx, users)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.response.Records[0].JobTitle != "Designer" {
		t.Fatalf("response = %#v", response.response)
	}
}

func TestOrgchartPeopleCacheWritesMissesInSingleTransaction(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	database, errorValue := service.openOrgchartDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `
		CREATE TRIGGER fail_orgchart_people_cache_batch
		BEFORE INSERT ON orgchart_people_cache_entries
		WHEN NEW.cache_kind = 'person' AND NEW.cache_key = 'user-25'
		BEGIN
			SELECT RAISE(ABORT, 'forced person batch failure');
		END
	`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	users := pagesUsersResponse{Records: make([]adminUserMutation, 0, 50)}
	for index := range 50 {
		users.Records = append(users.Records, adminUserMutation{
			UserID: fmt.Sprintf("user-%02d", index),
			Email:  fmt.Sprintf("user-%02d@example.com", index),
		})
	}
	if _, errorValue := service.applyCachedOrgchartPeople(ctx, users); errorValue == nil {
		t.Fatal("expected forced batch write failure")
	}
	database, errorValue = service.openOrgchartDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var entryCount int
	if errorValue := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM orgchart_people_cache_entries WHERE cache_kind = 'person'`).Scan(&entryCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if entryCount != 0 {
		t.Fatalf("person cache entries = %d; want atomic rollback", entryCount)
	}
}
