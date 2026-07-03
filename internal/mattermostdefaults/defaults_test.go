package mattermostdefaults

import "testing"

func TestPublicChannelsForLanguageLocalizesManagedChannelCopy(t *testing.T) {
	koreanChannels := PublicChannelsForLanguage("ko")
	englishChannels := PublicChannelsForLanguage("en")

	assertPublicChannel(t, koreanChannels[0], TownSquareChannelName, "광장", "", "")
	assertPublicChannel(t, koreanChannels[1], OffTopicChannelName, "잡담", "", "")
	assertPublicChannel(t, koreanChannels[2], FlowChannelName, "업무", "[업무 열기](/flow/)", "")
	assertPublicChannel(t, koreanChannels[3], CalendarChannelName, "일정", "[일정 열기](/calendar/)", "")
	assertPublicChannel(t, koreanChannels[4], AttendanceChannelName, "근태", "[근태 열기](/attendance/)", "")

	assertPublicChannel(t, englishChannels[0], TownSquareChannelName, "Town Square", "", "")
	assertPublicChannel(t, englishChannels[1], OffTopicChannelName, "Off-Topic", "", "")
	assertPublicChannel(t, englishChannels[2], FlowChannelName, "Flow", "[Open Flow](/flow/)", "")
	assertPublicChannel(t, englishChannels[3], CalendarChannelName, "Calendar", "[Open Calendar](/calendar/)", "")
	assertPublicChannel(t, englishChannels[4], AttendanceChannelName, "Attendance", "[Open Attendance](/attendance/)", "")
}

func TestCircleChannelDisplayNameRemovesCirclePrefix(t *testing.T) {
	tests := map[string]string{
		"circle-c-level":         "C-level",
		"circle-representative":  "Representative",
		"circle-admin":           "Admin",
		"circle-hr-compensation": "HR Compensation",
	}

	for channelName, expectedDisplayName := range tests {
		if displayName := CircleChannelDisplayName(channelName); displayName != expectedDisplayName {
			t.Fatalf("display name for %q = %q", channelName, displayName)
		}
	}
}

func assertPublicChannel(t *testing.T, channel PublicChannel, name string, displayName string, header string, purpose string) {
	t.Helper()
	if channel.Name != name || channel.DisplayName != displayName || channel.Header != header || channel.Purpose != purpose {
		t.Fatalf("channel = %+v", channel)
	}
}
