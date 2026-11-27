package blueclaw

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// The company host runs its own database, from the PostgreSQL the package
// carries, and its own cache, from the server the machine's package manager
// installs, and never uses the machine's default cluster or cache service. A
// company server that already runs a PostgreSQL or a Redis keeps its data in it;
// sharing that instance would put this company's tables beside theirs and make a
// package upgrade a change to both. On a Mac the same holds for the person's own
// `brew services`: the host starts its own binaries as its own accounts and
// leaves those services, and the data directories they own, alone.
//
// Under systemd neither service listens on a network address: each answers on a
// Unix socket in a directory only its own group can open, so there is no port to
// collide with and nothing on loopback to guard. The same two units serve every
// family. Where the cache's binaries are and whether it is called redis or valkey
// differ between distributions, and CompanyHostDataServiceScript is the one place
// that knows.
//
// launchd has no RuntimeDirectory= and nothing it starts as an unprivileged
// account can create one under /var/run, so on a Mac each socket sits in its
// service's own data directory, which only that account can open. The cache's
// one client, the messenger, runs as root and reaches it there. The agent does
// not, so the database also answers on DatabaseLoopbackAddress, where the
// company's roles sign in with their passwords.
const (
	CompanyHostDatabaseServiceName = "internkim-postgresql"
	CompanyHostCacheServiceName    = "internkim-cache"

	CompanyHostDatabaseUser = "internkim-postgres"
	CompanyHostCacheUser    = "internkim-cache"

	// The state root is 0700 root:root, so neither account could open a
	// directory inside it; both keep their state beside it, as the relay does.
	CompanyHostDatabaseStateDirectoryName = "internkim-postgres"
	CompanyHostCacheStateDirectoryName    = "internkim-cache"
	CompanyHostDatabaseDataPath           = "/var/lib/" + CompanyHostDatabaseStateDirectoryName
	CompanyHostCacheDataPath              = "/var/lib/" + CompanyHostCacheStateDirectoryName

	CompanyHostDatabaseRuntimeDirectoryName = "internkim-postgres"
	CompanyHostCacheRuntimeDirectoryName    = "internkim-cache"
	CompanyHostDatabaseSocketDirectory      = "/run/" + CompanyHostDatabaseRuntimeDirectoryName
	CompanyHostCacheSocketPath              = "/run/" + CompanyHostCacheRuntimeDirectoryName + "/cache.sock"

	CompanyHostDataServiceProgramName = "data-service"
	CompanyHostDataServiceMode        = 0o755

	companyHostDataDirectoryMode       = "0750"
	companyHostDataSocketMode          = "0660"
	CompanyHostDatabasePort            = "5432"
	companyHostDatabaseIdentMap        = "internkim"
	companyHostDatabaseRecoverySeconds = "86400"

	DatabaseServerProgramName = "postgres"

	// The builtin provider's C.UTF-8 sorts by code point, the same on every
	// machine, and classifies Hangul as letters: with lc_ctype C, pg_trgm takes
	// no trigram from a Korean word.
	companyHostDatabaseCharacterLocale = "C.UTF-8"
)

func CompanyHostDatabaseClusterOptions() []string {
	return []string{
		"--encoding=UTF8",
		"--locale-provider=builtin",
		"--builtin-locale=" + companyHostDatabaseCharacterLocale,
		"--locale=C",
		"--lc-ctype=" + companyHostDatabaseCharacterLocale,
		"--data-checksums",
	}
}

// IsDataService is true of the database and the cache. Restarting one restarts
// everything bound to it, so a package upgrade or an install that restarts the
// whole bundle in one transaction asks systemd to stop and start the same unit
// twice, and the job for each unit bound to it fails.
func (unit CompanyPackageUnit) IsDataService() bool {
	return unit.Name == CompanyHostDatabaseServiceName || unit.Name == CompanyHostCacheServiceName
}

