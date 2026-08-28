package admind

import (
	"database/sql"
	"encoding/json"
)

func scanTask(rows *sql.Rows) (Task, error) {
	var task Task
	var participantIDsDocument string
	var participantNamesDocument string
	errorValue := rows.Scan(
		&task.ID,
		&task.WeekCode,
		&task.OwnerID,
		&task.OwnerName,
		&participantIDsDocument,
		&participantNamesDocument,
		&task.Business,
		&task.Type,
		&task.Content,
		&task.Size,
		&task.Status,
		&task.StatusRank,
		&task.StartDate,
		&task.EndDate,
		&task.MattermostPostID,
		&task.CalendarEventID,
		&task.CreatedAt,
	)
	if errorValue != nil {
		return Task{}, errorValue
	}
	_ = json.Unmarshal([]byte(participantIDsDocument), &task.ParticipantIDs)
	_ = json.Unmarshal([]byte(participantNamesDocument), &task.ParticipantNames)
	return task, nil
}

func alignTasksWithMembers(tasks []Task, members []taskMember) []Task {
	memberByID := map[string]taskMember{}
	for _, member := range members {
		memberByID[member.ID] = member
	}
	for index, task := range tasks {
		if owner, found := memberByID[task.OwnerID]; found {
			tasks[index].OwnerName = owner.Name
		}
		names := make([]string, 0, len(task.ParticipantIDs))
		for _, memberID := range task.ParticipantIDs {
			if member, found := memberByID[memberID]; found {
				names = append(names, member.Name)
			}
		}
		if len(names) > 0 {
			tasks[index].ParticipantNames = names
		}
	}
	return tasks
}
