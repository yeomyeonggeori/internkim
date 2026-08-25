package admind

import (
	"encoding/json"
	"net/http"
	"strings"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

type organizationDirectoryCoverage struct {
	Members      int  `json:"members"`
	Profiles     int  `json:"profiles"`
	Described    int  `json:"described"`
	Undescribed  int  `json:"undescribed"`
	Seeded       int  `json:"seeded"`
	CanReadBack  bool `json:"canReadBack"`
	IsSeedNeeded bool `json:"isSeedNeeded"`
}

func (service *Service) handleOrganizationDirectoryCoverage(responseWriter http.ResponseWriter, request *http.Request) {
	shouldSeed := request.URL.Query().Get("seed") == "true"
	coverage, errorValue := service.organizationDirectoryCoverage(request, shouldSeed)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode(coverage)
}

func (service *Service) organizationDirectoryCoverage(request *http.Request, shouldSeed bool) (organizationDirectoryCoverage, error) {
	client := service.centralPlane()
	if client == nil {
		return organizationDirectoryCoverage{}, errNoCompanyDirectory
	}
	members, errorValue := client.Members(request.Context())
	if errorValue != nil {
		return organizationDirectoryCoverage{}, errorValue
	}
	profiles, errorValue := service.readOrganizationProfiles(request.Context())
	if errorValue != nil {
		return organizationDirectoryCoverage{}, errorValue
	}

	described := membersTheDirectoryDescribes(members)
	coverage := organizationDirectoryCoverage{
		Members:      len(members),
		Profiles:     len(profiles),
		Described:    described,
		Undescribed:  len(members) - described,
		IsSeedNeeded: isDirectorySeedNeeded(members, profiles),
	}
	coverage.CanReadBack = !coverage.IsSeedNeeded
	if !shouldSeed {
		return coverage, nil
	}
	if errorValue := service.writeOrganizationProfilesToTheDirectory(request.Context(), profiles); errorValue != nil {
		return coverage, errorValue
	}
	coverage.Seeded = len(profiles)
	coverage.IsSeedNeeded = false
	coverage.CanReadBack = true
	return coverage, nil
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

func isDirectorySeedNeeded(members []centralplane.Member, profiles []organizationProfile) bool {
	if membersTheDirectoryDescribes(members) > 0 {
		return false
	}
	for _, profile := range profiles {
		if profileDescribesSomebody(normalizeOrganizationProfile(profile)) {
			return true
		}
	}
	return false
}

func profileDescribesSomebody(profile organizationProfile) bool {
	return profile.JobTitle != "" || profile.PhoneNumber != "" || profile.HireDate != "" ||
		profile.GroupID != "" || profile.SupervisorID != ""
}