// RunsTheDataServiceScript is true where the data-service script finds the
// cache's server, and false where the layout names its directory.
func (layout CompanyHostLayout) RunsTheDataServiceScript() bool {
	return layout.CacheProgramDirectory == ""
}

func (layout CompanyHostLayout) DatabaseProgramDirectory() string {
	return layout.LibraryRoot + "/" + databaseTreeName + "/bin"
}

func (layout CompanyHostLayout) DatabaseProgram(programName string) string {
	return layout.DatabaseProgramDirectory() + "/" + programName
}

func (layout CompanyHostLayout) CacheProgram(programName string) string {
	if layout.CacheProgramDirectory == "" {
		return programName
	}
	return layout.CacheProgramDirectory + "/" + programName
}

// DatabasePort is the port the database answers on, which also names its
// socket file inside DatabaseSocketDirectory.
func (layout CompanyHostLayout) DatabasePort() string {
	if layout.DatabaseLoopbackAddress == "" {
		return CompanyHostDatabasePort
	}
	_, port, _ := net.SplitHostPort(layout.DatabaseLoopbackAddress)
	return port
}

func (layout CompanyHostLayout) DatabaseLoopbackHost() string {
	host, _, _ := net.SplitHostPort(layout.DatabaseLoopbackAddress)
	return host
}

// DatabaseURL is how a client reaches the company's database on this machine.
// The role and password are the company's own; the address is loopback where
// the layout names one and the socket directory otherwise.
//
// The socket form names localhost and then overrides it with host=. A URL with
// credentials and an empty host is refused by the url crate sqlx parses with
// ("empty host"), and the ampersand a second parameter would need is the one
// character tools/render-company-runtime's old sed treated as the matched text.
// A Unix socket has no TLS to disable, so there is no second parameter.
func (layout CompanyHostLayout) DatabaseURL(role string, password string, database string) string {
	credentials := url.UserPassword(role, password).String()
	if layout.DatabaseLoopbackAddress != "" {
		return fmt.Sprintf("postgres://%s@%s/%s?sslmode=disable", credentials, layout.DatabaseLoopbackAddress, database)
	}
	return fmt.Sprintf("postgres://%s@localhost/%s?host=%s", credentials, database, url.QueryEscape(layout.DatabaseSocketDirectory))
}

// CacheURL is what the messenger is told for REDIS_URL. The socket form is the
// one the client library documents as redis+unix.
func (layout CompanyHostLayout) CacheURL() string {
	return "redis+unix://" + layout.CacheSocketPath
}

func (layout CompanyHostLayout) DataServicePath() string {
	return layout.HelperRoot + "/" + CompanyHostDataServiceProgramName
}

// CompanyHostDataServiceUnits are the two units, systemd's alone. A Mac starts
// the same two services from CompanyHostDataLaunchDaemons.
func CompanyHostDataServiceUnits(layout CompanyHostLayout) []CompanyPackageUnit {
	if !layout.RunsTheDataServiceScript() {
		return nil
	}
	return []CompanyPackageUnit{
		{Name: CompanyHostDatabaseServiceName, Contents: companyHostDatabaseUnit(layout)},
		{Name: CompanyHostCacheServiceName, Contents: companyHostCacheUnit(layout)},
	}
}

func companyHostDatabaseUnit(layout CompanyHostLayout) string {
	program := layout.DataServicePath()
	return companyHostDataServiceUnit(dataServiceUnitSettings{
		Description:      "internkim company database",
		Account:          CompanyHostDatabaseUser,
		StateDirectory:   CompanyHostDatabaseStateDirectoryName,
		RuntimeDirectory: CompanyHostDatabaseRuntimeDirectoryName,
		ExecStart:        program + " postgres",
		ExecStartPost:    program + " postgres-ready",
		ExecReload:       program + " postgres-access",
		StopTimeout:      60,
	})
}

