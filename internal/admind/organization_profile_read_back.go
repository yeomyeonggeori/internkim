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
	log.Printf("organization profiles settled with the company directory: %d", len(changed))
	if errorValue := service.writeOrganizationProfilesToTheDirectory(ctx, changed); errorValue != nil {
		log.Printf("the organization read back did not teach the directory: %v", errorValue)
	}
}

func (service *Service) organizationGroupsCoveringTheDirectory(
	ctx context.Context,
	members []centralplane.Member,
) (map[string]string, error) {
	groups, errorValue := service.readOrganizationGroups(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	groupIDByName := map[string]string{}
	for _, group := range groups {
		groupIDByName[strings.ToLower(strings.TrimSpace(group.Name))] = strings.TrimSpace(group.ID)
	}
	missing := groupsTheDirectoryNamesAndTheDeviceLacks(members, groupIDByName)
	if len(missing) == 0 {
		return groupIDByName, nil
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
) []orgGroupRecord {
	missing := []orgGroupRecord{}
	seen := map[string]bool{}
	for _, member := range members {
		name := strings.TrimSpace(member.TeamName)
		key := strings.ToLower(name)
		if name == "" || seen[key] || groupIDByName[key] != "" {
			continue
		}
		teamID := strings.TrimSpace(member.TeamID)
		if teamID == "" {
			continue
		}
		seen[key] = true
		missing = append(missing, orgGroupRecord{ID: teamID, Name: name})
	}
	return missing
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
		if directoryOwnedFieldsDiffer(held, taken) {
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
	taken.JobTitle = firstNonEmpty(strings.TrimSpace(member.JobTitle), held.JobTitle)
	taken.PhoneNumber = firstNonEmpty(strings.TrimSpace(member.PhoneNumber), held.PhoneNumber)
	taken.HireDate = firstNonEmpty(strings.TrimSpace(member.HireDate), held.HireDate)
	taken.SupervisorID = firstNonEmpty(memberIDByEmail[strings.ToLower(strings.TrimSpace(member.SupervisorEmail))], held.SupervisorID)
	taken.GroupID = firstNonEmpty(groupIDByName[strings.ToLower(strings.TrimSpace(member.TeamName))], held.GroupID)
	return normalizeOrganizationProfile(taken)
}

func directoryOwnedFieldsDiffer(held organizationProfile, taken organizationProfile) bool {
	return held.MemberID != taken.MemberID ||
		held.Email != taken.Email ||
		held.JobTitle != taken.JobTitle ||
		held.PhoneNumber != taken.PhoneNumber ||
		held.HireDate != taken.HireDate ||
		held.SupervisorID != taken.SupervisorID ||
		held.GroupID != taken.GroupID
}
