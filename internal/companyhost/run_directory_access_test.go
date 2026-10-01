package companyhost

import (
	"path"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

type posixAccount struct {
	name   string
	groups []string
}

type posixEntry struct {
	owner string
	group string
	mode  uint32
}

const (
	setGroupIDBit = 0o2000
	readBit       = 0o4
	writeBit      = 0o2
	searchBit     = 0o1
)

func (entry posixEntry) grants(account posixAccount, permission uint32) bool {
	switch {
	case account.name == entry.owner:
		return entry.mode>>6&permission == permission
	case slices.Contains(account.groups, entry.group):
		return entry.mode>>3&permission == permission
	default:
		return entry.mode&permission == permission
	}
}

// runDirectory is what a host's run directory holds once the prepare script
// has run and every daemon has opened its socket: the directories as the
// script creates them, and each socket as the daemon that creates it leaves it.
type runDirectory struct {
	layout  blueclaw.CompanyHostLayout
	entries map[string]posixEntry
}

func preparedRunDirectory(t *testing.T, layout blueclaw.CompanyHostLayout) runDirectory {
	t.Helper()
	directory := runDirectory{layout: layout, entries: directoriesThePrepareScriptCreates(t, layout)}
	agentAccount := serviceAccount(t, layout, blueclaw.BlueclawServiceName)
	acpSocketPath := commandArgument(t, layout, blueclaw.BlueclawServiceName, "-acp-socket")
	directory.entries[acpSocketPath] = posixEntry{
		owner: agentAccount,
		group: directory.groupOfAFileCreatedIn(path.Dir(acpSocketPath), agentAccount),
		mode:  blueclaw.BlueclawACPSocketMode,
	}
	directory.entries[commandArgument(t, layout, blueclaw.AdmindServiceName, "-listen-socket")] = posixEntry{
		owner: blueclaw.RelayUserName,
		group: blueclaw.RelayUserName,
		mode:  blueclaw.AdmindSocketMode,
	}
	directory.entries[commandArgument(t, layout, blueclaw.CapabilitydServiceName, "--socket")] = posixEntry{
		owner: "root",
		group: blueclaw.BlueclawUser,
		mode:  blueclaw.CapabilitySocketMode,
	}
	return directory
}

// Linux gives a new file its creator's group unless the directory is setgid;
// BSD always gives it the directory's. The Linux rule is the narrower one, so
// it is the one modelled.
func (directory runDirectory) groupOfAFileCreatedIn(directoryPath string, creator string) string {
	parent, isKnown := directory.entries[directoryPath]
	if isKnown && parent.mode&setGroupIDBit != 0 {
		return parent.group
	}
	return creator
}

func (directory runDirectory) canPassThroughTo(account posixAccount, entryPath string) bool {
	for parentPath := path.Dir(entryPath); directory.holds(parentPath); parentPath = path.Dir(parentPath) {
		parent, isKnown := directory.entries[parentPath]
		if !isKnown || !parent.grants(account, searchBit) {
			return false
		}
	}
	return true
}

func (directory runDirectory) holds(entryPath string) bool {
	return entryPath == directory.layout.RunPath || strings.HasPrefix(entryPath, directory.layout.RunPath+"/")
}

func (directory runDirectory) canConnectTo(account posixAccount, socketPath string) bool {
	socket, isKnown := directory.entries[socketPath]
	return isKnown && directory.canPassThroughTo(account, socketPath) && socket.grants(account, readBit|writeBit)
}

func (directory runDirectory) canOpenDirectory(account posixAccount, directoryPath string) bool {
	entry, isKnown := directory.entries[directoryPath]
	return isKnown && directory.canPassThroughTo(account, directoryPath) && entry.grants(account, searchBit)
}

func directoriesThePrepareScriptCreates(t *testing.T, layout blueclaw.CompanyHostLayout) map[string]posixEntry {
	t.Helper()
	entries := map[string]posixEntry{}
	for _, line := range strings.Split(blueclaw.CompanyHostPrepareScriptFor(layout), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 9 || fields[0] != "install" || fields[1] != "-d" || fields[2] != "-o" || fields[4] != "-g" || fields[6] != "-m" {
			continue
		}
		mode, errorValue := strconv.ParseUint(fields[7], 8, 32)
		if errorValue != nil {
			t.Fatalf("the prepare script creates %s with mode %q, which is not octal", fields[8], fields[7])
		}
		entries[fields[8]] = posixEntry{owner: fields[3], group: fields[5], mode: uint32(mode)}
	}
	if _, isCreated := entries[layout.RunPath]; !isCreated {
		t.Fatalf("the prepare script does not create %s, so this model reads the wrong script", layout.RunPath)
	}
	return entries
}

func serviceAccount(t *testing.T, layout blueclaw.CompanyHostLayout, serviceName string) string {
	t.Helper()
	service, isFound := blueclaw.CompanyHostServiceNamed(layout, serviceName)
	if !isFound {
		t.Fatalf("the bundle has no %s", serviceName)
	}
	if service.Account == "" {
		return "root"
	}
	return service.Account
}

func commandArgument(t *testing.T, layout blueclaw.CompanyHostLayout, serviceName string, flag string) string {
	t.Helper()
	service, _ := blueclaw.CompanyHostServiceNamed(layout, serviceName)
	index := slices.Index(service.Command, flag)
	if index < 0 || index+1 >= len(service.Command) {
		t.Fatalf("%s is started without %s, so where it listens is not the layout's", serviceName, flag)
	}
	return service.Command[index+1]
}

func relaySetting(t *testing.T, layout blueclaw.CompanyHostLayout, name string) string {
	t.Helper()
	for _, entry := range relayEnvironment(layout, Connection{}) {
		if entry.Name == name {
			return entry.Value
		}
	}
	t.Fatalf("the relay's environment does not name %s, so it falls back to a Linux path on every machine", name)
	return ""
}

// The relay runs as its own account in its own group, and the unit gives it no
// other: the blueclaw group reads the keys, so it is never the relay's.
func relayAccount(t *testing.T) posixAccount {
	t.Helper()
	groups := []string{blueclaw.RelayUserName}
	for _, unit := range blueclaw.CompanyPackageUnits() {
		if unit.Name != blueclaw.RelayServiceName {
			continue
		}
		for _, line := range strings.Split(unit.Contents, "\n") {
			if value, isSupplementary := strings.CutPrefix(line, "SupplementaryGroups="); isSupplementary {
				groups = append(groups, strings.Fields(value)...)
			}
		}
	}
	return posixAccount{name: blueclaw.RelayUserName, groups: groups}
}

func taskUserAccount() posixAccount {
	return posixAccount{name: "bc_person_sample", groups: []string{"bc_person_sample", "bc_shared", "bc_circle_sample"}}
}

func companyHostLayouts() map[string]blueclaw.CompanyHostLayout {
	return map[string]blueclaw.CompanyHostLayout{
		"linux": blueclaw.LinuxCompanyHostLayout(),
		"macOS": blueclaw.MacCompanyHostLayout("/opt/homebrew"),
	}
}

func TestTheRelayIsPointedAtTheSocketsTheDaemonsListenOn(t *testing.T) {
	for name, layout := range companyHostLayouts() {
		if relayPath, daemonPath := relaySetting(t, layout, "ADMIND_SOCKET_PATH"), commandArgument(t, layout, blueclaw.AdmindServiceName, "-listen-socket"); relayPath != daemonPath {
			t.Errorf("%s: the relay calls admind at %s and admind listens at %s", name, relayPath, daemonPath)
		}
		if relayPath, daemonPath := relaySetting(t, layout, "BLUECLAW_ACP_SOCKET_PATH"), commandArgument(t, layout, blueclaw.BlueclawServiceName, "-acp-socket"); relayPath != daemonPath {
			t.Errorf("%s: the relay opens sessions at %s and blueclaw serves them at %s", name, relayPath, daemonPath)
		}
	}
}

func TestTheRelayReachesTheTwoSocketsItOpensAndNothingElse(t *testing.T) {
	relay := relayAccount(t)
	for name, layout := range companyHostLayouts() {
		directory := preparedRunDirectory(t, layout)
		for _, socketPath := range []string{relaySetting(t, layout, "ADMIND_SOCKET_PATH"), relaySetting(t, layout, "BLUECLAW_ACP_SOCKET_PATH")} {
			if !directory.canConnectTo(relay, socketPath) {
				t.Errorf("%s: %s cannot connect to %s (%+v), so no message a person sends reaches the agent", name, relay.name, socketPath, directory.entries)
			}
		}
		if directory.canConnectTo(relay, layout.CapabilitySocketPath()) {
			t.Errorf("%s: %s can call the capability daemon at %s, which holds the provider keys", name, relay.name, layout.CapabilitySocketPath())
		}
		if directory.canOpenDirectory(relay, layout.RunSecretsPath()) {
			t.Errorf("%s: %s can open %s", name, relay.name, layout.RunSecretsPath())
		}
		if directory.entries[layout.RunPath].grants(relay, readBit) {
			t.Errorf("%s: %s can list %s; passing through is all it needs", name, relay.name, layout.RunPath)
		}
	}
}

func TestATaskUserReachesNoSocketInTheRunDirectory(t *testing.T) {
	taskUser := taskUserAccount()
	for name, layout := range companyHostLayouts() {
		directory := preparedRunDirectory(t, layout)
		for entryPath, entry := range directory.entries {
			if !directory.holds(entryPath) {
				continue
			}
			if directory.canConnectTo(taskUser, entryPath) {
				t.Errorf("%s: a task user can connect to %s", name, entryPath)
			}
			if entryPath != layout.RunPath && entry.mode&0o7 != 0 {
				t.Errorf("%s: %s is mode %04o and grants others something; the run directory lets them pass through, so its entries must not", name, entryPath, entry.mode)
			}
		}
		if directory.entries[layout.RunPath].grants(taskUser, readBit) {
			t.Errorf("%s: a task user can list %s", name, layout.RunPath)
		}
	}
}

// The run directory lets others pass through, so a file the script writes
// there with the default umask could be read by anyone who knows its name.
func TestThePrepareScriptWritesUnderAPrivateUmask(t *testing.T) {
	for name, layout := range companyHostLayouts() {
		script := blueclaw.CompanyHostPrepareScriptFor(layout)
		umaskAt := strings.Index(script, "\numask 077\n")
		if umaskAt < 0 || umaskAt > strings.Index(script, layout.RunPath) {
			t.Errorf("%s: the prepare script does not set umask 077 before it first touches %s", name, layout.RunPath)
		}
	}
}
