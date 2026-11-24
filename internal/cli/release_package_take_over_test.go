package cli

import (
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func TestTheTakeOverIsShippedAndNeitherEnabledNorRestartedByThePackage(t *testing.T) {
	takeOver := blueclaw.CompanyPackageUnit{Name: blueclaw.CompanyHostTakeOverServiceName}.FileName()
	if !strings.Contains(unitFileNames(), takeOver) {
		t.Fatalf("the package ships no %s, so a standby has nothing to take its company over with", takeOver)
	}
	for name, names := range map[string]string{"enabled": enabledUnitFileNames(), "restarted": companyUnitFileNames()} {
		if strings.Contains(names, takeOver) {
			t.Errorf("%s is %s by the package, so installing or upgrading it would start a take-over nobody asked for", takeOver, name)
		}
	}
}
