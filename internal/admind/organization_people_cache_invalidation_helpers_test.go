package admind

import (
	"context"
	"testing"
)

func assertOrganizationPersonCacheFound(t *testing.T, service *Service, userID string, expected bool) {
	t.Helper()
	assertOrganizationCacheFound(t, service, organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: userID}, expected)
}

func assertOrganizationCacheFound(t *testing.T, service *Service, key organizationPeopleCacheKey, expected bool) {
	t.Helper()
	snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(context.Background(), []organizationPeopleCacheKey{key})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if snapshots[key].Found != expected {
		t.Fatalf("cache %v found = %t; want %t", key, snapshots[key].Found, expected)
	}
}

func preloadOrganizationIdentityCache(t *testing.T, service *Service, userID string, email string) {
	t.Helper()
	for _, key := range organizationPersonCacheKeys(organizationPersonIdentity{UserID: userID, Email: email}) {
		if _, errorValue := service.writeOrganizationPeopleCachePayloadIfCurrent(context.Background(), key, 0, "", []byte(`{}`)); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
}

func assertOrganizationIdentityMutationActive(t *testing.T, service *Service, userID string, email string) {
	t.Helper()
	keys := organizationPersonCacheKeys(organizationPersonIdentity{UserID: userID, Email: email})
	snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(context.Background(), keys)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, key := range keys {
		snapshot := snapshots[key]
		if !snapshot.IsDirty || snapshot.ActiveMutations != 1 || snapshot.Found {
			t.Fatalf("identity mutation state for %v = %#v", key, snapshot)
		}
	}
}

func assertOrganizationIdentityCacheFound(t *testing.T, service *Service, userID string, email string, expected bool) {
	t.Helper()
	for _, key := range organizationPersonCacheKeys(organizationPersonIdentity{UserID: userID, Email: email}) {
		assertOrganizationCacheFound(t, service, key, expected)
	}
}

func activeOrganizationPersonMutationKeys(t *testing.T, service *Service) []string {
	t.Helper()
	database, errorValue := service.openOrganizationDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	rows, errorValue := database.Query(`SELECT cache_key FROM organization_people_cache_states WHERE cache_kind = ? AND active_mutations > 0 ORDER BY cache_key`, string(organizationPeopleCachePerson))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer rows.Close()
	keys := []string{}
	for rows.Next() {
		var key string
		if errorValue := rows.Scan(&key); errorValue != nil {
			t.Fatal(errorValue)
		}
		keys = append(keys, key)
	}
	if errorValue := rows.Err(); errorValue != nil {
		t.Fatal(errorValue)
	}
	return keys
}

func localUsersPolicyWithPerson(userID string, email string) string {
	return `{"people":[{"personID":"` + userID + `","emails":["` + email + `"]}],"circles":[]}`
}

func localUsersPolicyWithPersonAndCircleSync(userID string, email string) string {
	return `{"people":[{"personID":"` + userID + `","emails":["` + email + `"]}],"circles":[],"circleSync":{"mattermostPrivateChannels":[{"circleID":"staff","channelName":"circle-staff"}]}}`
}

func newOrganizationProxyMutationTestService(t *testing.T) *Service {
	t.Helper()
	service := newLocalUsersTestService(t)
	service.Configuration.APIBaseURL = "https://api.intern.kim"
	service.Configuration.FleetIDPath = writeTestFile(t, "dc719d8e")
	service.Configuration.FleetSecretPath = writeTestFile(t, "secret-value")
	return service
}
