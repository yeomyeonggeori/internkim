package admind

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestOrganizationPeopleCacheReusesPersonEntry(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	if errorValue := service.writeOrganizationProfiles(ctx, []organizationProfile{{
		UserID:            "user-1",
		Email:             "one@example.com",
		JobTitle:          "Designer",
		EmploymentStatus:  organizationEmploymentStatusActive,
		IsOrganizationVisible: true,
	}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	users := pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Role: "member"}}}
	first, errorValue := service.applyCachedOrganizationPeople(ctx, users)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE organization_profiles SET job_title = 'Engineer' WHERE user_id = 'user-1'`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	database.Close()
	second, errorValue := service.applyCachedOrganizationPeople(ctx, users)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if first.response.Records[0].JobTitle != "Designer" || second.response.Records[0].JobTitle != "Designer" {
		t.Fatalf("first = %#v second = %#v", first.response, second.response)
	}
}

func TestOrganizationPeopleCacheRebuildsCorruptPersonPayload(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	key := organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: "user-1"}
	written, errorValue := service.writeOrganizationPeopleCachePayloadIfCurrent(ctx, key, 0, "", []byte(`{`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !written {
		t.Fatal("expected corrupt fixture write")
	}
	users := pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Role: "member"}}}

	response, errorValue := service.applyCachedOrganizationPeople(ctx, users)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(response.response.Records) != 1 || response.response.Records[0].UserID != "user-1" {
		t.Fatalf("response = %#v", response.response)
	}
}

func TestOrganizationPeopleCacheRejectsSemanticCorruption(t *testing.T) {
	t.Run("missing list records", func(t *testing.T) {
		service := newLocalUsersTestService(t)
		ctx := context.Background()
		key := organizationPeopleCacheKey{Kind: organizationPeopleCacheList, Key: organizationPeopleCacheSingletonKey}
		if written, errorValue := service.writeOrganizationPeopleCachePayloadIfCurrent(ctx, key, 0, "", []byte(`{}`)); errorValue != nil || !written {
			t.Fatalf("write corrupt list cache: written = %t error = %v", written, errorValue)
		}
		loadCount := 0
		response, _, errorValue := service.readCachedOrganizationUserList(ctx, func(context.Context) (pagesUsersResponse, error) {
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
		key         organizationPeopleCacheKey
		payloadJSON string
	}{
		{name: "empty person", key: organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: "user-1"}, payloadJSON: `{}`},
		{name: "missing identity", key: organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: "user-1"}, payloadJSON: `{"record":{"name":"Corrupt"}}`},
		{name: "mismatched user ID", key: organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: "user-1"}, payloadJSON: `{"record":{"userID":"user-2","email":"two@example.com","name":"Corrupt"}}`},
		{name: "mismatched email", key: organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: "email:one@example.com"}, payloadJSON: `{"record":{"email":"two@example.com","name":"Corrupt"}}`},
	}
	for _, testCase := range personCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := newLocalUsersTestService(t)
			ctx := context.Background()
			if written, errorValue := service.writeOrganizationPeopleCachePayloadIfCurrent(ctx, testCase.key, 0, "", []byte(testCase.payloadJSON)); errorValue != nil || !written {
				t.Fatalf("write corrupt person cache: written = %t error = %v", written, errorValue)
			}
			userID := "user-1"
			if strings.HasPrefix(testCase.key.Key, "email:") {
				userID = ""
			}
			users := pagesUsersResponse{Records: []adminUserMutation{{UserID: userID, Email: "one@example.com", Name: "Fresh"}}}
			response, errorValue := service.applyCachedOrganizationPeople(ctx, users)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if response.response.Records[0].Name != "Fresh" {
				t.Fatalf("response = %#v", response.response)
			}
			snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(ctx, []organizationPeopleCacheKey{testCase.key})
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
		{name: "mismatched profile identity", profileJSON: `{"userID":"user-2","email":"two@example.com","jobTitle":"Corrupt","employmentStatus":"active","isOrganizationVisible":true}`},
		{name: "invalid profile employment status", profileJSON: `{"userID":"user-1","email":"one@example.com","jobTitle":"Corrupt","employmentStatus":"unknown","isOrganizationVisible":true}`},
	}
	for _, testCase := range profileCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := newLocalUsersTestService(t)
			ctx := context.Background()
			if errorValue := service.writeOrganizationProfiles(ctx, []organizationProfile{{
				UserID:            "user-1",
				Email:             "one@example.com",
				JobTitle:          "Authoritative",
				EmploymentStatus:  organizationEmploymentStatusActive,
				IsOrganizationVisible: true,
			}}); errorValue != nil {
				t.Fatal(errorValue)
			}
			key := organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: "user-1"}
			payloadJSON := `{"record":{"userID":"user-1","email":"one@example.com","name":"Cached"},"profile":` + testCase.profileJSON + `}`
			snapshotsBeforeWrite, errorValue := service.readOrganizationPeopleCacheSnapshots(ctx, []organizationPeopleCacheKey{key})
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if written, errorValue := service.writeOrganizationPeopleCachePayloadIfCurrent(ctx, key, snapshotsBeforeWrite[key].Revision, "", []byte(payloadJSON)); errorValue != nil || !written {
				t.Fatalf("write corrupt person cache: written = %t error = %v", written, errorValue)
			}

			users := pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Name: "Fresh"}}}
			response, errorValue := service.applyCachedOrganizationPeople(ctx, users)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if record := response.response.Records[0]; record.Name != "Fresh" || record.JobTitle != "Authoritative" {
				t.Fatalf("response record = %#v", record)
			}

			snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(ctx, []organizationPeopleCacheKey{key})
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			var rebuiltPerson organizationCachedPerson
			if errorValue := json.Unmarshal(snapshots[key].PayloadJSON, &rebuiltPerson); errorValue != nil {
				t.Fatal(errorValue)
			}
			if rebuiltPerson.Profile == nil || rebuiltPerson.Profile.UserID != "user-1" || rebuiltPerson.Profile.Email != "one@example.com" || rebuiltPerson.Profile.JobTitle != "Authoritative" || rebuiltPerson.Profile.EmploymentStatus != organizationEmploymentStatusActive {
				t.Fatalf("rebuilt person = %#v", rebuiltPerson)
			}
		})
	}
}

func TestOrganizationPeopleCacheAcceptsUserIDOnlyPersonPayload(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	key := organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: "user-1"}
	payloadJSON := []byte(`{"record":{"userID":"user-1","name":"Cached"}}`)
	if written, errorValue := service.writeOrganizationPeopleCachePayloadIfCurrent(ctx, key, 0, "", payloadJSON); errorValue != nil || !written {
		t.Fatalf("write person cache: written = %t error = %v", written, errorValue)
	}
	users := pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Name: "Fresh"}}}
	response, errorValue := service.applyCachedOrganizationPeople(ctx, users)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.response.Records[0].Name != "Cached" {
		t.Fatalf("response = %#v", response.response)
	}
}

func TestOrganizationPeopleCacheHasNoTTL(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	if errorValue := service.writeOrganizationProfiles(ctx, []organizationProfile{{
		UserID:            "user-1",
		Email:             "one@example.com",
		JobTitle:          "Designer",
		EmploymentStatus:  organizationEmploymentStatusActive,
		IsOrganizationVisible: true,
	}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	users := pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com"}}}
	if _, errorValue := service.applyCachedOrganizationPeople(ctx, users); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE organization_people_cache_entries SET cached_at = '2000-01-01T00:00:00Z' WHERE cache_kind = 'person' AND cache_key = 'user-1'`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE organization_profiles SET job_title = 'Engineer' WHERE user_id = 'user-1'`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	response, errorValue := service.applyCachedOrganizationPeople(ctx, users)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.response.Records[0].JobTitle != "Designer" {
		t.Fatalf("response = %#v", response.response)
	}
}
