package admind

import "strings"

const orgchartPersonEmailCacheKeyPrefix = "email:"

type orgchartPersonIdentity struct {
	UserID string
	Email  string
}

func orgchartPersonCacheKey(userID string, email string) (orgchartPeopleCacheKey, bool) {
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID != "" {
		return orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: normalizedUserID}, true
	}
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return orgchartPeopleCacheKey{}, false
	}
	return orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: orgchartPersonEmailCacheKeyPrefix + normalizedEmail}, true
}

func orgchartPersonCacheKeys(identity orgchartPersonIdentity) []orgchartPeopleCacheKey {
	keys := []orgchartPeopleCacheKey{}
	if userIDKey, found := orgchartPersonCacheKey(identity.UserID, ""); found {
		keys = append(keys, userIDKey)
	}
	if emailKey, found := orgchartPersonCacheKey("", identity.Email); found {
		keys = append(keys, emailKey)
	}
	return keys
}
