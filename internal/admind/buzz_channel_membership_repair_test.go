package admind

import "testing"

func TestACircleRoomNamesTheCircleThatDecidesWhoBelongs(t *testing.T) {
	for _, testCase := range []struct {
		roomName string
		want     string
	}{
		{roomName: "circle-c-level", want: "c-level"},
		{roomName: "circle-hr-compensation", want: "hr-compensation"},
		{roomName: "town-square", want: ""},
		{roomName: "", want: ""},
	} {
		if circleID := circleIDOfRoom(testCase.roomName); circleID != testCase.want {
			t.Fatalf("circle of %q = %q; want %q", testCase.roomName, circleID, testCase.want)
		}
	}
}

func TestAPrivateRoomMirrorCarriesTheRoomsOwnName(t *testing.T) {
	shape := describeMattermostChannel(mattermostChannelRecord{
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
