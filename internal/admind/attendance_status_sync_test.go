package admind

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func customStatusProps(t *testing.T, text string, doubleEncoded bool) map[string]json.RawMessage {
	t.Helper()
	inner, errorValue := json.Marshal(mattermostCustomStatus{Emoji: "house", Text: text})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !doubleEncoded {
		return map[string]json.RawMessage{"customStatus": inner}
	}
	outer, errorValue := json.Marshal(string(inner))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return map[string]json.RawMessage{"customStatus": outer}
}

func customStatusTexts(statuses []mattermostCustomStatus) []string {
	texts := make([]string, 0, len(statuses))
	for _, status := range statuses {
		texts = append(texts, status.Text)
	}
	return texts
}

func setTwoAttendanceLocations(t *testing.T, service *Service) {
	t.Helper()
	if errorValue := service.writeAttendanceLocationsFile([]attendanceLocation{
		{ID: "office", Name: "사무실", Color: "#16a34a", IsDefault: true},
		{ID: "home", Name: "재택", Color: "#2563eb"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestMattermostCustomStatusText(t *testing.T) {
	cases := []struct {
		name string
		user mattermostUserRecord
		want string
	}{
		{name: "double encoded string", user: mattermostUserRecord{Props: customStatusProps(t, "재택", true)}, want: "재택"},
		{name: "object", user: mattermostUserRecord{Props: customStatusProps(t, "사무실", false)}, want: "사무실"},
		{name: "missing prop", user: mattermostUserRecord{}, want: ""},
		{name: "empty text", user: mattermostUserRecord{Props: customStatusProps(t, "", true)}, want: ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := mattermostCustomStatusText(testCase.user); got != testCase.want {
				t.Fatalf("got %q want %q", got, testCase.want)
			}
		})
	}
}

func TestAttendanceStatusPresetsMirrorButtons(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	single := customStatusTexts(service.attendanceStatusPresets())
	if len(single) != 2 || single[0] != "출근" || single[1] != "퇴근" {
		t.Fatalf("single-location presets = %v, want [출근 퇴근]", single)
	}

	setTwoAttendanceLocations(t, service)
	multiple := customStatusTexts(service.attendanceStatusPresets())
	if len(multiple) != 3 || multiple[0] != "사무실" || multiple[1] != "재택" || multiple[2] != "퇴근" {
		t.Fatalf("multi-location presets = %v, want [사무실 재택 퇴근]", multiple)
	}
}

func TestAttendanceCustomStatusForKindMatchesButtons(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	clockIn, ok := service.attendanceCustomStatusForKind(attendanceKindClockIn, service.defaultAttendanceLocation())
	if !ok || clockIn.Text != "출근" {
		t.Fatalf("single-location clock in = %+v ok=%v, want text 출근", clockIn, ok)
	}
	clockOut, ok := service.attendanceCustomStatusForKind(attendanceKindClockOut, attendanceLocation{})
	if !ok || clockOut.Text != "퇴근" {
		t.Fatalf("clock out = %+v ok=%v, want text 퇴근", clockOut, ok)
	}

	setTwoAttendanceLocations(t, service)
	home, ok := service.attendanceCustomStatusForKind(attendanceKindClockIn, service.attendanceLocationByID("home"))
	if !ok || home.Text != "재택" {
		t.Fatalf("multi-location clock in home = %+v ok=%v, want text 재택", home, ok)
	}
}

func TestCustomStatusPresetsEqual(t *testing.T) {
	presets := []mattermostCustomStatus{{Text: "출근"}, {Text: "퇴근"}}
	if !customStatusPresetsEqual([]mattermostCustomStatus{{Text: "출근"}, {Text: "퇴근"}}, presets) {
		t.Fatal("identical preset lists should be equal")
	}
	if customStatusPresetsEqual([]mattermostCustomStatus{{Text: "출근"}, {Text: "재택"}, {Text: "퇴근"}}, presets) {
		t.Fatal("stale extra preset should make lists unequal")
	}
}

func TestApplyAttendanceStatusChangeClocksInAndDedupes(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	service.saveMattermostAttendanceChannelID("attendance-channel")
	service.saveMattermostAttendanceEntryPostID("entry-post")
	setTwoAttendanceLocations(t, service)

	lastStatusByUser := map[string]string{}
	user := mattermostUserRecord{ID: "user-1", Props: customStatusProps(t, "재택", true)}
	service.applyAttendanceStatusChange(context.Background(), user, lastStatusByUser)
	service.applyAttendanceStatusChange(context.Background(), user, lastStatusByUser)

	events, errorValue := service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 1 || events[0].Kind != attendanceKindClockIn || events[0].LocationID != "home" {
		t.Fatalf("events after unchanged status = %+v", events)
	}

	moveLatestAttendanceEventByDurationForTest(t, service, -time.Hour)
	clockedOut := mattermostUserRecord{ID: "user-1", Props: customStatusProps(t, "퇴근", true)}
	service.applyAttendanceStatusChange(context.Background(), clockedOut, lastStatusByUser)

	events, errorValue = service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 2 || events[0].Kind != attendanceKindClockOut {
		t.Fatalf("events after clock out status = %+v", events)
	}
}

func TestSeededAttendanceStatusDoesNotReplayAsClockIn(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	user := mattermostUserRecord{
		ID:    "user-1",
		Props: map[string]json.RawMessage{"customStatus": json.RawMessage(`{"emoji":"office","text":"출근"}`)},
	}
	lastStatusByUser := map[string]string{}

	lastStatusByUser[user.ID] = mattermostCustomStatusText(user)
	service.applyAttendanceStatusChange(context.Background(), user, lastStatusByUser)

	month := time.Now().In(func() *time.Location { location, _ := service.workspaceTimeLocation(); return location }()).Format("2006-01")
	events, errorValue := service.readAttendanceEvents(context.Background(), month, "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 0 {
		t.Fatalf("expected a seeded status to record nothing, events = %+v", events)
	}
}
