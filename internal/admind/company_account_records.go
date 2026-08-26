package admind

import (
	"context"
	"fmt"
	"strings"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

// Who works here is the company's answer. The fleet register was the account
// list before a company existed to hold one, and a host that still asked it got
// a different roster than the one the person signs in to.
//
// Only account concerns travel on this record. Job title, supervisor, phone
// number and hire date belong to organization_profiles, which merges them in
// where a caller needs them.
func (service *Service) companyUserRecords(ctx context.Context) ([]adminUserMutation, error) {
	client := service.centralPlane()
	if client == nil {
		return nil, fmt.Errorf("this host has no company directory configured")
	}
	members, errorValue := client.Members(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	records := make([]adminUserMutation, 0, len(members))
	for _, member := range members {
		records = append(records, accountRecordOfCompanyMember(member))
	}
	return adminUserRecordsWithProfileImages(records), nil
}

func accountRecordOfCompanyMember(member centralplane.Member) adminUserMutation {
	return adminUserMutation{
		MemberID:           member.MemberID,
		Email:              strings.ToLower(strings.TrimSpace(member.Email)),
		Name:               strings.TrimSpace(member.Name),
		Role:               normalizeAdminUserRole(member.Role),
		Circles:            member.Circles,
		Status:             member.Status,
		Note:               member.Note,
		MattermostUserID:   member.Messenger["mattermost"],
		MattermostUsername: member.Messenger["mattermostUsername"],
	}
}

// The company is the only place a person is added, removed or promoted. A device
// that wrote somewhere else left two answers to one question.
func (service *Service) saveCompanyUserRecord(ctx context.Context, write centralplane.MemberWrite) error {
	client := service.centralPlane()
	if client == nil {
		return fmt.Errorf("this host has no company directory configured")
	}
	_, errorValue := client.SaveMember(ctx, write)
	return errorValue
}

func (service *Service) withdrawCompanyUser(ctx context.Context, email string) error {
	client := service.centralPlane()
	if client == nil {
		return fmt.Errorf("this host has no company directory configured")
	}
	return client.WithdrawMember(ctx, email)
}
