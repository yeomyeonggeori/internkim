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

func CircleChannelDisplayName(channelName string) string {
	value := strings.TrimPrefix(strings.TrimSpace(channelName), "circle-")
	switch value {
	case "":
		return strings.TrimSpace(channelName)
	case "c-level":
		return "C-level"
	case "hr":
		return "HR"
	default:
		return titleWords(strings.ReplaceAll(value, "-", " "))
	}
}

type publicChannelLabels struct {
	TownSquareDisplayName string
	OffTopicDisplayName   string
}

func publicChannelLabelsForLanguage(language string) publicChannelLabels {
	if strings.EqualFold(strings.TrimSpace(language), "en") {
		return publicChannelLabels{
			TownSquareDisplayName: "Town Square",
			OffTopicDisplayName:   "Off-Topic",
		}
	}
	return publicChannelLabels{
		TownSquareDisplayName: TownSquareChannelDisplayName,
		OffTopicDisplayName:   OffTopicChannelDisplayName,
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
