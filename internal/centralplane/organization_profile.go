package centralplane

import (
	"context"
	"fmt"
	"strings"
)

type OrganizationProfile struct {
	Email           string `json:"email"`
	JobTitle        string `json:"jobTitle"`
	PhoneNumber     string `json:"phoneNumber"`
	HireDate        string `json:"hireDate"`
	SupervisorEmail string `json:"supervisorEmail"`
	TeamName        string `json:"teamName"`
}

// One person_update per profile, as the administrator the roster names. The
// route this replaced created a team it had never heard of rather than
// refusing the profile, so the teams these profiles name are settled first and
// the profile is written against the team id, which resolves exactly.
func (client *Client) WriteOrganizationProfiles(ctx context.Context, profiles []OrganizationProfile) ([]string, error) {
	if len(profiles) == 0 {
		return nil, nil
	}
	if client == nil || !client.settings.Configured() {
		return nil, fmt.Errorf("central plane is not configured")
	}
	administratorEmail, errorValue := client.anAdministratorEmail(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	teamIDByName, errorValue := client.teamIDsForNames(ctx, administratorEmail, teamNamesOf(profiles))
	if errorValue != nil {
		return nil, errorValue
	}

	written := []string{}
	for _, profile := range profiles {
		email := strings.ToLower(strings.TrimSpace(profile.Email))
		if email == "" {
			return nil, fmt.Errorf("every profile names the person it belongs to")
		}
		change := map[string]string{
			"personHint":     email,
			"jobTitle":       strings.TrimSpace(profile.JobTitle),
			"phoneNumber":    strings.TrimSpace(profile.PhoneNumber),
			"hireDate":       strings.TrimSpace(profile.HireDate),
			"supervisorHint": strings.ToLower(strings.TrimSpace(profile.SupervisorEmail)),
			"teamHint":       teamIDByName[normalizedTeamName(profile.TeamName)],
		}
		if errorValue := client.runRecordTool(ctx, administratorEmail, "person_update", change, nil); errorValue != nil {
			return nil, errorValue
		}
		written = append(written, email)
	}
	return written, nil
}

func teamNamesOf(profiles []OrganizationProfile) []string {
	names := []string{}
	for _, profile := range profiles {
		if name := strings.TrimSpace(profile.TeamName); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func (client *Client) teamIDsForNames(
	ctx context.Context,
	administratorEmail string,
	names []string,
) (map[string]string, error) {
	teamIDByName := map[string]string{}
	if len(names) == 0 {
		return teamIDByName, nil
	}
	held, errorValue := client.Teams(ctx, administratorEmail)
	if errorValue != nil {
		return nil, errorValue
	}
	for _, team := range held {
		teamIDByName[normalizedTeamName(team.Name)] = team.TeamID
	}
	for _, name := range names {
		if _, isHeld := teamIDByName[normalizedTeamName(name)]; isHeld {
			continue
		}
		made, errorValue := client.AddTeam(ctx, administratorEmail, name, "", nil)
		if errorValue != nil {
			return nil, errorValue
		}
		teamIDByName[normalizedTeamName(name)] = made.TeamID
	}
	return teamIDByName, nil
}
