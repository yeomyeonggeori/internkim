package blueclaw

import (
	"fmt"
	"strings"
)

const (
	CompanyPackageSysusersPath    = "/usr/lib/sysusers.d/internkim.conf"
	CompanyPackageTmpfilesPath    = "/usr/lib/tmpfiles.d/internkim.conf"
	CompanyPackagePolkitRulesPath = "/usr/share/polkit-1/rules.d/50-internkim-administrators.rules"
	CompanyHostAdministratorGroup = "internkim-admin"

	companyHostNoLoginShell = "/usr/sbin/nologin"
)

type companyHostServiceAccount struct {
	Name        string
	Description string
	HomePath    string
}

func companyHostServiceAccounts() []companyHostServiceAccount {
	return []companyHostServiceAccount{
		{BlueclawUser, "the agent runs as", BlueclawHomePath},
		{RelayUserName, "the relay runs as", "-"},
		{CompanyHostDatabaseUser, "the database runs as", "-"},
		{CompanyHostCacheUser, "the cache runs as", "-"},
	}
}

func CompanyHostSysusersFile() string {
	lines := []string{}
	for _, account := range companyHostServiceAccounts() {
		lines = append(lines, fmt.Sprintf("u %s - %q %s %s", account.Name, account.Description, account.HomePath, companyHostNoLoginShell))
	}
	lines = append(lines, fmt.Sprintf("g %s -", CompanyHostAdministratorGroup))
	return strings.Join(append(lines, ""), "\n")
}

func CompanyHostPolkitRules() string {
	unitNames := []string{}
	for _, unit := range CompanyPackageUnits() {
		unitNames = append(unitNames, fmt.Sprintf("%q", unit.FileName()))
	}
	return strings.Join([]string{
		"polkit.addRule(function (action, subject) {",
		"  var units = [" + strings.Join(unitNames, ", ") + "];",
		`  var verbs = ["start", "stop", "restart", "try-restart"];`,
		`  if (action.id === "org.freedesktop.systemd1.manage-units" &&`,
		fmt.Sprintf("      subject.isInGroup(%q) &&", CompanyHostAdministratorGroup),
		`      units.indexOf(action.lookup("unit")) >= 0 &&`,
		`      verbs.indexOf(action.lookup("verb")) >= 0) {`,
		"    return polkit.Result.YES;",
		"  }",
		"});",
		"",
	}, "\n")
}

func CompanyHostTmpfilesFile() string {
	stateMode := fmt.Sprintf("%04o", CompanyHostStateRootMode)
	lines := []string{
		fmt.Sprintf("d %s 0750 %s %s -", BlueclawHomePath, BlueclawUser, BlueclawUser),
		fmt.Sprintf("d %s %s root root -", CompanyHostStateRoot, stateMode),
		fmt.Sprintf("d %s %s root root -", CompanyHostCompaniesRoot, stateMode),
		fmt.Sprintf("d %s 0755 root root -", CompanyHostConfigurationRoot),
		fmt.Sprintf("z %s 4755 root root -", LinuxCompanyHostLayout().POSIXHelperPath()),
		"",
	}
	return strings.Join(lines, "\n")
}
