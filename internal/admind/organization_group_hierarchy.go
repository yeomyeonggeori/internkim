package admind

import (
	"errors"
	"fmt"
)

var errOrganizationGroupInvalidHierarchy = errors.New("invalid organization hierarchy")

func validateOrganizationGroupHierarchy(groups []orgGroupRecord) error {
	parentIDByGroupID := make(map[string]string, len(groups))
	for _, group := range groups {
		parentIDByGroupID[group.ID] = group.ParentID
	}
	for _, group := range groups {
		if group.ParentID != "" {
			if _, found := parentIDByGroupID[group.ParentID]; !found {
				return fmt.Errorf("%w: organization %q references unknown parent %q", errOrganizationGroupInvalidHierarchy, group.ID, group.ParentID)
			}
		}
		if errorValue := validateOrganizationGroupParentChain(group.ID, parentIDByGroupID); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func validateOrganizationGroupParentChain(groupID string, parentIDByGroupID map[string]string) error {
	visitedGroupIDs := map[string]bool{}
	currentGroupID := groupID
	for currentGroupID != "" {
		if visitedGroupIDs[currentGroupID] {
			return fmt.Errorf("%w: cycle at organization %q", errOrganizationGroupInvalidHierarchy, currentGroupID)
		}
		visitedGroupIDs[currentGroupID] = true
		currentGroupID = parentIDByGroupID[currentGroupID]
	}
	return nil
}
