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
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), attendanceNotifyTimeout)
		defer cancel()
		service.tell(ctx, client, centralplane.Notification{
			Emails:   service.everyoneExcept(ctx, event.Email),
			Category: "attendance",
			Title:    title,
			Body:     attendanceNotifyBody(event),
			OpenPath: "/attendance/",
			Tag:      "attendance-" + event.ID,
		})
	}()
}

func (service *Service) alsoTellTheAdministratorsAboutLeave(record attendanceLeaveRequestRecord) {
	if !service.Configuration.AttendanceNotifyEnabled {
		return
	}
	client := service.centralPlane()
	if client == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), attendanceNotifyTimeout)
		defer cancel()
		service.tell(ctx, client, centralplane.Notification{
			Emails:   service.administrators(ctx),
			Category: "leave",
			Title:    "휴가 신청: " + firstNonEmpty(service.attendanceLeaveEmployeeName(ctx, record.EmployeeEmail), record.EmployeeEmail),
			Body:     leaveNotifyBody(record),
			OpenPath: "/attendance/",
			Tag:      "leave-" + record.ID,
		})
	}()
}

func (service *Service) tell(ctx context.Context, client *centralplane.Client, notification centralplane.Notification) {
	if len(notification.Emails) == 0 && len(notification.ExternalIDs) == 0 {
		return
	}
	result, errorValue := client.Notify(ctx, notification)
	if errorValue != nil {
		log.Printf("attendance notify: %s was not sent: %v", notification.Tag, errorValue)
		return
	}
	log.Printf("attendance notify: %s addressed %d, told %d, reached %d",
		notification.Tag, result.Addressed, result.Told, result.Reached)
}

func (service *Service) everyoneExcept(ctx context.Context, email string) []string {
	excluded := strings.ToLower(strings.TrimSpace(email))
	return attendanceNotifyRecipients(service.attendanceNotifyDirectory(ctx), func(record adminUserMutation) bool {
		return strings.ToLower(strings.TrimSpace(record.Email)) != excluded
	})
}

func (service *Service) administrators(ctx context.Context) []string {
	return attendanceNotifyRecipients(service.attendanceNotifyDirectory(ctx), func(record adminUserMutation) bool {
		return record.Role == adminUserRoleAdmin || record.Role == adminUserRoleOperationsAdmin
	})
}

func attendanceNotifyRecipients(records []adminUserMutation, wanted func(adminUserMutation) bool) []string {
	emails := make([]string, 0, len(records))
	for _, record := range records {
		email := strings.ToLower(strings.TrimSpace(record.Email))
		if email == "" || !wanted(record) {
			continue
		}
		emails = append(emails, email)
	}
	return emails
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
