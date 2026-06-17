package deployops

import (
	"fmt"
	"strings"
)

func SelectTargets(registry TargetRegistry, fleetIDs []string) ([]Target, error) {
	if len(fleetIDs) == 0 {
		return append([]Target(nil), registry.Targets...), nil
	}
	targetsByID := targetsByID(registry.Targets)
	targets := make([]Target, 0, len(fleetIDs))
	for _, fleetID := range fleetIDs {
		target, ok := targetsByID[strings.TrimSpace(fleetID)]
		if !ok {
			return nil, fmt.Errorf("unknown fleet target: %s", fleetID)
		}
		targets = append(targets, target)
	}
	return targets, nil
}

func targetsByID(targets []Target) map[string]Target {
	targetsByID := map[string]Target{}
	for _, target := range targets {
		targetsByID[target.ID] = target
	}
	return targetsByID
}