func companyHostCacheUnit(layout CompanyHostLayout) string {
	program := layout.DataServicePath()
	return companyHostDataServiceUnit(dataServiceUnitSettings{
		Description:      "internkim company cache",
		Account:          CompanyHostCacheUser,
		StateDirectory:   CompanyHostCacheStateDirectoryName,
		RuntimeDirectory: CompanyHostCacheRuntimeDirectoryName,
		ExecStart:        program + " cache",
		ExecStartPost:    program + " cache-ready",
		StopTimeout:      30,
	})
}

// CompanyHostDataLaunchDaemons are the database and the cache as LaunchDaemons,
// started from the binaries of the formulas the package depends on. They are
// apart from the bundle's daemons because the install prepares the database before
// it starts anything that opens it.
func CompanyHostDataLaunchDaemons(layout CompanyHostLayout) ([]CompanyHostLaunchDaemon, error) {
	if layout.RunsTheDataServiceScript() {
		return nil, nil
	}
	daemons := []CompanyHostLaunchDaemon{}
	for _, service := range []CompanyHostService{companyHostDatabaseDaemon(layout), companyHostCacheDaemon(layout)} {
		daemon, errorValue := companyHostLaunchDaemon(layout, service, nil)
		if errorValue != nil {
			return nil, errorValue
		}
		daemons = append(daemons, daemon)
	}
	return daemons, nil
}

func companyHostDatabaseDaemon(layout CompanyHostLayout) CompanyHostService {
	return CompanyHostService{
		Name:        CompanyHostDatabaseServiceName,
		Description: "internkim company database",
		Command: []string{
			layout.DatabaseProgram("postgres"),
			"-D", CompanyHostDatabaseDataPath,
			"-c", "listen_addresses=" + layout.DatabaseLoopbackHost(),
			"-c", "port=" + layout.DatabasePort(),
			"-c", "unix_socket_directories=" + layout.DatabaseSocketDirectory,
			"-c", "logging_collector=off",
		},
		Account:             CompanyHostDatabaseUser,
		WorkingDirectory:    CompanyHostDatabaseDataPath,
		RestartAfterSeconds: 5,
		StopTimeoutSeconds:  60,
	}
}

func companyHostCacheDaemon(layout CompanyHostLayout) CompanyHostService {
	return CompanyHostService{
		Name:        CompanyHostCacheServiceName,
		Description: "internkim company cache",
		Command: []string{
			layout.CacheProgram("redis-server"),
			"--port", "0",
			"--unixsocket", layout.CacheSocketPath,
			"--unixsocketperm", strings.TrimPrefix(companyHostDataSocketMode, "0"),
			"--save", "",
			"--appendonly", "no",
			"--dir", CompanyHostCacheDataPath,
		},
		Account:             CompanyHostCacheUser,
		WorkingDirectory:    CompanyHostCacheDataPath,
		RestartAfterSeconds: 5,
		StopTimeoutSeconds:  30,
	}
}

// CompanyHostDatabaseInitialization is what makes the cluster on a Mac, run once
// as the database's account. The superuser is that account's own name and signs
// in by peer on the socket, which is how the install reaches it; every other
// role signs in with its password on loopback.
func CompanyHostDatabaseInitialization(layout CompanyHostLayout) []string {
	return append([]string{
		layout.DatabaseProgram("initdb"),
		"--pgdata", CompanyHostDatabaseDataPath,
		"--username", CompanyHostDatabaseUser,
		"--auth-local=peer",
		"--auth-host=scram-sha-256",
	}, CompanyHostDatabaseClusterOptions()...)
}

type dataServiceUnitSettings struct {
	Description      string
	Account          string
	StateDirectory   string
	RuntimeDirectory string
	ExecStart        string
	ExecStartPost    string
	ExecReload       string
	StopTimeout      int
}

