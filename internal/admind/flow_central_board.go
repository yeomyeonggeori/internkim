package admind

import (
	"context"
	"log"
	"sort"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

type flowActorContextKey struct{}

// The board is read as whoever asked for it, so row level security decides what
// comes back. The reads are reached through several layers that carry only a
// context, and the actor is a property of the request rather than of any of
// them.
func withFlowActor(ctx context.Context, actorEmail string) context.Context {
	actorEmail = strings.ToLower(strings.TrimSpace(actorEmail))
	if actorEmail == "" {
		return ctx
	}
	return context.WithValue(ctx, flowActorContextKey{}, actorEmail)
}

func flowActorOf(ctx context.Context) string {
	actorEmail, _ := ctx.Value(flowActorContextKey{}).(string)
	return actorEmail
}

func (service *Service) companyBoardTasks(ctx context.Context, members []flowMember) ([]flowTask, bool) {
	actorEmail := flowActorOf(ctx)
	if actorEmail == "" {
		return nil, false
	}
	client := service.centralPlane()
	if client == nil {
		return nil, false
	}
	tasks, errorValue := client.BoardTasks(ctx, "email", actorEmail)
	if errorValue != nil {
		log.Printf("the board stays on this device for %s: %v", actorEmail, errorValue)
		return nil, false
	}
	return flowTasksOfBoardTasks(tasks, members), true
}

func flowTasksOfBoardTasks(tasks []centralplane.BoardTask, members []flowMember) []flowTask {
	memberByEmail := map[string]flowMember{}
	for _, member := range members {
		if email := strings.ToLower(strings.TrimSpace(member.Email)); email != "" {
			memberByEmail[email] = member
		}
	}
	converted := make([]flowTask, 0, len(tasks))
	for _, task := range tasks {
		converted = append(converted, flowTaskOfBoardTask(task, memberByEmail))
	}
	sort.SliceStable(converted, func(first int, second int) bool {
		return converted[first].CreatedAt > converted[second].CreatedAt
	})
	return converted
}

func flowTaskOfBoardTask(task centralplane.BoardTask, memberByEmail map[string]flowMember) flowTask {
	converted := flowTask{
		ID:           task.CentralID,
		Business:     task.Business,
		Type:         task.Type,
		Content:      task.Title,
		Size:         task.Size,
		Status:       task.Status,
		StartDate:    flowDayOfInstant(task.StartsAt),
		EndDate:      flowDayOfInstant(firstFilled(task.EndsAt, task.DueAt)),
		CreatedAt:    task.CreatedAt,
	}
	for _, email := range task.ParticipantMails {
		member, found := memberByEmail[strings.ToLower(strings.TrimSpace(email))]
		if !found {
			continue
		}
		converted.ParticipantIDs = append(converted.ParticipantIDs, member.ID)
		converted.ParticipantNames = append(converted.ParticipantNames, member.Name)
	}
	if len(converted.ParticipantIDs) > 0 {
		converted.OwnerID = converted.ParticipantIDs[0]
		converted.OwnerName = converted.ParticipantNames[0]
	}
	converted.WeekCode = flowWeekCodeOfDay(firstFilled(converted.StartDate, converted.EndDate))
	return converted
}

func firstFilled(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func flowDayOfInstant(instant string) string {
	instant = strings.TrimSpace(instant)
	if instant == "" {
		return ""
	}
	moment, errorValue := time.Parse(time.RFC3339, instant)
	if errorValue != nil {
		if len(instant) >= 10 {
			return instant[:10]
		}
		return ""
	}
	return moment.In(flowDateLocation()).Format("2006-01-02")
}

func flowWeekCodeOfDay(day string) string {
	if strings.TrimSpace(day) == "" {
		return ""
	}
	moment, errorValue := time.Parse("2006-01-02", day)
	if errorValue != nil {
		return ""
	}
	return weekCodeForDate(moment)
}

func flowTasksInWeek(tasks []flowTask, weekCode string) []flowTask {
	kept := []flowTask{}
	for _, task := range tasks {
		if task.WeekCode == weekCode {
			kept = append(kept, task)
		}
	}
	return kept
}

func flowTasksBetweenDays(tasks []flowTask, startDate string, endDate string) []flowTask {
	kept := []flowTask{}
	for _, task := range tasks {
		if dayFallsWithin(task.StartDate, startDate, endDate) || dayFallsWithin(task.EndDate, startDate, endDate) {
			kept = append(kept, task)
		}
	}
	return kept
}

func dayFallsWithin(day string, startDate string, endDate string) bool {
	if strings.TrimSpace(day) == "" {
		return false
	}
	return day >= startDate && day <= endDate
}
