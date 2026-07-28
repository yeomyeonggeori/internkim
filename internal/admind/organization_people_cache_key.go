package admind

import "strings"

const organizationPersonEmailCacheKeyPrefix = "email:"

type organizationPersonIdentity struct {
	UserID string
	Email  string
}

func organizationPersonCacheKey(userID string, email string) (organizationPeopleCacheKey, bool) {
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID != "" {
		return organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: normalizedUserID}, true
	}
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return organizationPeopleCacheKey{}, false
	}
	return organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: organizationPersonEmailCacheKeyPrefix + normalizedEmail}, true
}

func organizationPersonCacheKeys(identity organizationPersonIdentity) []organizationPeopleCacheKey {
	keys := []organizationPeopleCacheKey{}
	if userIDKey, found := organizationPersonCacheKey(identity.UserID, ""); found {
		keys = append(keys, userIDKey)
	}
	if emailKey, found := organizationPersonCacheKey("", identity.Email); found {
		keys = append(keys, emailKey)
	}
	return keys
}
