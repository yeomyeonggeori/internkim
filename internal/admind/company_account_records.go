package admind

import (
	"context"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/centralplane"
)

// The company's member table is the one place a person is added, renamed, given a
// role or removed. The host reads it to answer with and writes it through the same
// door; it keeps no list of its own that the two could disagree about.
func (service *Service) companyUserRecords(ctx context.Context) ([]adminUserMutation, error) {
	members, errorValue := service.companyMembers(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	records := make([]adminUserMutation, 0, len(members))
	for _, member := range members {
		if member.HasLeftTheCompany() {
			continue
		}
		records = append(records, accountRecordOfCompanyMember(member))
	}
	return records, nil
}

// An account record names the person and their standing in the company. What the
// organization says about them (title, phone, hire date, team, supervisor) is a
// separate concern with a separate owner and is merged in by the reader that
// wants it, never carried here.
func accountRecordOfCompanyMember(member centralplane.Member) adminUserMutation {
	return adminUserMutation{
		MemberID: strings.TrimSpace(member.MemberID),
		Handle:   handleFromEmail(member.Email),
		Name:     strings.TrimSpace(member.Name),
		Email:    normalizedRosterEmail(member.Email),
		Note:     strings.TrimSpace(member.Note),
		Role:     normalizeAdminUserRole(member.Role),
		Circles:  member.Circles,
		Status:   strings.TrimSpace(member.Status),
	}
}

func (service *Service) saveCompanyUserRecord(ctx context.Context, write centralplane.MemberWrite) error {
	client := service.centralPlane()
	if client == nil {
		return errNoCompanyDirectory
	}
	_, errorValue := client.SaveMember(ctx, write)
	return errorValue
}

func (service *Service) withdrawCompanyUser(ctx context.Context, email string) error {
	client := service.centralPlane()
	if client == nil {
		return errNoCompanyDirectory
	}
	return client.WithdrawMember(ctx, email)
}

// A handle is an identifier the company issues, not something a messenger hands
// back. The address already carries one everybody recognises.
func handleFromEmail(email string) string {
	address := strings.ToLower(strings.TrimSpace(email))
	if index := strings.Index(address, "@"); index > 0 {
		return address[:index]
	}
	return address
}
