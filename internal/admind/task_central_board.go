package admind

import (
	"context"
	"log"
	"sort"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

type taskActorContextKey struct{}

// The board is read as whoever asked for it, so row level security decides what
// comes back. The reads are reached through several layers that carry only a
// context, and the actor is a property of the request rather than of any of
// them.
func withTaskActor(ctx context.Context, actorEmail string) context.Context {
	actorEmail = strings.ToLower(strings.TrimSpace(actorEmail))
	if actorEmail == "" {
		return ctx
	}
	return context.WithValue(ctx, taskActorContextKey{}, actorEmail)
}

func taskActorOf(ctx context.Context) string {
	actorEmail, _ := ctx.Value(taskActorContextKey{}).(string)
	return actorEmail
}

func (service *Service) companyBoardTasks(ctx context.Context, members []taskMember) ([]Task, bool) {
	actorEmail := taskActorOf(ctx)
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
	return tasksOfBoardTasks(tasks, members), true
}

func tasksOfBoardTasks(tasks []centralplane.BoardTask, members []taskMember) []Task {
	memberByEmail := map[string]taskMember{}
	for _, member := range members {
		if email := strings.ToLower(strings.TrimSpace(member.Email)); email != "" {
			memberByEmail[email] = member
		}
	}
	converted := make([]Task, 0, len(tasks))
	for _, task := range tasks {
		converted = append(converted, taskOfBoardTask(task, memberByEmail))
	}
	sort.SliceStable(converted, func(first int, second int) bool {
		return converted[first].CreatedAt > converted[second].CreatedAt
	})
	return converted
}

func taskOfBoardTask(task centralplane.BoardTask, memberByEmail map[string]taskMember) Task {
	converted := Task{
		ID:        task.CentralID,
		Business:  task.Business,
		Type:      task.Type,
		Content:   task.Title,
		Size:      task.Size,
		Status:    deviceTaskStatus(task.Status),
		StartDate: taskDayOfInstant(task.StartsAt),
		EndDate:   taskDayOfInstant(firstFilled(task.EndsAt, task.DueAt)),
		CreatedAt: task.CreatedAt,
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
	converted.WeekCode = taskWeekCodeOfDay(firstFilled(converted.StartDate, converted.EndDate))
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

func taskDayOfInstant(instant string) string {
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
	return moment.In(taskDateLocation()).Format("2006-01-02")
}

func taskWeekCodeOfDay(day string) string {
	if strings.TrimSpace(day) == "" {
		return ""
	}
	moment, errorValue := time.Parse("2006-01-02", day)
	if errorValue != nil {
		return ""
	}
	return weekCodeForDate(moment)
}

func tasksInWeek(tasks []Task, weekCode string) []Task {
	kept := []Task{}
	for _, task := range tasks {
		if task.WeekCode == weekCode {
			kept = append(kept, task)
		}
	}
	return kept
}

func tasksBetweenDays(tasks []Task, startDate string, endDate string) []Task {
	kept := []Task{}
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

// A write is answered with the identifier every later read will show, which is
// the company's once it holds the task.
func (service *Service) taskAnsweredWithCompanyIdentity(ctx context.Context, task Task) Task {
	if service.centralPlane() == nil || strings.TrimSpace(task.ID) == "" {
		return task
	}
	database, errorValue := service.openTaskDatabase(ctx)
	if errorValue != nil {
		return task
	}
	defer database.Close()
	centralTaskID, errorValue := readTaskCentralIdentity(ctx, database, task.ID)
	if errorValue != nil || strings.TrimSpace(centralTaskID) == "" {
		return task
	}
	task.ID = centralTaskID
	return task
}
