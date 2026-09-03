package admind

import (
	"encoding/json"
	"net/http"
	"strings"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

type organizationRecordCoverage struct {
	Members     int      `json:"members"`
	Described   int      `json:"described"`
	Undescribed int      `json:"undescribed"`
	Uncarried   int      `json:"uncarried"`
	Carried     int      `json:"carried"`
	Refused     []string `json:"refused,omitempty"`
}

func (service *Service) handleOrganizationRecordCoverage(responseWriter http.ResponseWriter, request *http.Request) {
	shouldCarry := request.URL.Query().Get("carry") == "true"
	coverage, errorValue := service.organizationRecordCoverage(request, shouldCarry)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode(coverage)
}

func (service *Service) organizationRecordCoverage(request *http.Request, shouldCarry bool) (organizationRecordCoverage, error) {
	members, errorValue := service.companyMembers(request.Context())
	if errorValue != nil {
		return organizationRecordCoverage{}, errorValue
	}

	described := membersTheDirectoryDescribes(members)
	coverage := organizationRecordCoverage{
		Members:     len(members),
		Described:   described,
		Undescribed: len(members) - described,
	}

	database, errorValue := service.openOrganizationDatabase(request.Context())
	if errorValue != nil {
		return coverage, errorValue
	}
	uncarried, errorValue := uncarriedOrganizationProfiles(request.Context(), database)
	database.Close()
	if errorValue != nil {
		return coverage, errorValue
	}
	coverage.Uncarried = len(uncarried)
	if !shouldCarry {
		return coverage, nil
	}

	report, errorValue := service.carryTheOrganizationIntoTheRecord(request.Context())
	coverage.Carried = report.Profiles
	coverage.Refused = report.Refused
	coverage.Uncarried -= report.Profiles
	return coverage, errorValue
}

func membersTheDirectoryDescribes(members []centralplane.Member) int {
	described := 0
	for _, member := range members {
		if memberCarriesAProfile(member) {
			described++
		}
	}
	return described
}

func memberCarriesAProfile(member centralplane.Member) bool {
	return strings.TrimSpace(member.JobTitle) != "" ||
		strings.TrimSpace(member.PhoneNumber) != "" ||
		strings.TrimSpace(member.HireDate) != "" ||
		strings.TrimSpace(member.TeamName) != "" ||
		strings.TrimSpace(member.SupervisorEmail) != ""
}
