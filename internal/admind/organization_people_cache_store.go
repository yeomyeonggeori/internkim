package admind

type organizationPeopleCacheKind string

const (
	organizationPeopleCacheList         organizationPeopleCacheKind = "list"
	organizationPeopleCachePerson       organizationPeopleCacheKind = "person"
	organizationPeopleCacheGroups       organizationPeopleCacheKind = "groups"
	organizationPeopleCacheSingletonKey                             = "all"
)

type organizationPeopleCacheKey struct {
	Kind organizationPeopleCacheKind
	Key  string
}

type organizationPeopleCacheSnapshot struct {
	Revision        int64
	IsDirty         bool
	ActiveMutations int64
	SourceRevision  string
	SchemaVersion   int
	PayloadJSON     []byte
	Found           bool
}

func uniqueOrganizationPeopleCacheKeys(keys []organizationPeopleCacheKey) []organizationPeopleCacheKey {
	seen := make(map[organizationPeopleCacheKey]struct{}, len(keys))
	result := make([]organizationPeopleCacheKey, 0, len(keys))
	for _, key := range keys {
		if key.Kind == "" || key.Key == "" {
			continue
		}
		if _, found := seen[key]; found {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result
}
