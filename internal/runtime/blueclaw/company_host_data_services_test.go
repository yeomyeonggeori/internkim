package blueclaw

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func packagedUnit(t *testing.T, name string) string {
	t.Helper()
	for _, unit := range CompanyPackageUnits() {
		if unit.Name == name {
			return unit.Contents
		}
	}
	t.Fatalf("the package installs no %s unit", name)
	return ""
}

func TestNoPackagedUnitNamesTheDistributionsDatabaseOrCache(t *testing.T) {
	for _, unit := range CompanyPackageUnits() {
		for _, theirs := range []string{"postgresql.service", "redis-server.service", "redis.service", "valkey.service"} {
			if regexp.MustCompile(`(^|[ =])` + regexp.QuoteMeta(theirs)).MatchString(unit.Contents) {
				t.Errorf("%s names %s, which is a service the company did not make and may already be running its own data", unit.FileName(), theirs)
			}
		}
	}
}

func TestTheUnitsThatOpenTheDatabaseAreBoundToOurs(t *testing.T) {
	for _, name := range []string{BuzzRelayServiceName, BlueclawServiceName} {
		contents := packagedUnit(t, name)
		for _, required := range []string{
			"BindsTo=" + companyHostDatabaseUnitName,
			"After=network-online.target " + companyHostDatabaseUnitName,
		} {
			if !strings.Contains(contents, required) {
				t.Errorf("%s does not say %q", name, required)
			}
		}
	}
	if !strings.Contains(packagedUnit(t, BuzzRelayServiceName), "Wants=network-online.target "+companyHostCacheUnitName) {
		t.Error("the messenger does not want the company's cache")
	}
}

func TestOnlyTheAgentIsGivenTheDatabasesGroup(t *testing.T) {
	for _, unit := range CompanyPackageUnits() {
		hasIt := strings.Contains(unit.Contents, "SupplementaryGroups="+CompanyHostDatabaseUser)
		if hasIt != (unit.Name == BlueclawServiceName) {
			t.Errorf("%s has the database group = %v; only %s may, so that the agent reaches the socket and nothing else does", unit.Name, hasIt, BlueclawServiceName)
		}
	}
}

func TestEachDataServiceRunsAsItsOwnAccountWithPrivateStateAndASocketDirectory(t *testing.T) {
	for name, account := range map[string]string{
		CompanyHostDatabaseServiceName: CompanyHostDatabaseUser,
		CompanyHostCacheServiceName:    CompanyHostCacheUser,
	} {
		contents := packagedUnit(t, name)
		for _, required := range []string{
			"User=" + account,
			"Group=" + account,
			"StateDirectoryMode=0700",
			"RuntimeDirectoryMode=0750",
			"NoNewPrivileges=true",
			"ConditionPathExists=" + CompanyHostAgentKeyPath,
		} {
			if !strings.Contains(contents, required) {
				t.Errorf("%s does not say %q", name, required)
			}
		}
	}
}

func TestTheDataServicesLiveBesideTheStateRootBecauseNothingUnprivilegedCanEnterIt(t *testing.T) {
	for _, path := range []string{CompanyHostDatabaseDataPath, CompanyHostCacheDataPath, CompanyHostDatabaseSocketDirectory, CompanyHostCacheSocketPath} {
		if strings.HasPrefix(path, CompanyHostStateRoot+"/") || strings.HasPrefix(path, CompanyHostRunPath+"/") {
			t.Errorf("%s is inside a directory that only root, or root and the agent, can enter", path)
		}
	}
}

func TestTheDataServiceScriptIsShellAndHasNoPlaceholderLeft(t *testing.T) {
	script := CompanyHostDataServiceScript()
	if placeholder := regexp.MustCompile(`@[A-Z_]+@`).FindString(script); placeholder != "" {
		t.Fatalf("the data-service script still has the placeholder %s", placeholder)
	}
	command := exec.Command("sh", "-n")
	command.Stdin = strings.NewReader(script)
	if output, errorValue := command.CombinedOutput(); errorValue != nil {
		t.Fatalf("the data-service script is not valid shell: %s", output)
	}
}

