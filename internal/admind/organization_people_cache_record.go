package admind

type organizationCachedUserRecord struct {
	MemberID     string `json:"memberID,omitempty"`
	Handle       string `json:"handle,omitempty"`
	Name         string `json:"name,omitempty"`
	Email        string `json:"email"`
	Image        string `json:"image,omitempty"`
	HireDate     string `json:"hireDate,omitempty"`
	JobTitle     string `json:"jobTitle,omitempty"`
	GroupID      string `json:"groupID,omitempty"`
	PhoneNumber  string `json:"phoneNumber,omitempty"`
	SupervisorID string `json:"supervisorID,omitempty"`
}

func newOrganizationCachedUserRecord(record adminUserMutation) organizationCachedUserRecord {
	return organizationCachedUserRecord{
		MemberID:     record.MemberID,
		Handle:       record.Handle,
		Name:         record.Name,
		Email:        record.Email,
		Image:        record.Image,
		HireDate:     record.HireDate,
		JobTitle:     record.JobTitle,
		GroupID:      record.GroupID,
		PhoneNumber:  record.PhoneNumber,
		SupervisorID: record.SupervisorID,
	}
}

func applyOrganizationCachedUserRecord(record adminUserMutation, cachedRecord organizationCachedUserRecord) adminUserMutation {
	record.MemberID = cachedRecord.MemberID
	record.Handle = cachedRecord.Handle
	record.Name = cachedRecord.Name
	record.Email = cachedRecord.Email
	record.Image = cachedRecord.Image
	record.HireDate = cachedRecord.HireDate
	record.JobTitle = cachedRecord.JobTitle
	record.GroupID = cachedRecord.GroupID
	record.PhoneNumber = cachedRecord.PhoneNumber
	record.SupervisorID = cachedRecord.SupervisorID
	return record
}
