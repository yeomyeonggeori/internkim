package localfleet

import (
	"context"
	"strings"
	"testing"
)

func TestWithoutMattermostRefusesAFleetScenarioBeforeBuildingAnything(t *testing.T) {
	service := Service{}
	for scenario := range service.scenarioPlanBuilders() {
		errorValue := service.RunScenario(context.Background(), nil, scenario, true, true)
		if errorValue == nil {
			t.Fatalf("--without-mattermost accepted the fleet scenario %s", scenario)
		}
		if !strings.Contains(errorValue.Error(), "run it without the flag") {
			t.Fatalf("%s was refused for the wrong reason: %v", scenario, errorValue)
		}
	}
}

func TestWithoutMattermostStillCarriesAVirtualSessionScenario(t *testing.T) {
	service := Service{}
	if _, isOurs := service.scenarioPlanBuilders()["dm_send_confirm_acceptance"]; isOurs {
		t.Fatal("a virtual session name has leaked into the fleet's own vocabulary")
	}
}
