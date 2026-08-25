package admind

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

const attendanceNotifyTimeout = 20 * time.Second

// Clocking in is a fact the company reads together, so everyone is told and
// each person decides for themselves whether to hear it. The one who clocked
// is left out: they were there when it happened.
func (service *Service) alsoTellTheCompanyAboutAttendance(event attendanceEvent) {
	if !service.Configuration.AttendanceNotifyEnabled {
		return
	}
	title, told := attendanceNotifyTitle(event)
	if !told {
		return
	}
	client := service.centralPlane()
	if client == nil {
		return
	}
	go service.tell(client, centralplane.Notification{
		Platform:    "mattermost",
		ExternalIDs: service.everyoneExcept(context.Background(), event.MattermostUserID),
		Category:    "attendance",
		Title:       title,
		Body:        attendanceNotifyBody(event),
		OpenPath:    "/attendance/",
		Tag:         "attendance-" + event.ID,
	})
}

// A request waits on whoever gets to it first, so every administrator is told
// rather than one picked by the runtime.
func (service *Service) alsoTellTheAdministratorsAboutLeave(record attendanceLeaveRequestRecord, employeeName string) {
	if !service.Configuration.AttendanceNotifyEnabled {
		return
	}
	client := service.centralPlane()
	if client == nil {
		return
	}
	go service.tell(client, centralplane.Notification{
		Platform:    "mattermost",
		ExternalIDs: service.administrators(context.Background()),
		Category:    "leave",
		Title:       "휴가 신청: " + firstNonEmpty(employeeName, record.EmployeeEmail),
		Body:        leaveNotifyBody(record),
		OpenPath:    "/attendance/",
		Tag:         "leave-" + record.ID,
	})
}

func (service *Service) tell(client *centralplane.Client, notification centralplane.Notification) {
	if len(notification.ExternalIDs) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), attendanceNotifyTimeout)
	defer cancel()
	result, errorValue := client.Notify(ctx, notification)
	if errorValue != nil {
		log.Printf("attendance notify: %s was not sent: %v", notification.Tag, errorValue)
		return
	}
	log.Printf("attendance notify: %s addressed %d, told %d, reached %d",
		notification.Tag, len(notification.ExternalIDs), result.Told, result.Reached)
}

func (service *Service) everyoneExcept(ctx context.Context, externalID string) []string {
	return attendanceNotifyRecipients(service.attendanceNotifyDirectory(ctx), func(record adminUserMutation) bool {
		return record.MattermostUserID != externalID
	})
}

func (service *Service) administrators(ctx context.Context) []string {
	return attendanceNotifyRecipients(service.attendanceNotifyDirectory(ctx), func(record adminUserMutation) bool {
		return record.Role == adminUserRoleAdmin || record.Role == adminUserRoleOperationsAdmin
	})
}

func attendanceNotifyRecipients(records []adminUserMutation, wanted func(adminUserMutation) bool) []string {
	externalIDs := make([]string, 0, len(records))
	for _, record := range records {
		if record.MattermostUserID == "" || !wanted(record) {
			continue
		}
		externalIDs = append(externalIDs, record.MattermostUserID)
	}
	return externalIDs
}

func (service *Service) attendanceNotifyDirectory(ctx context.Context) []adminUserMutation {
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost", nil)
	if errorValue != nil {
		return nil
	}
	return service.accountDirectoryUserRecords(request)
}

func attendanceNotifyTitle(event attendanceEvent) (string, bool) {
	name := firstNonEmpty(event.DisplayName, event.MattermostUsername, event.Email)
	switch event.Kind {
	case attendanceKindClockIn:
		return name + " 출근", true
	case attendanceKindClockOut:
		return name + " 퇴근", true
	default:
		return "", false
	}
}

// Where somebody clocked in is the part a reader cannot guess, so it leads.
func attendanceNotifyBody(event attendanceEvent) string {
	at := strings.TrimSpace(event.LocalTime)
	where := strings.TrimSpace(event.LocationName)
	if where != "" && at != "" {
		return where + " · " + at
	}
	return firstNonEmpty(where, at)
}

func leaveNotifyBody(record attendanceLeaveRequestRecord) string {
	when := strings.TrimSpace(record.StartDate)
	if end := strings.TrimSpace(record.EndDate); end != "" && end != when {
		when = when + " ~ " + end
	}
	return firstNonEmpty(strings.TrimSpace(record.LeaveTypeName)+" "+when, when)
}

// The address book knows a member by name; the leave record only knows the
// address they applied with.
func (service *Service) attendanceLeaveEmployeeName(ctx context.Context, email string) string {
	wanted := strings.ToLower(strings.TrimSpace(email))
	if wanted == "" {
		return ""
	}
	for _, record := range service.attendanceNotifyDirectory(ctx) {
		if strings.ToLower(strings.TrimSpace(record.Email)) == wanted {
			return firstNonEmpty(record.Name, record.MattermostUsername)
		}
	}
	return ""
}
