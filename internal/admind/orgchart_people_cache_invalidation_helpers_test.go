package admind

import (
	"context"
	"testing"
)

func assertOrgchartPersonCacheFound(t *testing.T, service *Service, userID string, expected bool) {
	t.Helper()
	assertOrgchartCacheFound(t, service, orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: userID}, expected)
}

func assertOrgchartCacheFound(t *testing.T, service *Service, key orgchartPeopleCacheKey, expected bool) {
	t.Helper()
	snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(context.Background(), []orgchartPeopleCacheKey{key})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if snapshots[key].Found != expected {
		t.Fatalf("cache %v found = %t; want %t", key, snapshots[key].Found, expected)
	}
}

func preloadOrgchartIdentityCache(t *testing.T, service *Service, userID string, email string) {
	t.Helper()
	for _, key := range orgchartPersonCacheKeys(orgchartPersonIdentity{UserID: userID, Email: email}) {
		if _, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(context.Background(), key, 0, "", []byte(`{}`)); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
}

func assertOrgchartIdentityMutationActive(t *testing.T, service *Service, userID string, email string) {
	t.Helper()
	keys := orgchartPersonCacheKeys(orgchartPersonIdentity{UserID: userID, Email: email})
	snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(context.Background(), keys)
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

func assertOrgchartIdentityCacheFound(t *testing.T, service *Service, userID string, email string, expected bool) {
	t.Helper()
	for _, key := range orgchartPersonCacheKeys(orgchartPersonIdentity{UserID: userID, Email: email}) {
		assertOrgchartCacheFound(t, service, key, expected)
	}
}

func activeOrgchartPersonMutationKeys(t *testing.T, service *Service) []string {
	t.Helper()
	database, errorValue := service.openOrgchartDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	rows, errorValue := database.Query(`SELECT cache_key FROM orgchart_people_cache_states WHERE cache_kind = ? AND active_mutations > 0 ORDER BY cache_key`, string(orgchartPeopleCachePerson))
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

func newOrgchartProxyMutationTestService(t *testing.T) *Service {
	t.Helper()
	service := newLocalUsersTestService(t)
	service.Configuration.APIBaseURL = "https://api.intern.kim"
	service.Configuration.FleetIDPath = writeTestFile(t, "dc719d8e")
	service.Configuration.FleetSecretPath = writeTestFile(t, "secret-value")
	return service
}
