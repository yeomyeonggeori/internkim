package admind

import "testing"

func testGhostKeys() buzzServiceKeySet {
	return buzzServiceKeySet{
		bookkeeping: map[string]bool{"bootstrap-key": true, "old-bot-key": true},
		agent:       map[string]bool{"agent-key": true},
		derivable:   map[string]bool{"bootstrap-key": true, "old-bot-key": true, "agent-key": true, "person-1": true, "person-2": true},
	}
}

func TestClassifyDirectRoomFindsTheGhosts(t *testing.T) {
	cases := []struct {
		name       string
		room       buzzDirectRoom
		isGhost    bool
		isHeldBack bool
	}{
		{"bookkeeping key in a person's room", buzzDirectRoom{members: []string{"person-1", "old-bot-key", "bootstrap-key"}, messages: 40}, true, false},
		{"only the agent remains", buzzDirectRoom{members: []string{"agent-key"}, messages: 12}, true, false},
		{"one member and nothing said", buzzDirectRoom{members: []string{"person-1"}}, true, false},
		{"empty room", buzzDirectRoom{}, true, false},
		{"one member but messages exist", buzzDirectRoom{members: []string{"person-1"}, messages: 3}, false, true},
		{"counterpart key belongs to nobody", buzzDirectRoom{members: []string{"person-1"}, metadataParticipants: []string{"person-1", "dead-key"}, messages: 6}, true, false},
		{"counterpart is a real person who left", buzzDirectRoom{members: []string{"person-1"}, metadataParticipants: []string{"person-1", "person-2"}, messages: 6}, false, true},
		{"a person's real agent conversation", buzzDirectRoom{members: []string{"person-1", "agent-key"}, messages: 88}, false, false},
		{"two people talking", buzzDirectRoom{members: []string{"person-1", "person-2"}, messages: 5}, false, false},
	}
	for _, testCase := range cases {
		_, isGhost, isHeldBack := classifyDirectRoom(testCase.room, testGhostKeys())
		if isGhost != testCase.isGhost || isHeldBack != testCase.isHeldBack {
			t.Fatalf("%s: got ghost=%v heldBack=%v", testCase.name, isGhost, isHeldBack)
		}
	}
}

func TestParsePostgresTextArray(t *testing.T) {
	if held := parsePostgresTextArray(`{abc,"def"}`); len(held) != 2 || held[0] != "abc" || held[1] != "def" {
		t.Fatalf("unexpected parse: %v", held)
	}
	if held := parsePostgresTextArray(`{}`); len(held) != 0 {
		t.Fatalf("empty array must parse empty: %v", held)
	}
}
