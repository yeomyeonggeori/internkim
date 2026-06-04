package admind

import (
	"strings"

	"gitlab.com/eastriver/internkim/internal/mattermostdefaults"
)

func (service *Service) mattermostAttendanceEntryPostProps() map[string]any {
	text := service.adminText()
	return map[string]any{
		attendanceEntryPostProperty: true,
		"attachments": []mattermostAttachment{
			{
				Fallback: text.AttendanceEntryMessage,
				Text:     text.AttendanceEntryText,
				Actions:  service.mattermostAttendanceEntryActions(),
			},
		},
	}
}

func (service *Service) mattermostAttendanceEntryActions() []mattermostAction {
	locations, errorValue := service.readAttendanceLocations()
	if errorValue != nil {
		locations = defaultAttendanceLocations()
	}
	text := service.adminText()
	actions := make([]mattermostAction, 0, len(locations)+1)
	if len(locations) == 1 {
		actions = append(actions, service.mattermostAttendanceClockInButton(attendanceClockInAction, text.AttendanceClockIn, locations[0]))
	} else {
		for _, location := range locations {
			actions = append(actions, service.mattermostAttendanceClockInButton(attendanceClockInActionID(location), location.Name, location))
		}
	}
	actions = append(actions, service.mattermostInteractiveButton(attendanceClockOutAction, text.AttendanceClockOut, text.AttendanceClockOutTooltip, "danger"))
	return actions
}

func attendanceClockInActionID(location attendanceLocation) string {
	suffix := sanitizeMattermostActionIDPart(location.ID)
	if suffix == "" {
		return attendanceClockInAction
	}
	return attendanceClockInAction + suffix
}

func sanitizeMattermostActionIDPart(value string) string {
	var builder strings.Builder
	for _, character := range strings.TrimSpace(value) {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' {
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

func (service *Service) mattermostAttendanceClockInButton(actionID string, name string, location attendanceLocation) mattermostAction {
	return service.mattermostInteractiveButtonWithContext(
		actionID,
		name,
		service.adminText().AttendanceClockInTooltip,
		"success",
		mattermostInteractiveContext{Action: attendanceClockInAction, LocationID: location.ID},
	)
}

func (service *Service) mattermostAttendanceLink() string {
	label := mattermostdefaults.PublicChannelLinkLabel(attendanceChannelName, service.workspaceLanguage())
	baseURL := strings.TrimRight(strings.TrimSpace(service.mattermostFlowBaseURL()), "/")
	if baseURL == "" {
		return "[" + label + "](/attendance/)"
	}
	return "[" + label + "](" + baseURL + "/attendance/)"
}
