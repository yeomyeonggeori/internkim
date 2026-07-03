package mattermostdefaults

import "strings"

const (
	AdminResourcePath      = "admin"
	AttendanceResourcePath = "attendance"
	CalendarResourcePath   = "calendar"
	FlowResourcePath       = "flow"
	MailResourcePath       = "mail"
	MemoryResourcePath     = "memory"

	TownSquareChannelName        = "town-square"
	TownSquareChannelDisplayName = "광장"

	OffTopicChannelName        = "off-topic"
	OffTopicChannelDisplayName = "잡담"

	FlowChannelName        = "flow"
	FlowChannelDisplayName = "업무"

	CalendarChannelName        = "calendar"
	CalendarChannelDisplayName = "일정"

	AttendanceChannelName        = "attendance"
	AttendanceChannelDisplayName = "근태"
)

type PublicChannel struct {
	Name        string
	DisplayName string
	Header      string
	Purpose     string
}

func ManagedResourcePaths() []string {
	return []string{
		AdminResourcePath,
		AttendanceResourcePath,
		CalendarResourcePath,
		FlowResourcePath,
		MailResourcePath,
		MemoryResourcePath,
	}
}

func ManagedResourcePathSetting() string {
	return strings.Join(ManagedResourcePaths(), ",")
}

func DefaultPublicChannels() []PublicChannel {
	return PublicChannelsForLanguage("ko")
}

func PublicChannelsForLanguage(language string) []PublicChannel {
	labels := publicChannelLabelsForLanguage(language)
	return []PublicChannel{
		{Name: TownSquareChannelName, DisplayName: labels.TownSquareDisplayName},
		{Name: OffTopicChannelName, DisplayName: labels.OffTopicDisplayName},
		{Name: FlowChannelName, DisplayName: labels.FlowDisplayName, Header: PublicChannelLink(FlowChannelName, language, "/flow/")},
		{Name: CalendarChannelName, DisplayName: labels.CalendarDisplayName, Header: PublicChannelLink(CalendarChannelName, language, "/calendar/")},
		{Name: AttendanceChannelName, DisplayName: labels.AttendanceDisplayName, Header: PublicChannelLink(AttendanceChannelName, language, "/attendance/")},
	}
}

func PublicChannelForLanguage(channelName string, language string) (PublicChannel, bool) {
	normalizedChannelName := strings.TrimSpace(channelName)
	for _, channel := range PublicChannelsForLanguage(language) {
		if channel.Name == normalizedChannelName {
			return channel, true
		}
	}
	return PublicChannel{}, false
}

func PublicChannelLink(channelName string, language string, targetURL string) string {
	return "[" + PublicChannelLinkLabel(channelName, language) + "](" + strings.TrimSpace(targetURL) + ")"
}

func PublicChannelLinkLabel(channelName string, language string) string {
	labels := publicChannelLabelsForLanguage(language)
	switch strings.TrimSpace(channelName) {
	case FlowChannelName:
		return labels.FlowLinkLabel
	case CalendarChannelName:
		return labels.CalendarLinkLabel
	case AttendanceChannelName:
		return labels.AttendanceLinkLabel
	default:
		return strings.TrimSpace(channelName)
	}
}

func CircleChannelDisplayName(channelName string) string {
	value := strings.TrimPrefix(strings.TrimSpace(channelName), "circle-")
	switch value {
	case "":
		return strings.TrimSpace(channelName)
	case "c-level":
		return "C-level"
	case "hr-compensation":
		return "HR Compensation"
	default:
		return titleWords(strings.ReplaceAll(value, "-", " "))
	}
}

type publicChannelLabels struct {
	TownSquareDisplayName string
	OffTopicDisplayName   string
	FlowDisplayName       string
	FlowLinkLabel         string
	CalendarDisplayName   string
	CalendarLinkLabel     string
	AttendanceDisplayName string
	AttendanceLinkLabel   string
}

func publicChannelLabelsForLanguage(language string) publicChannelLabels {
	if strings.EqualFold(strings.TrimSpace(language), "en") {
		return publicChannelLabels{
			TownSquareDisplayName: "Town Square",
			OffTopicDisplayName:   "Off-Topic",
			FlowDisplayName:       "Flow",
			FlowLinkLabel:         "Open Flow",
			CalendarDisplayName:   "Calendar",
			CalendarLinkLabel:     "Open Calendar",
			AttendanceDisplayName: "Attendance",
			AttendanceLinkLabel:   "Open Attendance",
		}
	}
	return publicChannelLabels{
		TownSquareDisplayName: TownSquareChannelDisplayName,
		OffTopicDisplayName:   OffTopicChannelDisplayName,
		FlowDisplayName:       FlowChannelDisplayName,
		FlowLinkLabel:         "업무 열기",
		CalendarDisplayName:   CalendarChannelDisplayName,
		CalendarLinkLabel:     "일정 열기",
		AttendanceDisplayName: AttendanceChannelDisplayName,
		AttendanceLinkLabel:   "근태 열기",
	}
}

func titleWords(value string) string {
	words := strings.Fields(value)
	for index, word := range words {
		if len(word) <= 2 {
			words[index] = strings.ToUpper(word)
			continue
		}
		words[index] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
	}
	return strings.Join(words, " ")
}
