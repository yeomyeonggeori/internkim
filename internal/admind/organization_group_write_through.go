package admind

import (
	"context"
	"log"
	"strings"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

func (service *Service) settleOrganizationGroupsWithTheDirectory(ctx context.Context, groups []orgGroupRecord) error {
	client := service.centralPlane()
	if client == nil {
		return nil
	}
	settled, errorValue := client.SettleTeams(ctx, offeredTeamsFor(groups))
	if errorValue != nil {
		return errorValue
	}
	adopted, renamedIDs := groupsAdoptingTeamIDs(groups, settled)
	if len(adopted) == 0 {
		return nil
	}
	log.Printf("teams settled with the company directory: %d", len(settled))
	if errorValue := service.writeOrganizationGroups(ctx, adopted); errorValue != nil {
		return errorValue
	}
	return service.movePeopleOntoRenamedGroups(ctx, renamedIDs)
}

func (service *Service) movePeopleOntoRenamedGroups(ctx context.Context, renamedIDs map[string]string) error {
	if len(renamedIDs) == 0 {
		return nil
	}
	profiles, errorValue := service.readOrganizationProfiles(ctx)
	if errorValue != nil {
		return errorValue
	}
	moved := []organizationProfile{}
	for _, profile := range profiles {
		normalized := normalizeOrganizationProfile(profile)
		teamID := renamedIDs[normalized.GroupID]
		if teamID == "" {
			continue
		}
		normalized.GroupID = teamID
		moved = append(moved, normalized)
	}
	if len(moved) == 0 {
		return nil
	}
	log.Printf("people moved onto the team ids the directory issued: %d", len(moved))
	return service.writeOrganizationProfiles(ctx, moved)
}

func offeredTeamsFor(groups []orgGroupRecord) []centralplane.OfferedTeam {
	nameByID := map[string]string{}
	for _, group := range groups {
		nameByID[strings.TrimSpace(group.ID)] = strings.TrimSpace(group.Name)
	}
	offered := make([]centralplane.OfferedTeam, 0, len(groups))
	for _, group := range groups {
		offered = append(offered, centralplane.OfferedTeam{
			TeamID:     strings.TrimSpace(group.ID),
			Name:       strings.TrimSpace(group.Name),
			ParentName: nameByID[strings.TrimSpace(group.ParentID)],
		})
	}
	return offered
}

func groupsAdoptingTeamIDs(groups []orgGroupRecord, settled []centralplane.Team) ([]orgGroupRecord, map[string]string) {
	teamIDByName := map[string]string{}
	for _, team := range settled {
		teamIDByName[strings.ToLower(strings.TrimSpace(team.Name))] = strings.TrimSpace(team.TeamID)
	}
	adopted := make([]orgGroupRecord, 0, len(groups))
	renamedIDs := map[string]string{}
	for _, group := range groups {
		teamID := teamIDByName[strings.ToLower(strings.TrimSpace(group.Name))]
		if teamID == "" {
			adopted = append(adopted, group)
			continue
		}
		if teamID != strings.TrimSpace(group.ID) {
			renamedIDs[strings.TrimSpace(group.ID)] = teamID
		}
		adopted = append(adopted, orgGroupRecord{
			ID:       teamID,
			Name:     group.Name,
			ParentID: teamIDByName[strings.ToLower(parentNameOf(groups, group.ParentID))],
		})
	}
	if len(renamedIDs) == 0 {
		return nil, nil
	}
	return adopted, renamedIDs
}

func parentNameOf(groups []orgGroupRecord, parentID string) string {
	for _, group := range groups {
		if strings.TrimSpace(group.ID) == strings.TrimSpace(parentID) {
			return strings.TrimSpace(group.Name)
		}
	}
	return ""
}
