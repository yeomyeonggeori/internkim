package admind

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/buzzimport/mattermostadmin"
)

func TestMattermostChannelShapeMirrorsWhoCanRead(t *testing.T) {
	cases := []struct {
		name                string
		record              mattermostadmin.ChannelRecord
		expectedName        string
		expectedChannelType string
		expectedVisibility  string
	}{
		{
			name:                "public channel",
			record:              mattermostadmin.ChannelRecord{ID: "ch-1", Name: "town-square", DisplayName: "Town Square", Type: "O"},
			expectedName:        "Town Square",
			expectedChannelType: "stream",
			expectedVisibility:  "open",
		},
		{
			name:                "private channel",
			record:              mattermostadmin.ChannelRecord{ID: "ch-2", Name: "leadership", DisplayName: "Leadership", Type: "P"},
			expectedName:        "Leadership",
			expectedChannelType: "stream",
			expectedVisibility:  "private",
		},
		{
			name:                "direct conversation",
			record:              mattermostadmin.ChannelRecord{ID: "ch-3", Name: "user-a__user-b", Type: "D"},
			expectedName:        "user-a__user-b",
			expectedChannelType: "dm",
			expectedVisibility:  "private",
		},
		{
			name:                "group conversation without a name",
			record:              mattermostadmin.ChannelRecord{ID: "ch-4", Type: "G"},
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

func TestAPrivateRoomMirrorCarriesTheRoomsOwnName(t *testing.T) {
	shape := describeMattermostChannel(mattermostadmin.ChannelRecord{
		Name:        "board-room",
		DisplayName: "Board room",
		Type:        "P",
	}, "channel-1")

	if shape.RoomName != "board-room" {
		t.Fatalf("room name = %q; a mirrored room keeps the name it was opened under", shape.RoomName)
	}
	if shape.Visibility != "private" {
		t.Fatalf("visibility = %q; a private room must not become a channel anyone can walk into", shape.Visibility)
	}
}

func TestAnArchivedRoomIsSeenAsArchived(t *testing.T) {
	shape := describeMattermostChannel(mattermostadmin.ChannelRecord{
		Name:     "client-qa",
		Type:     "P",
		DeleteAt: 1756000000000,
	}, "channel-3")

	if !shape.IsArchived {
		t.Fatal("archiving an already archived room fails, so the room's own state has to be read first")
	}
}
