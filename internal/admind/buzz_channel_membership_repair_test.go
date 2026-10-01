package admind

import (
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/buzzimport/mattermostadmin"
)

func TestACircleRoomNamesTheCircleThatDecidesWhoBelongs(t *testing.T) {
	for _, testCase := range []struct {
		roomName string
		want     string
	}{
		{roomName: "circle-c-level", want: "c-level"},
		{roomName: "circle-hr", want: "hr"},
		{roomName: "town-square", want: ""},
		{roomName: "", want: ""},
	} {
		if circleID := circleIDOfRoom(testCase.roomName); circleID != testCase.want {
			t.Fatalf("circle of %q = %q; want %q", testCase.roomName, circleID, testCase.want)
		}
	}
}

func TestAPrivateRoomMirrorCarriesTheRoomsOwnName(t *testing.T) {
	shape := describeMattermostChannel(mattermostadmin.ChannelRecord{
		Name:        "circle-c-level",
		DisplayName: "C-Level",
		Type:        "P",
	}, "channel-1")

	if shape.RoomName != "circle-c-level" {
		t.Fatalf("room name = %q; without it a circle channel cannot be told from any other private room", shape.RoomName)
	}
	if shape.Visibility != "private" {
		t.Fatalf("visibility = %q; a private room must not become a channel anyone can walk into", shape.Visibility)
	}
}

func TestAPrivateRoomThatIsNoCirclesIsLeftAlone(t *testing.T) {
	shape := describeMattermostChannel(mattermostadmin.ChannelRecord{
		Name:        "smart-shop-onboarding",
		DisplayName: "스마트상점 온보딩",
		Type:        "P",
	}, "channel-2")

	if circleIDOfRoom(shape.RoomName) != "" {
		t.Fatal("a private room that is no circle's has no circle to judge its members by")
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
