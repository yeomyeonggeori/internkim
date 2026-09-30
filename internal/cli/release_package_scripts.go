package cli

import (
	"fmt"
	"path"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

// The maintainer scripts are one program in three dialects. What the scripts do
// is written once, in terms of a few shell functions; what differs between dpkg,
// rpm and pacman is how a script learns why it was run and which tool enables a
// unit, and that is the prologue each format gets. Nothing below the prologue
// names a format.
//
// A postinst that cannot do its job fails, naming what it was doing. A box left with
// the package installed but no users, no state directory or no registered units would
// answer `dpkg -l` and run nothing, which is worse than a package that refuses.
//
// What it deliberately does not do is start anything. Every unit carries a
// ConditionPathExists over a file `internkim install` writes, so starting them before
// a company exists is a no-op systemd records as an unmet condition. The last lines
// say what to type next.
//
// companyHostStateRootMode is the one mode the state root has, as the shell
// spells it. Everything that creates that directory reads the same constant.
var companyHostStateRootMode = fmt.Sprintf("%04o", blueclaw.CompanyHostStateRootMode)

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
	return "#!/bin/sh\nset -e\n\n" + scriptHelpers(format) + "\n" + body(format)
}

func postInstallBody(format linuxPackageFormat) string {
	helperPath := blueclaw.CompanyHostPOSIXHelperPath
	return strings.Join([]string{
		format.RunsOnlyWhen("postinst"),
		`refuse() {`,
		`  echo "internkim: $1" >&2`,
		`  exit 1`,
		`}`,
		``,
		systemUser(blueclaw.BlueclawUser, blueclaw.BlueclawHomePath, "the agent runs as"),
		systemUser(blueclaw.RelayUserName, "", "the relay runs as"),
		systemUser(blueclaw.CompanyHostDatabaseUser, "", "the database runs as"),
		systemUser(blueclaw.CompanyHostCacheUser, "", "the cache runs as"),
		ownedDirectory(blueclaw.BlueclawHomePath, blueclaw.BlueclawUser, "0750"),
		ownedDirectory(blueclaw.CompanyHostStateRoot, "root", companyHostStateRootMode),
		ownedDirectory(blueclaw.CompanyHostCompaniesRoot, "root", companyHostStateRootMode),
		ownedDirectory(blueclaw.CompanyHostConfigurationRoot, "root", "0755"),
		`chown root:root ` + helperPath + ` || refuse "could not take ownership of ` + helperPath + `"`,
		`chmod 4755 ` + helperPath + ` || refuse "could not make ` + helperPath + ` setuid"`,
		`command -v fc-cache >/dev/null 2>&1 && fc-cache -f ` + path.Dir(blueclaw.CompanyPackageDocumentFontPath) + ` >/dev/null 2>&1 || true`,
		``,
		`systemctl daemon-reload >/dev/null 2>&1 || refuse "systemd did not reload; this package supervises its services with systemd"`,
		`for unit in ` + unitFileNames() + `; do`,
		`  enable_unit "$unit" || refuse "could not enable $unit"`,
		`done`,
		`restart_units ` + restartedUnitFileNames(),
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
		`  stop_units ` + unitFileNames(),
		`  for unit in ` + unitFileNames() + `; do`,
		`    disable_unit "$unit"`,
		`  done`,
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
// rpm and pacman have no purge; their own rule keeps an edited configuration file
// beside the removed one.
func postRemoveBody(format linuxPackageFormat) string {
	lines := []string{
		`if ` + format.RemovalTest("postrm") + `; then`,
		`  for unit in ` + unitFileNames() + `; do`,
		`    retire_unit "$unit"`,
		`  done`,
		`  rm -rf ` + blueclaw.CompanyPackageInterpreterPath + ` ` + blueclaw.CompanyPackageDocumentVenvPath,
		`fi`,
		``,
	}
	if format.HasPurge {
		lines = append(lines,
			`if [ "$1" = purge ]; then`,
			`  for unit in `+unitFileNames()+`; do`,
			`    forget_unit "$unit"`,
			`  done`,
			`  rm -rf `+blueclaw.CompanyHostConfigurationRoot,
			`  if [ -d `+blueclaw.CompanyHostStateRoot+` ]; then`,
			`    echo "internkim: `+blueclaw.CompanyHostStateRoot+` was kept. It holds this company's identity on"`,
			`    echo "internkim: the plane and the seed that signs messages as each person, and neither"`,
			`    echo "internkim: can be recovered. 'internkim destroy --confirm' is what deletes it."`,
			`  fi`,
			`fi`,
			``)
	}
	return strings.Join(append(lines, `systemctl daemon-reload >/dev/null 2>&1 || true`, `exit 0`, ``), "\n")
}

func systemUser(name string, homePath string, role string) string {
	return strings.Join([]string{
		`create_system_user ` + name + ` "` + homePath + `" || refuse "could not create the ` + name + ` user ` + role + `"`,
		``,
	}, "\n")
}

func ownedDirectory(path string, owner string, mode string) string {
	return `install -d -o ` + owner + ` -g ` + owner + ` -m ` + mode + ` ` + path +
		` || refuse "could not create ` + path + `"`
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

// scriptHelpers is the part that differs by format: shadow-utils accounts are the
// same everywhere, and the unit verbs are the format's own. Debian's helpers keep
// dpkg's record of which units an administrator enabled; the others talk to
// systemctl, which is what their own macros expand to.
func scriptHelpers(format linuxPackageFormat) string {
	return accountHelpers + format.UnitHelpers
}

const accountHelpers = `create_system_user() {
  name="$1"
  home="$2"
  getent group "$name" >/dev/null || groupadd --system "$name" >/dev/null 2>&1 || true
  getent group "$name" >/dev/null || return 1
  getent passwd "$name" >/dev/null && return 0
  shell="$(command -v nologin || true)"
  [ -n "$shell" ] || shell=/bin/false
  useradd --system --gid "$name" --home-dir "${home:-/nonexistent}" --no-create-home --shell "$shell" "$name" >/dev/null 2>&1 || true
  getent passwd "$name" >/dev/null
}
`

// Every format restarts rather than starts: on a box with no company every unit's
// condition is unmet and this starts nothing either way, and on an upgrade it is what
// moves the running processes onto the binaries that were just unpacked. start leaves
// the old process serving while the package database, the file's mtime and the package
// manager all report the new version.
const debianUnitHelpers = `enable_unit() {
  deb-systemd-helper unmask "$1" >/dev/null 2>&1 || true
  deb-systemd-helper enable "$1" >/dev/null
}

restart_units() {
  deb-systemd-invoke restart "$@" >/dev/null 2>&1 || true
}

stop_units() {
  deb-systemd-invoke stop "$@" >/dev/null 2>&1 || true
}

disable_unit() {
  :
}

retire_unit() {
  deb-systemd-helper mask "$1" >/dev/null 2>&1 || true
}

forget_unit() {
  deb-systemd-helper purge "$1" >/dev/null 2>&1 || true
  deb-systemd-helper unmask "$1" >/dev/null 2>&1 || true
}
`

const systemctlUnitHelpers = `enable_unit() {
  systemctl unmask "$1" >/dev/null 2>&1 || true
  systemctl enable "$1" >/dev/null 2>&1
}

restart_units() {
  systemctl restart "$@" >/dev/null 2>&1 || true
}

stop_units() {
  systemctl stop "$@" >/dev/null 2>&1 || true
}

disable_unit() {
  systemctl disable "$1" >/dev/null 2>&1 || true
}

retire_unit() {
  :
}

forget_unit() {
  :
}
`