func companyHostDataServiceUnit(settings dataServiceUnitSettings) string {
	return fmt.Sprintf(`[Unit]
Description=%s
Documentation=https://docs.intern.kim/running-the-host/
After=local-fs.target
ConditionPathExists=%s

[Service]
User=%[3]s
Group=%[3]s
StateDirectory=%s
StateDirectoryMode=0700
RuntimeDirectory=%s
RuntimeDirectoryMode=%s
ExecStart=%s
ExecStartPost=%s
%sTimeoutStartSec=120
TimeoutStopSec=%d
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true

[Install]
WantedBy=multi-user.target
`, settings.Description, CompanyHostAgentKeyPath, settings.Account,
		settings.StateDirectory, settings.RuntimeDirectory, companyHostDataDirectoryMode,
		settings.ExecStart, settings.ExecStartPost, settings.reloadLine(), settings.StopTimeout)
}

func (settings dataServiceUnitSettings) reloadLine() string {
	if settings.ExecReload == "" {
		return ""
	}
	return "ExecReload=" + settings.ExecReload + "\n"
}

// CompanyHostDataServiceScript is the one program both data units run. It runs
// the PostgreSQL the package carries, refuses a cluster another major made,
// finds the cache server the distribution installed, makes the company's cluster
// on first start, and answers the readiness questions the units and the
// installer ask. It is rendered here rather than kept as a file so that the
// paths it names are the constants the units name.
func CompanyHostDataServiceScript() string {
	replacer := strings.NewReplacer(
		"@DATABASE_PROGRAMS@", LinuxCompanyHostLayout().DatabaseProgramDirectory(),
		"@CLUSTER_OPTIONS@", strings.Join(CompanyHostDatabaseClusterOptions(), " "),
		"@DATA@", CompanyHostDatabaseDataPath,
		"@SOCKET_DIRECTORY@", CompanyHostDatabaseSocketDirectory,
		"@SOCKET_MODE@", companyHostDataSocketMode,
		"@PORT@", CompanyHostDatabasePort,
		"@ROLE@", CompanyHostDatabaseUser,
		"@IDENT_MAP@", companyHostDatabaseIdentMap,
		"@RECOVERY_SECONDS@", companyHostDatabaseRecoverySeconds,
		"@CACHE_SOCKET@", CompanyHostCacheSocketPath,
		"@CACHE_DATA@", CompanyHostCacheDataPath,
	)
	return replacer.Replace(companyHostDataServiceTemplate)
}

