package admind

type organizationCachedUserRecord struct {
	UserID            string   `json:"userID,omitempty"`
	Handle            string   `json:"handle,omitempty"`
	Name              string   `json:"name,omitempty"`
	Email             string   `json:"email"`
	Image             string   `json:"image,omitempty"`
	HireDate          string   `json:"hireDate,omitempty"`
	JobTitle          string   `json:"jobTitle,omitempty"`
	Group             string   `json:"group,omitempty"`
	PositionLevel     int      `json:"positionLevel,omitempty"`
	PrimaryGroupID    string   `json:"primaryGroupID,omitempty"`
	GroupIDs          []string `json:"groupIDs,omitempty"`
	SupervisorID      string   `json:"supervisorID,omitempty"`
	ProjectIDs        []string `json:"projectIDs,omitempty"`
	TeamRole          string   `json:"teamRole,omitempty"`
	EmploymentStatus  string   `json:"employmentStatus,omitempty"`
	IsOrganizationVisible bool     `json:"isOrganizationVisible,omitempty"`
}

func newOrganizationCachedUserRecord(record adminUserMutation) organizationCachedUserRecord {
	return organizationCachedUserRecord{
		UserID:            record.UserID,
		Handle:            record.Handle,
		Name:              record.Name,
		Email:             record.Email,
		Image:             record.Image,
		HireDate:          record.HireDate,
		JobTitle:          record.JobTitle,
		Group:             record.Group,
		PositionLevel:     record.PositionLevel,
		PrimaryGroupID:    record.PrimaryGroupID,
		GroupIDs:          append([]string(nil), record.GroupIDs...),
		SupervisorID:      record.SupervisorID,
		ProjectIDs:        append([]string(nil), record.ProjectIDs...),
		TeamRole:          record.TeamRole,
		EmploymentStatus:  record.EmploymentStatus,
		IsOrganizationVisible: record.IsOrganizationVisible,
	}
}

func applyOrganizationCachedUserRecord(record adminUserMutation, cachedRecord organizationCachedUserRecord) adminUserMutation {
	record.UserID = cachedRecord.UserID
	record.Handle = cachedRecord.Handle
	record.Name = cachedRecord.Name
	record.Email = cachedRecord.Email
	record.Image = cachedRecord.Image
	record.HireDate = cachedRecord.HireDate
	record.JobTitle = cachedRecord.JobTitle
	record.Group = cachedRecord.Group
	record.PositionLevel = cachedRecord.PositionLevel
	record.PrimaryGroupID = cachedRecord.PrimaryGroupID
	record.GroupIDs = append([]string(nil), cachedRecord.GroupIDs...)
	record.SupervisorID = cachedRecord.SupervisorID
	record.ProjectIDs = append([]string(nil), cachedRecord.ProjectIDs...)
	record.TeamRole = cachedRecord.TeamRole
	record.EmploymentStatus = cachedRecord.EmploymentStatus
	record.IsOrganizationVisible = cachedRecord.IsOrganizationVisible
	return record
}
