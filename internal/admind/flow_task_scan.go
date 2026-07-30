package admind

import (
	"database/sql"
	"encoding/json"
)

func scanFlowTask(rows *sql.Rows) (flowTask, error) {
	var task flowTask
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
		&task.Goal,
		&task.Size,
		&task.Status,
		&task.StatusRank,
		&task.StartDate,
		&task.EndDate,
		&task.Flag,
		&task.RequestReason,
		&task.DecisionReason,
		&task.MattermostPostID,
		&task.CalendarEventID,
		&task.CreatedAt,
	)
	if errorValue != nil {
		return flowTask{}, errorValue
	}
	_ = json.Unmarshal([]byte(participantIDsDocument), &task.ParticipantIDs)
	_ = json.Unmarshal([]byte(participantNamesDocument), &task.ParticipantNames)
	return task, nil
}

func alignFlowTasksWithMembers(tasks []flowTask, members []flowMember) []flowTask {
	memberByID := map[string]flowMember{}
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
