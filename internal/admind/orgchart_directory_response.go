package admind

type orgchartDirectoryResponse struct {
	Records         []orgchartDirectoryRecord `json:"records"`
	AvailableGroups []orgGroupRecord          `json:"availableGroups,omitempty"`
}

type orgchartDirectoryRecord struct {
	UserID         string   `json:"userID,omitempty"`
	Handle         string   `json:"handle,omitempty"`
	Name           string   `json:"name,omitempty"`
	Email          string   `json:"email"`
	Image          string   `json:"image,omitempty"`
	HireDate       string   `json:"hireDate,omitempty"`
	JobTitle       string   `json:"jobTitle,omitempty"`
	Group          string   `json:"group,omitempty"`
	PrimaryGroupID string   `json:"primaryGroupID,omitempty"`
	GroupIDs       []string `json:"groupIDs,omitempty"`
	SupervisorID   string   `json:"supervisorID,omitempty"`
}

func newOrgchartDirectoryResponse(response pagesUsersResponse) orgchartDirectoryResponse {
	records := make([]orgchartDirectoryRecord, 0, len(response.Records))
	for _, record := range response.Records {
		records = append(records, newOrgchartDirectoryRecord(record))
	}
	return orgchartDirectoryResponse{
		Records:         records,
		AvailableGroups: response.AvailableGroups,
	}
}

func newOrgchartDirectoryRecord(record adminUserMutation) orgchartDirectoryRecord {
	return orgchartDirectoryRecord{
		UserID:         record.UserID,
		Handle:         record.Handle,
		Name:           record.Name,
		Email:          record.Email,
		Image:          record.Image,
		HireDate:       record.HireDate,
		JobTitle:       record.JobTitle,
		Group:          record.Group,
		PrimaryGroupID: record.PrimaryGroupID,
		GroupIDs:       record.GroupIDs,
		SupervisorID:   record.SupervisorID,
	}
}