const companyHostDataServiceTemplate = `#!/bin/sh
set -eu

programs="@DATABASE_PROGRAMS@"
data="@DATA@"
socket_directory="@SOCKET_DIRECTORY@"
cache_socket="@CACHE_SOCKET@"

fail() {
  echo "internkim-data-service: $1" >&2
  exit 1
}

carried_major() {
  "$programs/postgres" --version | sed -n 's/^postgres (PostgreSQL) \([0-9][0-9]*\).*/\1/p'
}

refuse_a_cluster_another_major_made() {
  [ -r "$data/PG_VERSION" ] || return 0
  cluster_major="$(cat "$data/PG_VERSION")"
  carried="$(carried_major)"
  [ "$cluster_major" = "$carried" ] || fail "the company database in $data was made by PostgreSQL $cluster_major, and this package carries PostgreSQL $carried, which cannot open it. Move the company to a new box: a move restores the company from its snapshot"
}

initialize_cluster() {
  if [ -r "$data/PG_VERSION" ]; then return 0; fi
  "$programs/initdb" --pgdata "$data" --username @ROLE@ --auth-local=peer --auth-host=reject \
    @CLUSTER_OPTIONS@ >/dev/null
}

write_access_rules() {
  printf '%s\n' \
    'local all @ROLE@ peer map=@IDENT_MAP@' \
    'local replication @ROLE@ peer map=@IDENT_MAP@' \
    'local all all scram-sha-256' > "$data/pg_hba.conf"
  printf '%s\n' \
    '@IDENT_MAP@ @ROLE@ @ROLE@' \
    '@IDENT_MAP@ root @ROLE@' > "$data/pg_ident.conf"
}

reload_access_rules() {
  refuse_a_cluster_another_major_made
  write_access_rules
  exec "$programs/pg_ctl" --pgdata "$data" --silent reload
}

run_postgres() {
  refuse_a_cluster_another_major_made
  initialize_cluster
  write_access_rules
  exec "$programs/postgres" -D "$data" \
    -c listen_addresses= \
    -c logging_collector=off \
    -c port=@PORT@ \
    -c unix_socket_directories="$socket_directory" \
    -c unix_socket_permissions=@SOCKET_MODE@ \
    -c unix_socket_group=@ROLE@
}

wait_until() {
  for attempt in $(seq 1 100); do
    "$@" >/dev/null 2>&1 && return 0
    sleep 0.5
  done
  return 1
}

postgres_answers() {
  "$programs/pg_isready" --quiet --host "$socket_directory"
}

postgres_ready() {
  wait_until postgres_answers || fail "PostgreSQL did not accept connections on $socket_directory"
}

recovery_has_ended() {
  [ "$("$programs/psql" --host "$data" --port @PORT@ --username @ROLE@ --dbname postgres --no-psqlrc --tuples-only --no-align \
    --command 'select pg_is_in_recovery()' 2>/dev/null)" = f ]
}

wait_for_recovery_to_end() {
  waited=0
  while [ "$waited" -lt @RECOVERY_SECONDS@ ]; do
    recovery_has_ended && return 0
    "$programs/pg_ctl" --pgdata "$data" --silent status >/dev/null || fail "PostgreSQL stopped before it replayed the kept WAL to its end"
    sleep 1
    waited=$((waited + 1))
  done
  fail "PostgreSQL was still replaying the kept WAL after @RECOVERY_SECONDS@ seconds"
}

recover_postgres() {
  refuse_a_cluster_another_major_made
  write_access_rules
  "$programs/pg_ctl" --pgdata "$data" --wait --timeout @RECOVERY_SECONDS@ --silent \
    --options "-c listen_addresses= -c port=@PORT@ -c unix_socket_directories=$data" start
  wait_for_recovery_to_end
  "$programs/pg_ctl" --pgdata "$data" --wait --mode fast --silent stop
}

cache_program() {
  for name in valkey-server redis-server; do
    if command -v "$name"; then return 0; fi
  done
  fail "neither valkey-server nor redis-server is installed"
}

cache_client() {
  for name in valkey-cli redis-cli; do
    if command -v "$name"; then return 0; fi
  done
  fail "neither valkey-cli nor redis-cli is installed"
}

run_cache() {
  program="$(cache_program)"
  exec "$program" --port 0 --unixsocket "$cache_socket" --unixsocketperm @SOCKET_MODE@ \
    --save '' --appendonly no --dir "@CACHE_DATA@"
}

cache_answers() {
  client="$(cache_client)"
  [ "$("$client" -s "$cache_socket" ping)" = PONG ]
}

case "${1:-}" in
  postgres) run_postgres ;;
  postgres-ready) postgres_ready ;;
  postgres-access) reload_access_rules ;;
  postgres-recover) recover_postgres ;;
  psql) shift; exec "$programs/psql" --host "$socket_directory" --no-psqlrc "$@" ;;
  pg_dump|pg_restore) program="$1"; shift; exec "$programs/$program" --host "$socket_directory" "$@" ;;
  postgres-major) carried_major ;;
  cache) run_cache ;;
  cache-ready) wait_until cache_answers || fail "the cache did not answer on $cache_socket" ;;
  cache-ping) client="$(cache_client)"; exec "$client" -s "$cache_socket" ping ;;
  *) fail "usage: data-service postgres|postgres-ready|postgres-access|postgres-recover|psql|pg_dump|pg_restore|postgres-major|cache|cache-ready|cache-ping" ;;
esac
`