func TestTheDatabaseListensOnNoNetworkAddressAndTheCacheOnNoPort(t *testing.T) {
	script := CompanyHostDataServiceScript()
	for _, required := range []string{"-c listen_addresses=", "--port 0"} {
		if !strings.Contains(script, required) {
			t.Errorf("the data-service script does not pass %q, so a service could listen on the network", required)
		}
	}
	if !strings.Contains(script, "local all all scram-sha-256") || strings.Contains(script, "\nhost ") {
		t.Error("the cluster's pg_hba.conf names a network rule or lets a local role in without a password")
	}
}

func TestTheDatabaseRewritesItsAccessRulesWhenReloaded(t *testing.T) {
	layout := LinuxCompanyHostLayout()
	if !strings.Contains(companyHostDatabaseUnit(layout), "ExecReload="+layout.DataServicePath()+" postgres-access\n") {
		t.Error("the database unit has no reload, so a database an upgrade leaves running keeps the access rules it started with")
	}
	if strings.Contains(companyHostCacheUnit(layout), "ExecReload=") {
		t.Error("the cache has no access rules to reload")
	}
	script := CompanyHostDataServiceScript()
	rewritesThenReloads := "write_access_rules\n  exec \"$programs/pg_ctl\" --pgdata \"$data\" --silent reload"
	if !strings.Contains(script, "postgres-access) reload_access_rules ;;") || !strings.Contains(script, rewritesThenReloads) {
		t.Error("postgres-access does not write the access rules and then ask the server to read them")
	}
}

func TestEveryClientIsGivenASocketAddressOnALinuxHost(t *testing.T) {
	layout := LinuxCompanyHostLayout()
	url := layout.DatabaseURL("internkim", "pass word", "blueclaw")
	if !strings.Contains(url, "host=%2Frun%2Finternkim-postgres") || strings.Contains(url, "127.0.0.1") {
		t.Errorf("the database address is %q and does not name the socket directory", url)
	}
	if strings.Contains(url, "@/") || strings.ContainsAny(url[strings.Index(url, "?"):], "&|") {
		t.Errorf("the database address %q has an empty host, which sqlx refuses, or a character the renderer once mangled", url)
	}
	if layout.CacheURL() != "redis+unix://"+CompanyHostCacheSocketPath {
		t.Errorf("the cache address is %q", layout.CacheURL())
	}
}

// A person's own PostgreSQL and Redis on the same Mac answer on 5432 and 6379.
// The host's database answers on a port of its own and its cache on no port.
func TestAMacClientReachesTheHostsOwnDatabaseAndCacheAndNotThePersons(t *testing.T) {
	mac := MacCompanyHostLayout(testHomebrewPrefix)
	if url := mac.DatabaseURL("internkim", "x", "blueclaw"); !strings.Contains(url, "@"+macDatabaseLoopbackAddress+"/") || strings.Contains(url, ":5432") {
		t.Errorf("the database address is %q", url)
	}
	if mac.CacheURL() != "redis+unix://"+CompanyHostCacheDataPath+"/cache.sock" {
		t.Errorf("the cache address is %q, which is not the socket in the cache's own directory", mac.CacheURL())
	}
}

func macDataDaemon(t *testing.T, serviceName string) CompanyHostLaunchDaemon {
	t.Helper()
	daemons, errorValue := CompanyHostDataLaunchDaemons(MacCompanyHostLayout(testHomebrewPrefix))
	if errorValue != nil {
		t.Fatalf("the data daemons do not render: %v", errorValue)
	}
	for _, daemon := range daemons {
		if daemon.ServiceName == serviceName {
			return daemon
		}
	}
	t.Fatalf("a Mac has no %s daemon among %v", serviceName, daemons)
	return CompanyHostLaunchDaemon{}
}

func renderedPlistString(key string, value string) string {
	return "<key>" + key + "</key>\n\t<string>" + value + "</string>"
}

func TestTheMacDatabaseIsTheCarriedPostgresRunAsTheHostsOwnAccount(t *testing.T) {
	daemon := macDataDaemon(t, CompanyHostDatabaseServiceName)
	if daemon.Label != "kim.intern."+CompanyHostDatabaseServiceName {
		t.Errorf("the database's label is %s", daemon.Label)
	}
	for _, required := range []string{
		renderedPlistString("UserName", CompanyHostDatabaseUser),
		renderedPlistString("GroupName", CompanyHostDatabaseUser),
		"<array>\n\t\t<string>" + testHomebrewPrefix + "/opt/internkim/libexec/postgresql/bin/postgres</string>",
		"<string>-D</string>\n\t\t<string>/var/lib/internkim-postgres</string>",
		"<string>listen_addresses=127.0.0.1</string>",
		"<string>port=18432</string>",
		"<string>unix_socket_directories=/var/lib/internkim-postgres</string>",
	} {
		if !strings.Contains(daemon.Contents, required) {
			t.Errorf("the database daemon does not carry %q:\n%s", required, daemon.Contents)
		}
	}
}

