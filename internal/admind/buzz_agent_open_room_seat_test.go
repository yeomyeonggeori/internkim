package admind

import (
	"context"
	"database/sql"
	"reflect"
	"sort"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/buzzidentity"
)

const (
	roomCommunity            = "00000000-0000-0000-0000-0000000000c1"
	personOpenRoom           = "00000000-0000-0000-0000-000000000001"
	personPrivateRoom        = "00000000-0000-0000-0000-000000000002"
	companyRoomHoldingAgent  = "00000000-0000-0000-0000-000000000003"
	roomTheAgentWasRemoved   = "00000000-0000-0000-0000-000000000004"
	deletedOpenRoom          = "00000000-0000-0000-0000-000000000005"
	archivedOpenRoom         = "00000000-0000-0000-0000-000000000006"
	directMessageRoom        = "00000000-0000-0000-0000-000000000007"
	personWhoOpenedTheRooms  = "a1a1000000000000000000000000000000000000000000000000000000000001"
	roomShapeCreateStatement = `
CREATE TABLE channels (
    id uuid NOT NULL, community_id uuid NOT NULL, name text NOT NULL,
    channel_type text NOT NULL, visibility text NOT NULL, created_by bytea NOT NULL,
    archived_at timestamptz, deleted_at timestamptz);
CREATE TABLE channel_members (
    channel_id uuid NOT NULL, community_id uuid NOT NULL, pubkey bytea NOT NULL,
    role text NOT NULL, removed_at timestamptz)`
)

func TestTheAgentIsSeatedOnlyInOpenRoomsThatDoNotHoldIt(t *testing.T) {
	relay := disposableRelayDatabase(t)
	if _, errorValue := relay.Exec(roomShapeCreateStatement); errorValue != nil {
		t.Fatalf("create the room tables: %v", errorValue)
	}
	agentKey := derivedKey(t, buzzidentity.AgentSubject)
	holdRoom(t, relay, personOpenRoom, "stream", "open", "", "")
	holdRoom(t, relay, personPrivateRoom, "stream", "private", "", "")
	holdRoom(t, relay, companyRoomHoldingAgent, "stream", "open", "", "")
	holdRoom(t, relay, roomTheAgentWasRemoved, "stream", "open", "", "")
	holdRoom(t, relay, deletedOpenRoom, "stream", "open", "", "now()")
	holdRoom(t, relay, archivedOpenRoom, "stream", "open", "now()", "")
	holdRoom(t, relay, directMessageRoom, "dm", "private", "", "")
	seatInRoom(t, relay, companyRoomHoldingAgent, agentKey, false)
	seatInRoom(t, relay, roomTheAgentWasRemoved, agentKey, true)

	missing, errorValue := openRoomsMissingTheAgent(context.Background(), relay, agentKey)
	if errorValue != nil {
		t.Fatalf("read the open rooms without the agent: %v", errorValue)
	}

	sort.Strings(missing)
	expected := []string{personOpenRoom, roomTheAgentWasRemoved}
	if !reflect.DeepEqual(missing, expected) {
		t.Fatalf("the agent belongs in every live open room it is not in, and in no private one: got %v, want %v", missing, expected)
	}
}

func holdRoom(t *testing.T, relay *sql.DB, channelID, channelType, visibility, archivedAt, deletedAt string) {
	t.Helper()
	_, errorValue := relay.Exec(`
INSERT INTO channels (id, community_id, name, channel_type, visibility, created_by, archived_at, deleted_at)
VALUES ($1, $2, 'room', $3, $4, decode($5, 'hex'), nullif($6, '')::timestamptz, nullif($7, '')::timestamptz)`,
		channelID, roomCommunity, channelType, visibility, personWhoOpenedTheRooms, archivedAt, deletedAt)
	if errorValue != nil {
		t.Fatalf("hold room %s: %v", channelID, errorValue)
	}
}

func seatInRoom(t *testing.T, relay *sql.DB, channelID, pubkey string, isRemoved bool) {
	t.Helper()
	_, errorValue := relay.Exec(`
INSERT INTO channel_members (channel_id, community_id, pubkey, role, removed_at)
VALUES ($1, $2, decode($3, 'hex'), 'member', CASE WHEN $4 THEN now() END)`,
		channelID, roomCommunity, pubkey, isRemoved)
	if errorValue != nil {
		t.Fatalf("seat %s in %s: %v", pubkey, channelID, errorValue)
	}
}
