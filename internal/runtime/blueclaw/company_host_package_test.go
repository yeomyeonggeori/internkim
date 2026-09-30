package blueclaw

import (
	"fmt"
	"strings"
	"testing"
)

func TestEveryBundledUnitWaitsForSomethingInstallWrites(t *testing.T) {
	for _, unit := range CompanyHostSystemdUnits(LinuxCompanyHostLayout()) {
		if !strings.Contains(unit.Contents, "ConditionPathExists=") {
			t.Fatalf("%s starts as soon as it is enabled; on a box that has the package and no "+
				"company it would restart into the same failure forever", unit.FileName())
		}
	}
}

func TestTheBoxUnitIsThePackagedUnitThatRunsBeforeThereIsACompany(t *testing.T) {
	bundled := map[string]bool{}
	for _, unit := range CompanyHostSystemdUnits(LinuxCompanyHostLayout()) {
		bundled[unit.Name] = true
	}
	for _, unit := range CompanyPackageUnits() {
		if bundled[unit.Name] {
			continue
		}
		if unit.Name != BoxServiceName {
			t.Fatalf("%s is packaged outside the bundle and is not the box unit", unit.FileName())
		}
		if strings.Contains(unit.Contents, "ConditionPathExists=") {
			t.Fatalf("%s waits for a company, and it is what a company is claimed through", unit.FileName())
		}
		bundled[unit.Name] = true
	}
	if !bundled[BoxServiceName] {
		t.Fatal("the package ships no box unit, so a box nobody configured never announces itself")
	}
	if _, isBundled := CompanyHostServiceNamed(LinuxCompanyHostLayout(), BoxServiceName); isBundled {
		t.Fatal("the box unit is in the bundle, so the install it runs would restart it halfway through")
	}
}

// dpkg may not write /usr/local, and the device path owns what is there. A packaged
// unit naming that directory would start the device's binary or nothing at all.
func TestNoPackagedUnitReachesIntoUsrLocal(t *testing.T) {
	for _, unit := range CompanyPackageUnits() {
		if strings.Contains(unit.Contents, "/usr/local/") {
			t.Fatalf("%s names /usr/local, which this package does not install into", unit.FileName())
		}
	}
	if strings.Contains(CompanyHostPrepareScript(), "/usr/local/") {
		t.Fatal("the preparation script names /usr/local, which this package does not install into")
	}
}

func TestEveryPackagedUnitStartsAProgramFromThePackage(t *testing.T) {
	started := 0
	for _, unit := range CompanyPackageUnits() {
		for _, line := range strings.Split(unit.Contents, "\n") {
			if !strings.HasPrefix(line, "ExecStart=") {
				continue
			}
			program := strings.Fields(strings.TrimPrefix(line, "ExecStart="))[0]
			if !strings.HasPrefix(program, CompanyPackageBinaryRoot+"/") && !strings.HasPrefix(program, CompanyPackageHelperRoot+"/") {
				t.Fatalf("%s starts %s, which the package does not install", unit.FileName(), program)
			}
			started++
		}
	}
	if started < len(CompanyPackageUnits()) {
		t.Fatalf("read %d ExecStart lines from %d units, so this test is reading the wrong place",
			started, len(CompanyPackageUnits()))
	}
}

// The object store is being replaced, so the package must not depend on which server
// answers. The endpoint is written once, in the file an operator edits.
func TestTheObjectStoreIsNamedOnlyWhereAnOperatorCanChangeIt(t *testing.T) {
	for _, unit := range CompanyPackageUnits() {
		if strings.Contains(unit.Contents, "BUZZ_S3_") {
			t.Fatalf("%s names the object store; swapping the server would mean editing a unit dpkg owns", unit.FileName())
		}
		if strings.Contains(strings.ToLower(unit.Contents), "minio") {
			t.Fatalf("%s names MinIO, which is archived upstream and being replaced", unit.FileName())
		}
	}
	settings := CompanyHostSettingsFile()
	for _, setting := range []string{"BUZZ_S3_ENDPOINT=", "BUZZ_S3_BUCKET=", "BUZZ_S3_REGION="} {
		if !strings.Contains(settings, setting) {
			t.Fatalf("%s is not in the settings file, so nothing on the box says where media goes", setting)
		}
	}
}

// The relay is the one service whose device unit and packaged unit are the same
// unit. They differ in two values of one declaration: the binary path dpkg is
// allowed to write, and the state directory an unprivileged account can reach on
// a host whose state root is 0700. Anything else differing means the renderer
// grew a second definition.
func TestThePackagedRelayUnitIsTheDeviceUnitWithThePackagesTwoValues(t *testing.T) {
	packaged := ""
	for _, unit := range CompanyPackageUnits() {
		if unit.Name == RelayServiceName {
			packaged = unit.Contents
		}
	}
	if packaged == "" {
		t.Fatal("the package installs no relay unit, and the relay is what keeps the screen alive when the agent is down")
	}
	expected := strings.ReplaceAll(RelayServiceUnit(), RelayBinaryPath, CompanyPackageBinaryPath(RelayName))
	expected = strings.ReplaceAll(expected,
		RelayStateDirectoryPath(RelayStateDirectoryName), RelayStateDirectoryPath(CompanyHostRelayStateDirectoryName))
	expected = strings.ReplaceAll(expected,
		"StateDirectory="+RelayStateDirectoryName, "StateDirectory="+CompanyHostRelayStateDirectoryName)
	if packaged != expected {
		t.Fatalf("the packaged relay unit differs from the device one by more than those two values:\n%s", packaged)
	}
}

