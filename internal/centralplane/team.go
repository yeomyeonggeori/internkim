package centralplane

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type Team struct {
	TeamID       string `json:"teamID"`
	Name         string `json:"name"`
	ParentTeamID string `json:"parentTeamID"`
}

type OfferedTeam struct {
	TeamID     string `json:"teamID"`
	Name       string `json:"name"`
	ParentName string `json:"parentName"`
}

type answeredTeam struct {
	TeamID       string `json:"teamID"`
	Name         string `json:"name"`
	ParentTeamID string `json:"parentTeamID"`
	Position     int    `json:"position"`
}

func (team answeredTeam) team() Team {
	return Team{TeamID: team.TeamID, Name: team.Name, ParentTeamID: team.ParentTeamID}
}

func (client *Client) Teams(ctx context.Context, requesterEmail string) ([]Team, error) {
	held, errorValue := client.heldTeams(ctx, requesterEmail)
	if errorValue != nil {
		return nil, errorValue
	}
	teams := make([]Team, 0, len(held))
	for _, team := range held {
		teams = append(teams, team.team())
	}
	return teams, nil
}

func (client *Client) AddTeam(
	ctx context.Context,
	administratorEmail string,
	name string,
	parentTeamID string,
	position *int,
) (Team, error) {
	made := answeredTeam{}
	input := map[string]any{"name": strings.TrimSpace(name)}
	if strings.TrimSpace(parentTeamID) != "" {
		input["parentHint"] = strings.TrimSpace(parentTeamID)
	}
	if position != nil {
		input["position"] = *position
	}
	if errorValue := client.runRecordTool(ctx, administratorEmail, "team_add", input, &made); errorValue != nil {
		return Team{}, errorValue
	}
	return made.team(), nil
}

// The route this replaced took the whole tree and replaced it. The tools name
// one organization each, so the offered tree is diffed against the held one:
// what is new is added, what moved or was renamed is updated, and what nobody
// offered is removed from the leaves up, because an organization with
// organizations under it refuses to go.
func (client *Client) SettleTeams(ctx context.Context, teams []OfferedTeam) ([]Team, error) {
	if client == nil || !client.settings.Configured() {
		return nil, fmt.Errorf("central plane is not configured")
	}
	administratorEmail, errorValue := client.claimedAdministratorEmail()
	if errorValue != nil {
		return nil, errorValue
	}
	held, errorValue := client.heldTeams(ctx, administratorEmail)
	if errorValue != nil {
		return nil, errorValue
	}

	settled, errorValue := client.settledTeams(ctx, administratorEmail, held, teams)
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := client.removeTeamsNobodyOffered(ctx, administratorEmail, held, settled); errorValue != nil {
		return nil, errorValue
	}

	answered := make([]Team, 0, len(settled))
	for _, team := range settled {
		answered = append(answered, team.team())
	}
	return answered, nil
}

func (client *Client) settledTeams(
	ctx context.Context,
	administratorEmail string,
	held []answeredTeam,
	offered []OfferedTeam,
) ([]answeredTeam, error) {
	heldByID := map[string]answeredTeam{}
	heldByName := map[string]answeredTeam{}
	for _, team := range held {
		heldByID[team.TeamID] = team
		heldByName[normalizedTeamName(team.Name)] = team
	}

	settled := make([]answeredTeam, 0, len(offered))
	for position, offeredTeam := range offered {
		name := strings.TrimSpace(offeredTeam.Name)
		if name == "" {
			return nil, fmt.Errorf("every organization needs a name")
		}
		known, isKnown := heldByID[strings.TrimSpace(offeredTeam.TeamID)]
		if !isKnown {
			known, isKnown = heldByName[normalizedTeamName(name)]
		}
		if isKnown {
			settled = append(settled, answeredTeam{
				TeamID:       known.TeamID,
				Name:         name,
				ParentTeamID: known.ParentTeamID,
				Position:     known.Position,
			})
			continue
		}
		made, errorValue := client.AddTeam(ctx, administratorEmail, name, "", &position)
		if errorValue != nil {
			return nil, errorValue
		}
		settled = append(settled, answeredTeam{TeamID: made.TeamID, Name: made.Name, Position: position})
	}

	return client.parentedTeams(ctx, administratorEmail, held, settled, offered)
}

func (client *Client) parentedTeams(
	ctx context.Context,
	administratorEmail string,
	held []answeredTeam,
	settled []answeredTeam,
	offered []OfferedTeam,
) ([]answeredTeam, error) {
	idByName := map[string]string{}
	for _, team := range settled {
		idByName[normalizedTeamName(team.Name)] = team.TeamID
	}
	heldByID := map[string]answeredTeam{}
	for _, team := range held {
		heldByID[team.TeamID] = team
	}

	for index, team := range settled {
		parentTeamID := idByName[normalizedTeamName(offered[index].ParentName)]
		if parentTeamID == team.TeamID {
			parentTeamID = ""
		}
		wanted := answeredTeam{
			TeamID:       team.TeamID,
			Name:         team.Name,
			ParentTeamID: parentTeamID,
			Position:     index,
		}
		if heldByID[team.TeamID] == wanted {
			settled[index] = wanted
			continue
		}
		change := map[string]any{
			"teamHint":   team.TeamID,
			"name":       team.Name,
			"parentHint": parentTeamID,
			"position":   index,
		}
		written := answeredTeam{}
		if errorValue := client.runRecordTool(ctx, administratorEmail, "team_update", change, &written); errorValue != nil {
			return nil, errorValue
		}
		settled[index] = written
	}
	return settled, nil
}

func (client *Client) removeTeamsNobodyOffered(
	ctx context.Context,
	administratorEmail string,
	held []answeredTeam,
	settled []answeredTeam,
) error {
	kept := map[string]bool{}
	for _, team := range settled {
		kept[team.TeamID] = true
	}
	for _, team := range doomedTeamsDeepestFirst(held, kept) {
		if errorValue := client.runRecordTool(ctx, administratorEmail, "team_delete",
			map[string]string{"teamHint": team.TeamID}, nil); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func doomedTeamsDeepestFirst(held []answeredTeam, kept map[string]bool) []answeredTeam {
	byID := map[string]answeredTeam{}
	for _, team := range held {
		byID[team.TeamID] = team
	}
	depthOf := func(team answeredTeam) int {
		depth := 0
		walked := team
		for walked.ParentTeamID != "" && depth < len(held) {
			parent, isHeld := byID[walked.ParentTeamID]
			if !isHeld {
				break
			}
			walked = parent
			depth++
		}
		return depth
	}

	doomed := []answeredTeam{}
	for _, team := range held {
		if !kept[team.TeamID] {
			doomed = append(doomed, team)
		}
	}
	sort.SliceStable(doomed, func(first int, second int) bool {
		return depthOf(doomed[first]) > depthOf(doomed[second])
	})
	return doomed
}

func (client *Client) heldTeams(ctx context.Context, requesterEmail string) ([]answeredTeam, error) {
	if client == nil || !client.settings.Configured() {
		return nil, fmt.Errorf("central plane is not configured")
	}
	var answer struct {
		Teams []answeredTeam `json:"teams"`
	}
	if errorValue := client.runRecordTool(ctx, requesterEmail, "team_list", map[string]any{}, &answer); errorValue != nil {
		return nil, errorValue
	}
	return answer.Teams, nil
}

func normalizedTeamName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
