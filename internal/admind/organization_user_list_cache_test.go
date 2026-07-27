package admind

import (
	"context"
	"testing"
)

func TestOrganizationPeopleCacheRejectsDuplicateListIdentities(t *testing.T) {
	testCases := []struct {
		name        string
		payloadJSON string
	}{
		{
			name:        "duplicate user ID",
			payloadJSON: `{"records":[{"userID":"user-1","email":"one@example.com"},{"userID":"user-1","email":"two@example.com"}]}`,
		},
		{
			name:        "normalized duplicate email",
			payloadJSON: `{"records":[{"userID":"user-1","email":" One@Example.com "},{"userID":"user-2","email":"one@example.com"}]}`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := newLocalUsersTestService(t)
			ctx := context.Background()
			key := organizationPeopleCacheKey{Kind: organizationPeopleCacheList, Key: organizationPeopleCacheSingletonKey}
			written, errorValue := service.writeOrganizationPeopleCachePayloadIfCurrent(ctx, key, 0, "", []byte(testCase.payloadJSON))
			if errorValue != nil || !written {
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
	}
}