func TestTheMacCacheIsHomebrewsRedisOnASocketOnly(t *testing.T) {
	daemon := macDataDaemon(t, CompanyHostCacheServiceName)
	for _, required := range []string{
		renderedPlistString("UserName", CompanyHostCacheUser),
		"<array>\n\t\t<string>" + testHomebrewPrefix + "/opt/redis/bin/redis-server</string>",
		"<string>--port</string>\n\t\t<string>0</string>",
		"<string>--unixsocket</string>\n\t\t<string>/var/lib/internkim-cache/cache.sock</string>",
		"<string>--dir</string>\n\t\t<string>/var/lib/internkim-cache</string>",
	} {
		if !strings.Contains(daemon.Contents, required) {
			t.Errorf("the cache daemon does not carry %q:\n%s", required, daemon.Contents)
		}
	}
}

func TestTheMacClusterIsMadeAsTheDatabasesAccountWithPeerForItAlone(t *testing.T) {
	initialization := strings.Join(CompanyHostDatabaseInitialization(MacCompanyHostLayout(testHomebrewPrefix)), " ")
	for _, required := range []string{
		testHomebrewPrefix + "/opt/internkim/libexec/postgresql/bin/initdb",
		"--pgdata " + CompanyHostDatabaseDataPath,
		"--username " + CompanyHostDatabaseUser,
		"--auth-local=peer",
		"--auth-host=scram-sha-256",
	} {
		if !strings.Contains(initialization, required) {
			t.Errorf("the cluster is made without %q: %s", required, initialization)
		}
	}
}

func TestALinuxHostHasNoDataLaunchDaemons(t *testing.T) {
	daemons, errorValue := CompanyHostDataLaunchDaemons(LinuxCompanyHostLayout())
	if errorValue != nil || len(daemons) != 0 {
		t.Fatalf("a Linux host renders launchd daemons for its database and cache: %v %v", daemons, errorValue)
	}
}

func dataServiceScriptOver(t *testing.T, programs string, data string) string {
	t.Helper()
	script := CompanyHostDataServiceScript()
	carried := `programs="` + LinuxCompanyHostLayout().DatabaseProgramDirectory() + `"`
	if !strings.Contains(script, carried) {
		t.Fatal("the data-service script does not name the carried programs to point at the fixture")
	}
	script = strings.Replace(script, carried, `programs="`+programs+`"`, 1)
	return strings.Replace(script, `data="`+CompanyHostDatabaseDataPath+`"`, `data="`+data+`"`, 1)
}