// Every unit reads the operator's file after the company's, so an edited setting wins
// over a rendered default rather than being silently overridden by it.
func TestTheOperatorSettingsFileIsReadLast(t *testing.T) {
	for _, unit := range CompanyPackageUnits() {
		settingsPosition := strings.Index(unit.Contents, "EnvironmentFile=-"+CompanyHostSettingsPath)
		if settingsPosition < 0 {
			continue
		}
		for _, line := range strings.Split(unit.Contents[settingsPosition:], "\n")[1:] {
			if strings.HasPrefix(line, "Environment=") || strings.HasPrefix(line, "EnvironmentFile=") {
				t.Fatalf("%s sets %q after reading %s, so editing that file changes nothing",
					unit.FileName(), line, CompanyHostSettingsPath)
			}
		}
	}
}

// The preparation script is the half of host/entrypoint.sh systemd does not take over.
// It must refuse rather than leave a box whose agent has no identity to act with.
func TestThePreparationScriptRefusesWithoutACompany(t *testing.T) {
	script := CompanyHostPrepareScript()
	if !strings.Contains(script, "set -e") {
		t.Fatal("the preparation script continues past a failed step")
	}
	if !strings.Contains(script, "exit 1") || !strings.Contains(script, CompanyHostAgentKeyPath) {
		t.Fatalf("the preparation script does not refuse when %s is missing", CompanyHostAgentKeyPath)
	}
}

// The package is where the object store stopped being MinIO. The gateway is a pinned
// download like the two browsers, its unit is one entry beside the others, and the
// relay waits for it — which is the whole cost of the swap on this path.
func TestThePackageCarriesTheMediaStoreAndTheRelayWaitsForIt(t *testing.T) {
	media := ""
	relay := ""
	for _, unit := range CompanyPackageUnits() {
		switch unit.Name {
		case BuzzMediaServiceName:
			media = unit.Contents
		case BuzzRelayServiceName:
			relay = unit.Contents
		}
	}
	if media == "" {
		t.Fatalf("the package installs no %s unit, so the messenger has nowhere to put attachments", BuzzMediaServiceName)
	}
	if !strings.Contains(media, CompanyPackageBinaryPath(BuzzMediaProgramName)) {
		t.Fatalf("the %s unit does not start %s", BuzzMediaServiceName, CompanyPackageBinaryPath(BuzzMediaProgramName))
	}
	if strings.Contains(media, "--versioning-dir") {
		t.Fatal("the packaged media unit enables versioning; read TestBuzzMediaUnitDoesNotEnableVersioning before adding it")
	}
	if !strings.Contains(relay, "After=") || !strings.Contains(relay, BuzzMediaServiceName+".service") {
		t.Fatalf("%s does not wait for %s, so the relay can come up with no store to write to",
			BuzzRelayServiceName, BuzzMediaServiceName)
	}
}

// The gateway is served the bucket as a directory, so something has to make it. On the
// device that is the provisioning step; here it is dpkg.
func TestTheBucketIsADirectoryUnderTheGatewayRoot(t *testing.T) {
	if !strings.HasPrefix(CompanyHostMediaBucketPath, CompanyHostMediaRootPath+"/") {
		t.Fatalf("the packaged bucket %s is not under the root the packaged gateway serves, %s; "+
			"dpkg would create a directory the gateway never opens",
			CompanyHostMediaBucketPath, CompanyHostMediaRootPath)
	}
	if strings.HasPrefix(CompanyHostMediaBucketPath, BuzzMediaRootPath+"/") {
		t.Fatalf("the packaged bucket is the device's at %s", BuzzMediaRootPath)
	}
	for _, unit := range CompanyPackageUnits() {
		if unit.Name != BuzzMediaServiceName {
			continue
		}
		if !strings.Contains(unit.Contents, "posix "+CompanyHostMediaRootPath) {
			t.Fatalf("the packaged gateway does not serve %s, so the bucket dpkg creates is not the one it opens",
				CompanyHostMediaRootPath)
		}
	}
}

// The state root is 0700 root:root, which means no unprivileged account can
// traverse it. A unit that runs as an ordinary account and keeps its state
// inside it would be given a directory it cannot open, and nothing would say so
// until a real company existed. The relay is the unit this catches.
func TestNoUnprivilegedUnitKeepsItsStateInsideTheCompanyTree(t *testing.T) {
	for _, unit := range CompanyPackageUnits() {
		account := settingOf(unit.Contents, "User")
		stateDirectory := settingOf(unit.Contents, "StateDirectory")
		if account == "" || account == "root" || stateDirectory == "" {
			continue
		}
		path := RelayStateDirectoryPath(stateDirectory)
		if strings.HasPrefix(path, CompanyHostStateRoot+"/") {
			t.Fatalf(
				"%s runs as %s and keeps its state at %s, inside %s, which is %04o root:root; it could not open its own directory",
				unit.Name, account, path, CompanyHostStateRoot, CompanyHostStateRootMode)
		}
	}
}

// Three things create the state root: the package, the prepare service, and
// `internkim install` on a box that never saw a package. One mode, read from one
// constant, or the first company to arrive silently changes it.
func TestThePrepareServiceCreatesTheStateRootWithTheModeThePackageGivesIt(t *testing.T) {
	expected := fmt.Sprintf("install -d -o root -g root -m %04o %s", CompanyHostStateRootMode, CompanyHostStateRoot)
	if !strings.Contains(CompanyHostPrepareScript(), expected) {
		t.Fatalf("the prepare service does not create the state root as %q, so a company changes its mode", expected)
	}
}

func settingOf(unitContents string, name string) string {
	for _, line := range strings.Split(unitContents, "\n") {
		if value, found := strings.CutPrefix(line, name+"="); found {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
