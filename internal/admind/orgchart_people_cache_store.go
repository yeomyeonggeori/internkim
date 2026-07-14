package admind

type orgchartPeopleCacheKind string

const (
	orgchartPeopleCacheList         orgchartPeopleCacheKind = "list"
	orgchartPeopleCachePerson       orgchartPeopleCacheKind = "person"
	orgchartPeopleCacheGroups       orgchartPeopleCacheKind = "groups"
	orgchartPeopleCacheSingletonKey                         = "all"
)

type orgchartPeopleCacheKey struct {
	Kind orgchartPeopleCacheKind
	Key  string
}

type orgchartPeopleCacheSnapshot struct {
	Revision        int64
	IsDirty         bool
	ActiveMutations int64
	SourceRevision  string
	SchemaVersion   int
	PayloadJSON     []byte
	Found           bool
}

type orgchartPeopleCachePolicy struct {
	CanUsePersonCache         bool
	HasExpectedListRevision   bool
	ExpectedListRevision      int64
	HasExpectedSourceRevision bool
	ExpectedSourceRevision    string
}

var orgchartPeopleCacheEnabled = orgchartPeopleCachePolicy{CanUsePersonCache: true}
var orgchartPeopleCacheBypassed = orgchartPeopleCachePolicy{}

func orgchartPeopleCachePolicyForListRevision(revision int64, sourceRevision string) orgchartPeopleCachePolicy {
	return orgchartPeopleCachePolicy{
		CanUsePersonCache:         true,
		HasExpectedListRevision:   true,
		ExpectedListRevision:      revision,
		HasExpectedSourceRevision: true,
		ExpectedSourceRevision:    sourceRevision,
	}
}

func uniqueOrgchartPeopleCacheKeys(keys []orgchartPeopleCacheKey) []orgchartPeopleCacheKey {
	seen := make(map[orgchartPeopleCacheKey]struct{}, len(keys))
	result := make([]orgchartPeopleCacheKey, 0, len(keys))
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
