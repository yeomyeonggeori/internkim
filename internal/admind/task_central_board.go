package admind

import (
	"context"
	"errors"
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

// The company board is the record, so a read that fails is an error rather
// than a quiet fall back to the device's stale copy.
func (service *Service) companyBoardTasks(ctx context.Context, members []taskMember) ([]Task, bool, error) {
	actorEmail := taskActorOf(ctx)
	if actorEmail == "" {
		return nil, false, nil
	}
	client := service.centralPlane()
	if client == nil {
		return nil, false, nil
	}
	tasks, errorValue := client.BoardTasks(ctx, "email", actorEmail)
	if errorValue != nil {
		log.Printf("the company board did not answer for %s: %v", actorEmail, errorValue)
		return nil, true, errorValue
	}
	return tasksOfBoardTasks(tasks, members, service.companyTimeLocation(ctx)), true, nil
}

func tasksOfBoardTasks(tasks []centralplane.BoardTask, members []taskMember, location *time.Location) []Task {
	memberByEmail := map[string]taskMember{}
	for _, member := range members {
		if email := strings.ToLower(strings.TrimSpace(member.Email)); email != "" {
			memberByEmail[email] = member
		}
	}
	converted := make([]Task, 0, len(tasks))
	for _, task := range tasks {
		converted = append(converted, taskOfBoardTask(task, memberByEmail, location))
	}
	sort.SliceStable(converted, func(first int, second int) bool {
		return converted[first].CreatedAt > converted[second].CreatedAt
	})
	return converted
}

func taskOfBoardTask(task centralplane.BoardTask, memberByEmail map[string]taskMember, location *time.Location) Task {
	converted := Task{
		ID:        task.CentralID,
		Business:  task.Business,
		Type:      task.Type,
		Content:   task.Title,
		Size:      task.Size,
		Status:    cleanTaskStatus(task.Status),
		StartDate: taskDayOfInstant(task.StartsAt, location),
		EndDate:   taskDayOfInstant(firstFilled(task.EndsAt, task.DueAt), location),
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

func taskDayOfInstant(instant string, location *time.Location) string {
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
	return moment.In(location).Format("2006-01-02")
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

var errTaskBoardUnnamed = errors.New("this device names no company, and the company keeps the board")

func (service *Service) readTasks(ctx context.Context, weekCode string, members []taskMember) ([]Task, error) {
	tasks, answered, errorValue := service.companyBoardTasks(ctx, members)
	if !answered {
		return nil, errTaskBoardUnnamed
	}
	if errorValue != nil {
		return nil, errorValue
	}
	return tasksInWeek(tasks, weekCode), nil
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