func writeFixtureProgram(t *testing.T, directory string, name string, body string) {
	t.Helper()
	if errorValue := os.WriteFile(filepath.Join(directory, name), []byte("#!/bin/sh\n"+body), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func runRecoveryAgainstFixture(t *testing.T, statusBody string) (string, string, error) {
	t.Helper()
	programs, data := t.TempDir(), t.TempDir()
	calls := filepath.Join(t.TempDir(), "calls")
	writeFixtureProgram(t, programs, "pg_ctl", "for word; do last=$word; done\necho \"pg_ctl $last\" >> "+calls+"\n"+statusBody)
	writeFixtureProgram(t, programs, "psql", "echo psql >> "+calls+"\n[ \"$(grep -c psql "+calls+")\" -ge 3 ] && echo f || echo t\n")
	writeFixtureProgram(t, programs, "sleep", "")
	command := exec.Command("sh", "-s", "postgres-recover")
	command.Stdin = strings.NewReader(dataServiceScriptOver(t, programs, data))
	command.Env = append(os.Environ(), "PATH="+programs+":"+os.Getenv("PATH"))
	output, errorValue := command.CombinedOutput()
	recorded, _ := os.ReadFile(calls)
	return string(recorded), string(output), errorValue
}

func TestReplayingTheKeptWALStopsPostgreSQLOnlyOnceRecoveryHasEnded(t *testing.T) {
	calls, output, errorValue := runRecoveryAgainstFixture(t, "exit 0\n")
	if errorValue != nil {
		t.Fatalf("postgres-recover failed: %v\n%s", errorValue, output)
	}
	want := "pg_ctl start\npsql\npg_ctl status\npsql\npg_ctl status\npsql\npg_ctl stop\n"
	if calls != want {
		t.Fatalf("pg_ctl --wait returns once the server is consistent, before it has replayed the WAL after that point, so stopping then leaves the database in recovery on its old timeline; the calls were:\n%s", calls)
	}
}

func TestReplayingTheKeptWALFailsWhenPostgreSQLStopsBeforeItsEnd(t *testing.T) {
	calls, output, errorValue := runRecoveryAgainstFixture(t, "[ \"$last\" = status ] && exit 3\nexit 0\n")
	if errorValue == nil {
		t.Fatalf("postgres-recover succeeded although PostgreSQL stopped while replaying; the calls were:\n%s", calls)
	}
	if !strings.Contains(output, "stopped before it replayed the kept WAL to its end") {
		t.Fatalf("postgres-recover failed without saying the replay was cut short: %s", output)
	}
}

func TestEveryClusterIsMadeWithOneLocaleThatClassifiesHangul(t *testing.T) {
	wanted := "--encoding=UTF8 --locale-provider=builtin --builtin-locale=C.UTF-8 --locale=C --lc-ctype=C.UTF-8 --data-checksums"
	mac := strings.Join(CompanyHostDatabaseInitialization(MacCompanyHostLayout(testHomebrewPrefix)), " ")
	if !strings.Contains(mac, wanted) {
		t.Errorf("a Mac makes its cluster with %q and not %q", mac, wanted)
	}
	if !strings.Contains(CompanyHostDataServiceScript(), wanted) {
		t.Errorf("the data-service script does not make its cluster with %q", wanted)
	}
}

func TestEveryHostRunsTheDatabaseProgramsThePackageCarries(t *testing.T) {
	for _, layout := range []CompanyHostLayout{LinuxCompanyHostLayout(), MacCompanyHostLayout(testHomebrewPrefix)} {
		if want := layout.LibraryRoot + "/postgresql/bin/psql"; layout.DatabaseProgram("psql") != want {
			t.Errorf("psql is %s and the package carries it at %s", layout.DatabaseProgram("psql"), want)
		}
	}
	if !strings.Contains(CompanyHostDataServiceScript(), "programs=\""+LinuxCompanyHostLayout().DatabaseProgramDirectory()+"\"") {
		t.Error("the data-service script does not run the PostgreSQL the package carries")
	}
}

func TestTheDataServiceRefusesAClusterAnotherMajorMade(t *testing.T) {
	data := t.TempDir()
	writeTestFile(t, data+"/PG_VERSION", "16\n", 0o644)
	script := strings.ReplaceAll(CompanyHostDataServiceScript(), CompanyHostDatabaseDataPath, data)

	command := exec.Command("sh", "-s", "postgres")
	command.Stdin = strings.NewReader(script)
	output, errorValue := command.CombinedOutput()

	if errorValue == nil {
		t.Fatalf("a cluster PostgreSQL 16 made was started by the PostgreSQL 17 the package carries: %s", output)
	}
	for _, wanted := range []string{"made by PostgreSQL 16", "carries PostgreSQL 17", "new box", "snapshot"} {
		if !strings.Contains(string(output), wanted) {
			t.Errorf("the refusal does not say %q: %s", wanted, output)
		}
	}
}

func writeTestFile(t *testing.T, path string, contents string, mode os.FileMode) {
	t.Helper()
	if errorValue := os.WriteFile(path, []byte(contents), mode); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestTheDataServiceNamesTheMajorThePackageCarries(t *testing.T) {
	command := exec.Command("sh", "-s", "postgres-major")
	command.Stdin = strings.NewReader(CompanyHostDataServiceScript())
	output, errorValue := command.Output()
	if errorValue != nil || strings.TrimSpace(string(output)) != CarriedDatabaseMajor {
		t.Fatalf("postgres-major answered %q, %v, and the package carries %s", output, errorValue, CarriedDatabaseMajor)
	}
}
