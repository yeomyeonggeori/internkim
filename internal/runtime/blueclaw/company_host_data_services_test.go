package blueclaw

import (
	"os/exec"
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

func TestTheMacDatabaseIsHomebrewsPostgresRunAsTheHostsOwnAccount(t *testing.T) {
	daemon := macDataDaemon(t, CompanyHostDatabaseServiceName)
	if daemon.Label != "kim.intern."+CompanyHostDatabaseServiceName {
		t.Errorf("the database's label is %s", daemon.Label)
	}
	for _, required := range []string{
		renderedPlistString("UserName", CompanyHostDatabaseUser),
		renderedPlistString("GroupName", CompanyHostDatabaseUser),
		"<array>\n\t\t<string>" + testHomebrewPrefix + "/opt/postgresql@17/bin/postgres</string>",
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
		testHomebrewPrefix + "/opt/postgresql@17/bin/initdb",
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
