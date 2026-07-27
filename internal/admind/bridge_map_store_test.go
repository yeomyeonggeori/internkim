package admind

import (
	"context"
	"path/filepath"
	"testing"
)

func newBridgeMapTestService(t *testing.T) *Service {
	t.Helper()
	return &Service{Configuration: Configuration{
		BridgeMapDatabasePath: filepath.Join(t.TempDir(), "bridge-map.sqlite"),
	}}
}

func TestBridgeMessageMappingRoundTripsBothDirections(t *testing.T) {
	service := newBridgeMapTestService(t)
	database, errorValue := service.openBridgeMapDatabase(context.Background())
	if errorValue != nil {
		t.Fatalf("open bridge map database: %v", errorValue)
	}
	defer database.Close()

	mapping := bridgeMessageMapping{
		BuzzEventID:       "event-1",
		Platform:          "mattermost",
		ExternalID:        "post-1",
		ExternalChannelID: "channel-1",
	}
	if errorValue := service.recordBridgeMessage(context.Background(), database, mapping); errorValue != nil {
		t.Fatalf("record: %v", errorValue)
	}

	byExternal, found, errorValue := service.bridgeMessageByExternal(context.Background(), database, "mattermost", "post-1")
	if errorValue != nil || !found {
		t.Fatalf("by external: found=%v error=%v", found, errorValue)
	}
	if byExternal.BuzzEventID != "event-1" {
		t.Fatalf("by external buzz id = %q", byExternal.BuzzEventID)
	}

	byEvent, found, errorValue := service.bridgeMessageByEvent(context.Background(), database, "event-1", "mattermost")
	if errorValue != nil || !found {
		t.Fatalf("by event: found=%v error=%v", found, errorValue)
	}
	if byEvent.ExternalID != "post-1" {
		t.Fatalf("by event external id = %q", byEvent.ExternalID)
	}
}

func TestBridgeMessageMappingMissReturnsNotFound(t *testing.T) {
	service := newBridgeMapTestService(t)
	database, errorValue := service.openBridgeMapDatabase(context.Background())
	if errorValue != nil {
		t.Fatalf("open: %v", errorValue)
	}
	defer database.Close()
	_, found, errorValue := service.bridgeMessageByExternal(context.Background(), database, "slack", "never")
	if errorValue != nil {
		t.Fatalf("lookup error: %v", errorValue)
	}
	if found {
		t.Fatal("expected miss for unknown external id")
	}
}

func TestBridgeMessageRecordIsIdempotent(t *testing.T) {
	service := newBridgeMapTestService(t)
	database, errorValue := service.openBridgeMapDatabase(context.Background())
	if errorValue != nil {
		t.Fatalf("open: %v", errorValue)
	}
	defer database.Close()
	mapping := bridgeMessageMapping{BuzzEventID: "e", Platform: "mattermost", ExternalID: "p", ExternalChannelID: "c"}
	if errorValue := service.recordBridgeMessage(context.Background(), database, mapping); errorValue != nil {
		t.Fatalf("first record: %v", errorValue)
	}
	if errorValue := service.recordBridgeMessage(context.Background(), database, mapping); errorValue != nil {
		t.Fatalf("second record must be a no-op, got: %v", errorValue)
	}
}

func TestBridgeChannelMappingRoundTrips(t *testing.T) {
	service := newBridgeMapTestService(t)
	database, errorValue := service.openBridgeMapDatabase(context.Background())
	if errorValue != nil {
		t.Fatalf("open: %v", errorValue)
	}
	defer database.Close()
	mapping := bridgeChannelMapping{BuzzChannelID: "buzz-ch", Platform: "mattermost", ExternalChannelID: "mm-ch"}
	if errorValue := service.recordBridgeChannel(context.Background(), database, mapping); errorValue != nil {
		t.Fatalf("record channel: %v", errorValue)
	}
	byExternal, found, errorValue := service.bridgeChannelByExternal(context.Background(), database, "mattermost", "mm-ch")
	if errorValue != nil || !found || byExternal.BuzzChannelID != "buzz-ch" {
		t.Fatalf("channel by external: found=%v error=%v mapping=%+v", found, errorValue, byExternal)
	}
	byBuzz, found, errorValue := service.bridgeChannelByBuzz(context.Background(), database, "buzz-ch", "mattermost")
	if errorValue != nil || !found || byBuzz.ExternalChannelID != "mm-ch" {
		t.Fatalf("channel by buzz: found=%v error=%v mapping=%+v", found, errorValue, byBuzz)
	}
}
