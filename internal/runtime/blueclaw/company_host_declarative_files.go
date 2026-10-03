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
