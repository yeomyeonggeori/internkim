package admind

import (
	"errors"
	"fmt"
)

var errOrgchartGroupInvalidHierarchy = errors.New("invalid organization hierarchy")

func validateOrgchartGroupHierarchy(groups []orgGroupRecord) error {
	parentIDByGroupID := make(map[string]string, len(groups))
	for _, group := range groups {
		parentIDByGroupID[group.ID] = group.ParentID
	}
	for _, group := range groups {
		if group.ParentID != "" {
			if _, found := parentIDByGroupID[group.ParentID]; !found {
				return fmt.Errorf("%w: organization %q references unknown parent %q", errOrgchartGroupInvalidHierarchy, group.ID, group.ParentID)
			}
		}
		if errorValue := validateOrgchartGroupParentChain(group.ID, parentIDByGroupID); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func validateOrgchartGroupParentChain(groupID string, parentIDByGroupID map[string]string) error {
	visitedGroupIDs := map[string]bool{}
	currentGroupID := groupID
	for currentGroupID != "" {
		if visitedGroupIDs[currentGroupID] {
			return fmt.Errorf("%w: cycle at organization %q", errOrgchartGroupInvalidHierarchy, currentGroupID)
		}
		visitedGroupIDs[currentGroupID] = true
		currentGroupID = parentIDByGroupID[currentGroupID]
	}
	return nil
}
