package admind

import (
	"context"
	"log"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

const organizationReadBackInterval = 10 * time.Minute

func (service *Service) keepOrganizationProfilesReadBack(ctx context.Context) {
	if service.centralPlane() == nil {
		return
	}
	service.readOrganizationProfilesBackFromTheDirectory(ctx)

	ticker := time.NewTicker(organizationReadBackInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			service.readOrganizationProfilesBackFromTheDirectory(ctx)
		}
	}
}

func (service *Service) readOrganizationProfilesBackFromTheDirectory(ctx context.Context) {
	client := service.centralPlane()
	if client == nil {
		return
	}
	members, errorValue := client.Members(ctx)
	if errorValue != nil {
		log.Printf("the company directory did not answer the organization read back: %v", errorValue)
		return
	}
	profiles, errorValue := service.readOrganizationProfiles(ctx)
	if errorValue != nil {
		log.Printf("the organization read back did not read what this device holds: %v", errorValue)
		return
	}
	if isDirectorySeedNeeded(members, profiles) {
		log.Printf("the organization read back is holding off: the company directory describes nobody yet and this device describes %d people; seed it with `internkim recover ssh -action organization-seed-the-directory`", len(profiles))
		return
	}
	groupIDByName, errorValue := service.organizationGroupsCoveringTheDirectory(ctx, members)
	if errorValue != nil {
		log.Printf("the organization read back did not settle the groups: %v", errorValue)
		return
	}
	changed, errorValue := service.organizationProfilesTakenFromTheDirectory(ctx, members, groupIDByName)
	if errorValue != nil {
		log.Printf("the organization read back did not merge: %v", errorValue)
		return
	}
	if len(changed) == 0 {
		return
	}
	if errorValue := service.writeOrganizationProfiles(ctx, changed); errorValue != nil {
		log.Printf("the organization read back did not write: %v", errorValue)
		return
	}
	log.Printf("organization profiles taken from the company directory: %d", len(changed))
}

func (service *Service) organizationGroupsCoveringTheDirectory(
	ctx context.Context,
	members []centralplane.Member,
) (map[string]string, error) {
	groups, errorValue := service.readOrganizationGroups(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	heldIDs := map[string]bool{}
	groupIDByName := map[string]string{}
	for _, group := range groups {
		heldIDs[strings.TrimSpace(group.ID)] = true
		groupIDByName[strings.ToLower(strings.TrimSpace(group.Name))] = strings.TrimSpace(group.ID)
	}
	groups = groupsRenamedByTheDirectory(groups, members)
	for _, group := range groups {
		groupIDByName[strings.ToLower(strings.TrimSpace(group.Name))] = strings.TrimSpace(group.ID)
	}
	missing := groupsTheDirectoryNamesAndTheDeviceLacks(members, groupIDByName, heldIDs)
	if len(missing) == 0 {
		return groupIDByName, service.writeOrganizationGroups(ctx, groups)
	}
	if errorValue := service.writeOrganizationGroups(ctx, append(groups, missing...)); errorValue != nil {
		return nil, errorValue
	}
	for _, group := range missing {
		groupIDByName[strings.ToLower(group.Name)] = group.ID
	}
	log.Printf("teams taken from the company directory: %d", len(missing))
	return groupIDByName, nil
}

func groupsTheDirectoryNamesAndTheDeviceLacks(
	members []centralplane.Member,
	groupIDByName map[string]string,
	heldIDs map[string]bool,
) []orgGroupRecord {
	missing := []orgGroupRecord{}
	seen := map[string]bool{}
	for _, member := range members {
		name := strings.TrimSpace(member.TeamName)
		key := strings.ToLower(name)
		teamID := strings.TrimSpace(member.TeamID)
		if name == "" || teamID == "" || seen[key] || groupIDByName[key] != "" || heldIDs[teamID] {
			continue
		}
		seen[key] = true
		missing = append(missing, orgGroupRecord{ID: teamID, Name: name})
	}
	return missing
}

func groupsRenamedByTheDirectory(groups []orgGroupRecord, members []centralplane.Member) []orgGroupRecord {
	nameByTeamID := map[string]string{}
	for _, member := range members {
		teamID := strings.TrimSpace(member.TeamID)
		if teamID != "" {
			nameByTeamID[teamID] = strings.TrimSpace(member.TeamName)
		}
	}
	renamed := make([]orgGroupRecord, 0, len(groups))
	for _, group := range groups {
		name := nameByTeamID[strings.TrimSpace(group.ID)]
		if name == "" || name == strings.TrimSpace(group.Name) {
			renamed = append(renamed, group)
			continue
		}
		renamed = append(renamed, orgGroupRecord{ID: group.ID, Name: name, ParentID: group.ParentID})
	}
	return renamed
}

func (service *Service) organizationProfilesTakenFromTheDirectory(
	ctx context.Context,
	members []centralplane.Member,
	groupIDByName map[string]string,
) ([]organizationProfile, error) {
	profiles, errorValue := service.readOrganizationProfiles(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	heldByEmail := map[string]organizationProfile{}
	for _, profile := range profiles {
		normalized := normalizeOrganizationProfile(profile)
		heldByEmail[normalized.Email] = normalized
	}
	memberIDByEmail := map[string]string{}
	for _, member := range members {
		memberIDByEmail[strings.ToLower(strings.TrimSpace(member.Email))] = strings.TrimSpace(member.MemberID)
	}

	changed := []organizationProfile{}
	for _, member := range members {
		email := strings.ToLower(strings.TrimSpace(member.Email))
		if email == "" {
			continue
		}
		held := heldByEmail[email]
		taken := organizationProfileTakenFrom(held, member, memberIDByEmail, groupIDByName)
		if taken != held {
			changed = append(changed, taken)
		}
	}
	return changed, nil
}

func organizationProfileTakenFrom(
	held organizationProfile,
	member centralplane.Member,
	memberIDByEmail map[string]string,
	groupIDByName map[string]string,
) organizationProfile {
	taken := held
	taken.Email = strings.ToLower(strings.TrimSpace(member.Email))
	if taken.MemberID == "" {
		taken.MemberID = strings.TrimSpace(member.MemberID)
	}
	taken.JobTitle = strings.TrimSpace(member.JobTitle)
	taken.PhoneNumber = strings.TrimSpace(member.PhoneNumber)
	taken.HireDate = strings.TrimSpace(member.HireDate)
	taken.Status = strings.TrimSpace(member.Status)
	taken.SupervisorID = memberIDByEmail[strings.ToLower(strings.TrimSpace(member.SupervisorEmail))]
	taken.GroupID = groupIDByName[strings.ToLower(strings.TrimSpace(member.TeamName))]
	return normalizeOrganizationProfile(taken)
}
