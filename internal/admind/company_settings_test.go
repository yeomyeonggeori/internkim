package admind

import (
	"context"
	"strings"
	"testing"
)

func TestTheDayBoundaryFollowsTheZoneTheCompanyKeeps(t *testing.T) {
	service := newCompanyPolicyService(t, companyRowAnswering("America/Los_Angeles", "en"))

	if location := service.companyTimeLocation(context.Background()); location.String() != "America/Los_Angeles" {
		t.Fatalf("the device counts days where the company is, got %q", location)
	}
}

// Counting days in UTC is what made the agent a day early, so a company the
// device could not ask keeps the zone this product has always counted in.
func TestACompanyTheDeviceCouldNotAskCountsDaysInTheNamedDefault(t *testing.T) {
	service := newCompanyPolicyService(t, companyRowRefusing())

	if location := service.companyTimeLocation(context.Background()); location.String() != defaultCompanyTimeZone {
		t.Fatalf("an unasked company counts days in %s, got %q", defaultCompanyTimeZone, location)
	}
}

func TestATimeZoneTheDeviceCannotLoadIsRefusedInWords(t *testing.T) {
	_, errorValue := workspaceSettingsChange(workspaceSettings{TimeZone: "system"})

	if errorValue == nil {
		t.Fatal("a name that is not a time zone must be refused")
	}
	if !strings.Contains(errorValue.Error(), "Asia/Seoul") {
		t.Fatalf("the refusal must say what a time zone name looks like, got %q", errorValue)
	}
}
