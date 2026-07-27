package admind

type organizationDirectoryResponse struct {
	Records         []organizationDirectoryRecord `json:"records"`
	AvailableGroups []orgGroupRecord              `json:"availableGroups,omitempty"`
}

type organizationDirectoryRecord struct {
	UserID       string `json:"userID,omitempty"`
	Handle       string `json:"handle,omitempty"`
	Name         string `json:"name,omitempty"`
	Email        string `json:"email"`
	Image        string `json:"image,omitempty"`
	HireDate     string `json:"hireDate,omitempty"`
	JobTitle     string `json:"jobTitle,omitempty"`
	GroupID      string `json:"groupID,omitempty"`
	SupervisorID string `json:"supervisorID,omitempty"`
}

func newOrganizationDirectoryResponse(response pagesUsersResponse) organizationDirectoryResponse {
	records := make([]organizationDirectoryRecord, 0, len(response.Records))
	for _, record := range response.Records {
		records = append(records, newOrganizationDirectoryRecord(record))
	}
	return organizationDirectoryResponse{
		Records:         records,
		AvailableGroups: response.AvailableGroups,
	}
}

func newOrganizationDirectoryRecord(record adminUserMutation) organizationDirectoryRecord {
	return organizationDirectoryRecord{
		UserID:       record.UserID,
		Handle:       record.Handle,
		Name:         record.Name,
		Email:        record.Email,
		Image:        record.Image,
		HireDate:     record.HireDate,
		JobTitle:     record.JobTitle,
		GroupID:      record.GroupID,
		SupervisorID: record.SupervisorID,
	}
}
