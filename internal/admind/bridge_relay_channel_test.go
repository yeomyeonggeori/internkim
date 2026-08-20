package admind

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMattermostChannelShapeMirrorsWhoCanRead(t *testing.T) {
	cases := []struct {
		name                string
		record              mattermostChannelRecord
		expectedName        string
		expectedChannelType string
		expectedVisibility  string
	}{
		{
			name:                "public channel",
			record:              mattermostChannelRecord{ID: "ch-1", Name: "town-square", DisplayName: "Town Square", Type: "O"},
			expectedName:        "Town Square",
			expectedChannelType: "stream",
			expectedVisibility:  "open",
		},
		{
			name:                "private channel",
			record:              mattermostChannelRecord{ID: "ch-2", Name: "leadership", DisplayName: "Leadership", Type: "P"},
			expectedName:        "Leadership",
			expectedChannelType: "stream",
			expectedVisibility:  "private",
		},
		{
			name:                "direct conversation",
			record:              mattermostChannelRecord{ID: "ch-3", Name: "user-a__user-b", Type: "D"},
			expectedName:        "user-a__user-b",
			expectedChannelType: "dm",
			expectedVisibility:  "private",
		},
		{
			name:                "group conversation without a name",
			record:              mattermostChannelRecord{ID: "ch-4", Type: "G"},
			expectedName:        "ch-4",
			expectedChannelType: "dm",
			expectedVisibility:  "private",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			shape := describeMattermostChannel(testCase.record, testCase.record.ID)
			if shape.Name != testCase.expectedName {
				t.Fatalf("name = %q, want %q", shape.Name, testCase.expectedName)
			}
			if shape.ChannelType != testCase.expectedChannelType {
				t.Fatalf("channel type = %q, want %q", shape.ChannelType, testCase.expectedChannelType)
			}
			if shape.Visibility != testCase.expectedVisibility {
				t.Fatalf("visibility = %q, want %q", shape.Visibility, testCase.expectedVisibility)
			}
		})
	}
}

func TestBridgeChannelResolvesWithoutARelayToWriteTo(t *testing.T) {
	service := &Service{Configuration: Configuration{
		BridgeMapDatabasePath: filepath.Join(t.TempDir(), "bridge-map.sqlite"),
		BuzzKeySeedPath:       writeTestFile(t, "seed-for-derivation"),
	}}
	buzzChannelID, errorValue := service.resolveBridgeChannel(context.Background(), "mattermost", "ch-1")
	if errorValue != nil {
		t.Fatalf("resolve: %v", errorValue)
	}
	if buzzChannelID == "" {
		t.Fatal("resolve returned no channel id")
	}
}

func TestBridgeRelayChannelIsNotCreatedForAnUnknownPlatform(t *testing.T) {
	service := &Service{Configuration: Configuration{
		BuzzKeySeedPath: writeTestFile(t, "seed-for-derivation"),
		BuzzRelayURL:    "ws://127.0.0.1:3000",
		BuzzDatabaseURL: "postgres://localhost/buzz",
	}}
	_, errorValue := service.describeBridgeRelayChannel(context.Background(), "slack", "ch-1")
	if errorValue != errBridgeChannelKindUnknown {
		t.Fatalf("shape error = %v, want %v", errorValue, errBridgeChannelKindUnknown)
	}
}
