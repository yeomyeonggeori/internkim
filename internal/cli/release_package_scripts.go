package cli

import (
	"path"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

// The maintainer scripts are one program for every format. The accounts and the
// directories are declared to systemd (sysusers.d and tmpfiles.d, shipped in the
// package) and the units are enabled through systemctl, so nothing below names a
// distribution's own account or unit tools. The one thing that differs between
// dpkg, rpm and pacman is how a script learns why it was run, and that is the
// prologue (RunsOnlyWhen) and the removal test each format gets.
//
// A postinst that cannot do its job fails, naming what it was doing. A box left with
// the package installed but no users, no state directory or no registered units would
// answer `dpkg -l` and run nothing, which is worse than a package that refuses.
//
// What it deliberately does not do is start anything. Every unit carries a
// ConditionPathExists over a file `internkim install` writes, so starting them before
// a company exists is a no-op systemd records as an unmet condition. The last lines
// say what to type next.

type packageScript string

const (
	postInstallScript packageScript = "postinst"
	preRemoveScript   packageScript = "prerm"
	postRemoveScript  packageScript = "postrm"
)

func maintainerScript(format linuxPackageFormat, script packageScript) string {
	body := map[packageScript]func(linuxPackageFormat) string{
		postInstallScript: postInstallBody,
		preRemoveScript:   preRemoveBody,
		postRemoveScript:  postRemoveBody,
	}[script]
	return "#!/bin/sh\nset -e\n\n" + body(format)
}

func postInstallBody(format linuxPackageFormat) string {
	return strings.Join([]string{
		format.RunsOnlyWhen("postinst"),
		`refuse() {`,
		`  echo "internkim: $1" >&2`,
		`  exit 1`,
		`}`,
		``,
		`systemd-sysusers ` + blueclaw.CompanyPackageSysusersPath + ` || refuse "systemd-sysusers could not create the service accounts declared in ` + blueclaw.CompanyPackageSysusersPath + `"`,
		`systemd-tmpfiles --create ` + blueclaw.CompanyPackageTmpfilesPath + ` || refuse "systemd-tmpfiles could not create the directories declared in ` + blueclaw.CompanyPackageTmpfilesPath + `"`,
		`command -v fc-cache >/dev/null 2>&1 && fc-cache -f ` + path.Dir(blueclaw.CompanyPackageDocumentFontPath) + ` >/dev/null 2>&1 || true`,
		hostSetupLines(blueclaw.LinuxCompanyHostLayout().PythonSetupCommands()),
		forgetTheDeviceUsersSync(``),
		``,
		`systemctl daemon-reload >/dev/null 2>&1 || refuse "systemd did not reload; this package supervises its services with systemd"`,
		`for unit in ` + unitFileNames() + `; do`,
		`  systemctl unmask "$unit" >/dev/null 2>&1 || true`,
		`  systemctl enable "$unit" >/dev/null 2>&1 || refuse "could not enable $unit"`,
		`done`,
		`systemctl restart ` + restartedUnitFileNames() + ` >/dev/null 2>&1 || true`,
		``,
		`if [ ! -e ` + blueclaw.CompanyHostCurrentPath + ` ]; then`,
		`  echo "internkim: installed. No company is configured yet, so every service but the box is idle."`,
		`  echo "internkim: an administrator on this network connects it from ` + blueclaw.CompanyPackageHomepage + `/settings/setup,"`,
		`  echo "internkim: or run 'internkim install <internkim-host.json>' to give it a company from a file."`,
		`fi`,
		`exit 0`,
		``,
	}, "\n")
}

func preRemoveBody(format linuxPackageFormat) string {
	return strings.Join([]string{
		`if ` + format.RemovalTest("prerm") + `; then`,
		`  systemctl stop ` + unitFileNames() + ` >/dev/null 2>&1 || true`,
		`  systemctl disable ` + unitFileNames() + ` >/dev/null 2>&1 || true`,
		forgetTheDeviceUsersSync(`  `),
		`fi`,
		`exit 0`,
		``,
	}, "\n")
}

// `apt remove` leaves both trees. `apt purge` additionally removes the configuration
// and still leaves the state: the identity seed signs a message under a person's own
// name and the agent key is the company's identity on the plane, neither is
// recoverable, and a customer who typed purge to reinstall would lose the company.
// The host's database and cache keep their own trees beside it, and the message names
// all three in the one command that deletes them, because the package that could have
// offered a command for it is gone by the time it prints.
// rpm and pacman have no purge; their own rule keeps an edited configuration file
// beside the removed one.
func postRemoveBody(format linuxPackageFormat) string {
	debianLayout := blueclaw.LinuxCompanyHostLayout()
	lines := []string{
		`if ` + format.RemovalTest("postrm") + `; then`,
		`  rm -rf ` + debianLayout.PythonRoot() + ` ` + debianLayout.DocumentVirtualEnvironmentPath(),
		`fi`,
		``,
	}
	if format.HasPurge {
		lines = append(lines,
			`if [ "$1" = purge ]; then`,
			`  rm -rf `+blueclaw.CompanyHostConfigurationRoot,
			`  if [ -d `+blueclaw.CompanyHostStateRoot+` ]; then`,
			`    echo "internkim: `+keptStateList()+` were kept. They hold this company's"`,
			`    echo "internkim: identity on the plane, the seed that signs messages as each person and the"`,
			`    echo "internkim: host's database, and none of it can be recovered. To delete them:"`,
			`    echo "internkim:   sudo rm -rf `+strings.Join(keptStatePaths(), " ")+`"`,
			`  fi`,
			`fi`,
			``)
	}
	return strings.Join(append(lines, `systemctl daemon-reload >/dev/null 2>&1 || true`, `exit 0`, ``), "\n")
}

// The first release's admind wrote the device's users sync onto the company host,
// where it fails every hour, and the package owns none of its three files.
func forgetTheDeviceUsersSync(indentation string) string {
	return indentation + `systemctl disable --now internkim-users-sync.timer internkim-users-sync.service >/dev/null 2>&1 || true` + "\n" +
		indentation + `rm -f ` + strings.Join(deviceUsersSyncPaths(), " ")
}

func deviceUsersSyncPaths() []string {
	return []string{blueclaw.InternKimUsersSyncScriptPath, blueclaw.InternKimUsersSyncServicePath, blueclaw.InternKimUsersSyncTimerPath}
}

func keptStatePaths() []string {
	return []string{blueclaw.CompanyHostStateRoot, blueclaw.CompanyHostDatabaseDataPath, blueclaw.CompanyHostCacheDataPath}
}

func keptStateList() string {
	paths := keptStatePaths()
	return strings.Join(paths[:len(paths)-1], ", ") + " and " + paths[len(paths)-1]
}

func restartedUnitFileNames() string {
	names := []string{}
	for _, unit := range blueclaw.CompanyPackageUnits() {
		if !unit.IsDataService() {
			names = append(names, unit.FileName())
		}
	}
	return strings.Join(names, " ")
}

func unitFileNames() string {
	names := []string{}
	for _, unit := range blueclaw.CompanyPackageUnits() {
		names = append(names, unit.FileName())
	}
	return strings.Join(names, " ")
}

func hostSetupLines(commands []blueclaw.HostSetupCommand) string {
	lines := []string{}
	for _, command := range commands {
		lines = append(lines, shellWords(command.Arguments)+` || refuse "could not `+command.Purpose+`; the output above names what failed"`)
	}
	return strings.Join(lines, "\n")
}

func shellWords(words []string) string {
	quoted := make([]string, len(words))
	for index, word := range words {
		quoted[index] = shellWord(word)
	}
	return strings.Join(quoted, " ")
}

func shellWord(word string) string {
	if word != "" && strings.Trim(word, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_./=:+-") == "" {
		return word
	}
	return "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
}
