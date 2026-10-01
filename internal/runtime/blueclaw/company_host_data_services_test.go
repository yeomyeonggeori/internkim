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
	mac := MacCompanyHostLayout("/opt/homebrew")
	if !strings.Contains(mac.DatabaseURL("internkim", "x", "blueclaw"), "127.0.0.1:5432") || mac.CacheURL() != BuzzRelayRedisURL {
		t.Error("a Mac is given a socket address for a database Homebrew runs on loopback")
	}
}
