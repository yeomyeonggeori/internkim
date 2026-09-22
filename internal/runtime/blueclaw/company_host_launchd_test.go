package blueclaw

import (
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const testHomebrewPrefix = "/opt/homebrew"

// What `internkim install` will have on disk by the time it writes the plists:
// the company's own environment and the six secret files the services read.
func environmentFilesForTest() map[string]string {
	return map[string]string{
		CompanyHostEnvironmentPath: strings.Join([]string{
			"DATABASE_URL=postgres://internkim:secret@127.0.0.1:5432/blueclaw?sslmode=disable",
			"SUPABASE_URL=https://example.supabase.test",
			"SUPABASE_PUBLISHABLE_KEY=publishable",
			"INTERNKIM_APP_URL=https://example.test",
			"GATEWAY_URL=wss://gateway.example.test",
			"MESSENGER_PLATFORM=buzz",
			"CHATD_BOT_USER_NAME=internkim",
			"",
		}, "\n"),
		CompanyHostSettingsPath:     CompanyHostSettingsFile(),
		CompanyHostBuzzRelayKeyPath: "BUZZ_RELAY_PRIVATE_KEY=key\nRELAY_OWNER_PUBKEY=owner\n",
		CompanyHostBuzzDatabasePath: "DATABASE_URL=postgres://internkim:secret@127.0.0.1:5432/buzz?sslmode=disable\n",
		CompanyHostRelayMediaPath:   "BUZZ_S3_ACCESS_KEY=access\nBUZZ_S3_SECRET_KEY=secret\n",
		CompanyHostMediaSecretPath:  "ROOT_ACCESS_KEY_ID=access\nROOT_SECRET_ACCESS_KEY=secret\n",
		CompanyHostChatdSecretPath:  "CHATD_BUZZ_PRIVATE_KEY=private\n",
		RelayEnvironmentFilePath:    "SUPABASE_URL=https://example.supabase.test\nMESSENGER_PLATFORM=buzz\n",
	}
}

func launchDaemonsForTest(t *testing.T) []CompanyHostLaunchDaemon {
	t.Helper()
	daemons, errorValue := CompanyHostLaunchDaemons(MacCompanyHostLayout(testHomebrewPrefix), environmentFilesForTest())
	if errorValue != nil {
		t.Fatalf("the bundle does not render for launchd: %v", errorValue)
	}
	return daemons
}

// One declaration, two supervisors. A service that exists for systemd and not
// for launchd is a Mac that runs eight of the nine things a Debian box runs, and
// nothing would say which one is missing.
func TestTheSameServicesAreRenderedForBothSupervisors(t *testing.T) {
	daemons := launchDaemonsForTest(t)
	units := CompanyHostSystemdUnits(MacCompanyHostLayout(testHomebrewPrefix))
	if len(daemons) != len(units) {
		t.Fatalf("systemd gets %d units and launchd gets %d daemons", len(units), len(daemons))
	}
	for index, unit := range units {
		if daemons[index].ServiceName != unit.Name {
			t.Fatalf("service %d is %s for systemd and %s for launchd", index, unit.Name, daemons[index].ServiceName)
		}
	}
}

// launchd hands ProgramArguments to execve untouched. A ${NAME} that reached a
// plist would be passed to the daemon as its own value, and the daemon would
// start and behave wrongly rather than fail.
func TestNoLaunchDaemonCarriesAnUnexpandedReference(t *testing.T) {
	for _, daemon := range launchDaemonsForTest(t) {
		for _, line := range strings.Split(daemon.Contents, "\n") {
			if strings.Contains(line, "${") && strings.Contains(line, "<string>") {
				t.Fatalf("%s carries %q, which launchd does not expand", daemon.FileName(), strings.TrimSpace(line))
			}
		}
	}
}

// A required environment file that is not there is what happens when the plists
// are written before there is a company. Rendering an empty EnvironmentVariables
// would produce nine daemons that start and can do nothing.
func TestARequiredEnvironmentFileThatIsMissingRefusesToRender(t *testing.T) {
	files := environmentFilesForTest()
	delete(files, CompanyHostBuzzDatabasePath)
	_, errorValue := CompanyHostLaunchDaemons(MacCompanyHostLayout(testHomebrewPrefix), files)
	if errorValue == nil {
		t.Fatal("the messenger rendered without the file that names its database")
	}
	if !strings.Contains(errorValue.Error(), CompanyHostBuzzDatabasePath) {
		t.Fatalf("the refusal does not name the missing file: %v", errorValue)
	}
}

// An optional file is the operator's, and a Mac that has never had one edited is
// the ordinary case.
func TestAnAbsentOperatorSettingsFileIsNotAFailure(t *testing.T) {
	files := environmentFilesForTest()
	delete(files, CompanyHostSettingsPath)
	if _, errorValue := CompanyHostLaunchDaemons(MacCompanyHostLayout(testHomebrewPrefix), files); errorValue != nil {
		t.Fatalf("a Mac with no edited settings file will not come up: %v", errorValue)
	}
}

// systemd applies EnvironmentFile= in the order the unit lists them, so the
// operator's file wins. launchd applies nothing; the merge happens here, and it
// has to keep the same answer or an edited setting would do nothing on a Mac.
func TestTheOperatorSettingsFileStillWinsOnAMac(t *testing.T) {
	files := environmentFilesForTest()
	files[CompanyHostSettingsPath] = "BUZZ_S3_ENDPOINT=http://127.0.0.1:9999\n"
	daemons, errorValue := CompanyHostLaunchDaemons(MacCompanyHostLayout(testHomebrewPrefix), files)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, daemon := range daemons {
		if daemon.ServiceName != BuzzRelayServiceName {
			continue
		}
		if !strings.Contains(daemon.Contents, "http://127.0.0.1:9999") {
			t.Fatalf("the messenger's plist does not carry the edited endpoint:\n%s", daemon.Contents)
		}
		return
	}
	t.Fatal("no messenger daemon was rendered")
}

// A plist launchd cannot parse is a daemon that never starts, and `launchctl
// bootstrap` reports it as one line about the file rather than about the
// service. plutil is the parser launchd itself uses; where there is no plutil,
// well-formed XML is what can be checked.
func TestEveryLaunchDaemonIsAPlistThatParses(t *testing.T) {
	plutilPath, noPlutil := exec.LookPath("plutil")
	for _, daemon := range launchDaemonsForTest(t) {
		if errorValue := xml.Unmarshal([]byte(daemon.Contents), new(struct {
			XMLName xml.Name
			Content []byte `xml:",innerxml"`
		})); errorValue != nil {
			t.Fatalf("%s is not well-formed XML: %v", daemon.FileName(), errorValue)
		}
		if noPlutil != nil {
			continue
		}
		path := filepath.Join(t.TempDir(), daemon.FileName())
		if errorValue := os.WriteFile(path, []byte(daemon.Contents), 0o644); errorValue != nil {
			t.Fatal(errorValue)
		}
		if commandOutput, errorValue := exec.Command(plutilPath, "-lint", path).CombinedOutput(); errorValue != nil {
			t.Fatalf("plutil refuses %s: %s", daemon.FileName(), strings.TrimSpace(string(commandOutput)))
		}
	}
}

// Every daemon must carry the label launchctl is asked about, or `launchctl
// bootstrap` loads a job under one name and `launchctl print` is asked about
// another, and the install reports a service that is not there.
func TestEveryLaunchDaemonCarriesTheLabelItsFileNames(t *testing.T) {
	for _, daemon := range launchDaemonsForTest(t) {
		if !strings.Contains(daemon.Contents, "<string>"+daemon.Label+"</string>") {
			t.Fatalf("%s does not carry its own label", daemon.FileName())
		}
		if !strings.HasPrefix(daemon.Label, CompanyHostLaunchDaemonLabelPrefix) {
			t.Fatalf("%s is not in this product's namespace", daemon.Label)
		}
	}
}

// launchd starts a daemon with /usr/bin:/bin:/usr/sbin:/sbin. Every program the
// bundle shells out to — psql, redis-cli, git, jq, and the bun and uv the keg
// carries — is outside that, so a daemon without a PATH of its own finds none of
// them and fails at the first shell-out rather than at start.
func TestEveryDaemonIsGivenAPathThatHoldsWhatItShellsOutTo(t *testing.T) {
	layout := MacCompanyHostLayout(testHomebrewPrefix)
	for _, daemon := range launchDaemonsForTest(t) {
		if !strings.Contains(daemon.Contents, "<key>PATH</key>") {
			t.Fatalf("%s is started with launchd's own PATH, which holds neither Homebrew nor this package", daemon.FileName())
		}
		if !strings.Contains(daemon.Contents, escapePlistText(layout.SearchPath)) {
			t.Fatalf("%s carries a PATH that is not the layout's:\n%s", daemon.FileName(), daemon.Contents)
		}
	}
}

// The agent is the one service that runs unprivileged, and on launchd UserName
// applies only in the privileged system domain. A plist that lost it would run
// the agent as root and the POSIX boundary would be gone with it.
func TestTheAgentRunsAsItsOwnAccountOnAMacToo(t *testing.T) {
	for _, daemon := range launchDaemonsForTest(t) {
		if daemon.ServiceName != BlueclawServiceName {
			continue
		}
		if !strings.Contains(daemon.Contents, "<key>UserName</key>\n\t<string>"+BlueclawUser+"</string>") {
			t.Fatalf("the agent's plist does not run it as %s:\n%s", BlueclawUser, daemon.Contents)
		}
		return
	}
	t.Fatal("no agent daemon was rendered")
}

// macOS 11 and newer seal the root volume, so a path invented at / cannot be
// created there. The Debian layout keeps /workspace; the Mac layout must not,
// and it must not answer with something inside the 0700 state root either.
func TestTheMacLayoutPutsTheWorkspaceWhereAMacCanHaveOne(t *testing.T) {
	layout := MacCompanyHostLayout(testHomebrewPrefix)
	if strings.Count(layout.WorkspacePath, "/") < 2 {
		t.Fatalf("the Mac workspace is %s, a directory at the root of a read-only volume", layout.WorkspacePath)
	}
	if strings.HasPrefix(layout.WorkspacePath, CompanyHostStateRoot+"/") {
		t.Fatalf("the Mac workspace is inside %s, which is %04o root:root; no task user could open it",
			CompanyHostStateRoot, CompanyHostStateRootMode)
	}
	for _, path := range []string{layout.BinaryRoot, layout.HelperRoot, layout.LibraryRoot} {
		if !strings.HasPrefix(path, testHomebrewPrefix+"/") {
			t.Fatalf("%s is outside the Homebrew prefix, so `brew uninstall` would leave it behind", path)
		}
	}
	// A program a plist names has to survive an upgrade that replaces the keg,
	// which the opt link does and the Cellar path does not. It must also not be
	// the prefix's own bin: the package vendors bun, uv and agent-browser, and
	// Homebrew has a formula for each of those names, so a keg that put them
	// there could not be linked at all.
	if !strings.HasPrefix(layout.BinaryRoot, testHomebrewPrefix+"/opt/") {
		t.Fatalf("the programs are at %s, which an upgrade moves", layout.BinaryRoot)
	}
	if strings.HasPrefix(layout.BinaryRoot, testHomebrewPrefix+"/bin") {
		t.Fatalf("the programs are in the prefix's bin, where bun, uv and agent-browser collide with their own formulas")
	}
}
