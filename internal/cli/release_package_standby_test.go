package cli

import (
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func theTwoBranchesOf(t *testing.T, script string) (string, string) {
	t.Helper()
	withCompany, withoutCompany, isSplit := strings.Cut(script, "\nelse\n")
	if !isSplit {
		t.Fatalf("the postinst does not tell a box with a company from one without:\n%s", script)
	}
	return withCompany, withoutCompany
}

func TestAnUpgradeOnABoxWithoutACompanyStartsNoneOfTheCompanysUnits(t *testing.T) {
	_, withoutCompany := theTwoBranchesOf(t, bringTheCompanyBackOnThisRelease(blueclaw.CompanyHostCurrentPath, refreshCommand()))

	for _, unit := range blueclaw.CompanyPackageUnits() {
		if unit.Name == blueclaw.BoxServiceName || unit.IsDataService() {
			continue
		}
		for _, line := range strings.Split(withoutCompany, "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "systemctl" && fields[1] == "restart" && strings.Contains(line, unit.FileName()) {
				t.Fatalf("an upgrade on a box without a company starts %s, which a standby stopped on purpose:\n%s", unit.FileName(), withoutCompany)
			}
		}
		if !strings.Contains(withoutCompany, "systemctl try-restart") || !strings.Contains(withoutCompany, unit.FileName()) {
			t.Fatalf("an upgrade on a box without a company leaves %s on the old binaries when it is running:\n%s", unit.FileName(), withoutCompany)
		}
	}
	if !strings.Contains(withoutCompany, "systemctl restart "+boxUnitFileName()) {
		t.Fatalf("an upgrade on a box without a company does not restart the box:\n%s", withoutCompany)
	}
}

func TestAnUpgradeOnABoxWithACompanyBringsItBackAsBefore(t *testing.T) {
	withCompany, _ := theTwoBranchesOf(t, bringTheCompanyBackOnThisRelease(blueclaw.CompanyHostCurrentPath, refreshCommand()))

	want := strings.Join([]string{
		`if [ -e ` + blueclaw.CompanyHostCurrentPath + ` ]; then`,
		`  systemctl restart ` + boxUnitFileName() + ` >/dev/null 2>&1 || true`,
		`  ` + refreshCommand() + ` || refuse "this release is installed and the company's server did not come back on it; the lines above name what is silent. Once that is fixed, run 'sudo ` + refreshCommand() + `'"`,
	}, "\n")
	if withCompany != want {
		t.Fatalf("the branch for a box with a company changed:\n%s", withCompany)
	}
}
