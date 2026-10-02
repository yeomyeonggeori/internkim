package blueclaw

import (
	"fmt"
	"strings"
)

const (
	CompanyPackageSysusersPath = "/usr/lib/sysusers.d/internkim.conf"
	CompanyPackageTmpfilesPath = "/usr/lib/tmpfiles.d/internkim.conf"

	companyHostNoLoginShell = "/usr/sbin/nologin"
)

// CompanyHostServiceAccount is one unprivileged account the bundle runs a
// service as. The relay has its own because it outlives the agent, and the
// database and the cache have theirs because each owns its data directory.
// HomePath is empty for an account with no home.
type CompanyHostServiceAccount struct {
	Name        string
	Description string
	HomePath    string
}

// CompanyHostServiceAccounts is every account, for sysusers on Linux and for
// the directory service on a Mac.
func CompanyHostServiceAccounts(layout CompanyHostLayout) []CompanyHostServiceAccount {
	return []CompanyHostServiceAccount{
		{BlueclawUser, "internkim agent", layout.AgentHomePath},
		{RelayUserName, "internkim relay", ""},
		{CompanyHostDatabaseUser, "internkim database", ""},
		{CompanyHostCacheUser, "internkim cache", ""},
	}
}

func CompanyHostSysusersFile() string {
	lines := []string{}
	for _, account := range CompanyHostServiceAccounts(LinuxCompanyHostLayout()) {
		homePath := account.HomePath
		if homePath == "" {
			homePath = "-"
		}
		lines = append(lines, fmt.Sprintf("u %s - %q %s %s", account.Name, account.Description, homePath, companyHostNoLoginShell))
	}
	return strings.Join(append(lines, ""), "\n")
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
