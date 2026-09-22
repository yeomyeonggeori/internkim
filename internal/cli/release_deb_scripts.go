package cli

import (
	"fmt"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

// A postinst that cannot do its job fails, naming what it was doing. A box left with
// the package installed but no users, no state directory or no registered units would
// answer `dpkg -l` and run nothing, which is worse than a package that refuses.
//
// What it deliberately does not do is start anything. Every unit carries a
// ConditionPathExists over a file `internkim install` writes, so starting them before
// a company exists is a no-op systemd records as an unmet condition. The last lines
// say what to type next.
// companyHostStateRootMode is the one mode the state root has, as the shell
// spells it. Everything that creates that directory reads the same constant.
var companyHostStateRootMode = fmt.Sprintf("%04o", blueclaw.CompanyHostStateRootMode)

func debPostInstallScript() string {
	helperPath := blueclaw.CompanyHostPOSIXHelperPath
	return debScriptHeader() + strings.Join([]string{
		`[ "$1" = configure ] || exit 0`,
		``,
		`refuse() {`,
		`  echo "internkim: $1" >&2`,
		`  exit 1`,
		`}`,
		``,
		debSystemUser(blueclaw.BlueclawUser, blueclaw.BlueclawHomePath, "the agent runs as"),
		debSystemUser(blueclaw.RelayUserName, "", "the relay runs as"),
		debOwnedDirectory(blueclaw.BlueclawHomePath, blueclaw.BlueclawUser, "0750"),
		debOwnedDirectory(blueclaw.CompanyHostStateRoot, "root", companyHostStateRootMode),
		debOwnedDirectory(blueclaw.CompanyHostCompaniesRoot, "root", companyHostStateRootMode),
		debOwnedDirectory(blueclaw.CompanyHostConfigurationRoot, "root", "0755"),
		`# The helper is what lets the unprivileged agent act as the person who asked;`,
		`# an install that leaves it unprivileged has an agent that can read nothing.`,
		`chown root:root ` + helperPath + ` || refuse "could not take ownership of ` + helperPath + `"`,
		`chmod 4755 ` + helperPath + ` || refuse "could not make ` + helperPath + ` setuid"`,
		``,
		`systemctl daemon-reload >/dev/null 2>&1 || refuse "systemd did not reload; this package supervises its services with systemd"`,
		`for unit in ` + debUnitFileNames() + `; do`,
		`  deb-systemd-helper unmask "$unit" >/dev/null 2>&1 || true`,
		`  deb-systemd-helper enable "$unit" >/dev/null || refuse "could not enable $unit"`,
		`done`,
		`# Conditions are unmet until a company exists, so this starts nothing.`,
		`deb-systemd-invoke start ` + debUnitFileNames() + ` >/dev/null 2>&1 || true`,
		``,
		`if [ ! -e ` + blueclaw.CompanyHostCurrentPath + ` ]; then`,
		`  echo "internkim: installed. No company is configured yet, so every service is idle."`,
		`  echo "internkim: run 'internkim install <internkim-host.json>' to give this box a company."`,
		`fi`,
		`exit 0`,
		``,
	}, "\n")
}

func debSystemUser(name string, homePath string, role string) string {
	home := "--no-create-home"
	if homePath != "" {
		home = "--home " + homePath
	}
	return strings.Join([]string{
		`addgroup --system ` + name + ` >/dev/null 2>&1 || true`,
		`getent group ` + name + ` >/dev/null || refuse "could not create the ` + name + ` group ` + role + `"`,
		`adduser --system --ingroup ` + name + ` ` + home + ` --shell /usr/sbin/nologin ` + name + ` >/dev/null 2>&1 || true`,
		`getent passwd ` + name + ` >/dev/null || refuse "could not create the ` + name + ` user ` + role + `"`,
		``,
	}, "\n")
}

func debOwnedDirectory(path string, owner string, mode string) string {
	return `install -d -o ` + owner + ` -g ` + owner + ` -m ` + mode + ` ` + path +
		` || refuse "could not create ` + path + `"`
}

func debPreRemoveScript() string {
	return debScriptHeader() + strings.Join([]string{
		`if [ "$1" = remove ] || [ "$1" = deconfigure ]; then`,
		`  deb-systemd-invoke stop ` + debUnitFileNames() + ` >/dev/null 2>&1 || true`,
		`fi`,
		`exit 0`,
		``,
	}, "\n")
}

// `apt remove` leaves both trees. `apt purge` additionally removes the configuration
// and still leaves the state: the identity seed signs a message under a person's own
// name and the agent key is the company's identity on the plane, neither is
// recoverable, and a customer who typed purge to reinstall would lose the company.
// Deleting them is `internkim destroy --confirm`, which is the command that says so.
func debPostRemoveScript() string {
	return debScriptHeader() + strings.Join([]string{
		`if [ "$1" = remove ] || [ "$1" = purge ]; then`,
		`  for unit in ` + debUnitFileNames() + `; do`,
		`    deb-systemd-helper mask "$unit" >/dev/null 2>&1 || true`,
		`  done`,
		`fi`,
		``,
		`if [ "$1" = purge ]; then`,
		`  for unit in ` + debUnitFileNames() + `; do`,
		`    deb-systemd-helper purge "$unit" >/dev/null 2>&1 || true`,
		`    deb-systemd-helper unmask "$unit" >/dev/null 2>&1 || true`,
		`  done`,
		`  rm -rf ` + blueclaw.CompanyHostConfigurationRoot,
		`  if [ -d ` + blueclaw.CompanyHostStateRoot + ` ]; then`,
		`    echo "internkim: ` + blueclaw.CompanyHostStateRoot + ` was kept. It holds this company's identity on"`,
		`    echo "internkim: the plane and the seed that signs messages as each person, and neither"`,
		`    echo "internkim: can be recovered. 'internkim destroy --confirm' is what deletes it."`,
		`  fi`,
		`fi`,
		``,
		`systemctl daemon-reload >/dev/null 2>&1 || true`,
		`exit 0`,
		``,
	}, "\n")
}

func debScriptHeader() string {
	return "#!/bin/sh\nset -e\n\n"
}

func debUnitFileNames() string {
	names := []string{}
	for _, unit := range blueclaw.CompanyPackageUnits() {
		names = append(names, unit.FileName())
	}
	return strings.Join(names, " ")
}
