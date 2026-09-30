package blueclaw

import (
	"fmt"
	"net/url"
	"strings"
)

// On a systemd machine the company host runs its own database and its own cache,
// from the server binaries the distribution installs, and never uses the
// distribution's default cluster or cache service. A company server that already
// runs a PostgreSQL or a Redis keeps its data in it; sharing that instance would
// put this company's tables beside theirs and make a package upgrade a change to
// both. Neither service listens on a network address: each answers on a Unix
// socket in a directory only its own group can open, so there is no port to
// collide with and nothing on loopback to guard.
//
// The same two units serve every family. Where the binaries are, which major
// version a cluster was made by, and whether the cache is called redis or valkey
// differ between distributions, and CompanyHostDataServiceScript is the one place
// that knows.
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

	companyHostDataDirectoryMode = "0750"
	companyHostDataSocketMode    = "0660"
	companyHostDatabasePort      = "5432"
	companyHostDatabaseCollation = "C.UTF-8"

	// The extension the memory store needs for embeddings. The migration skips
	// its tables when the server cannot create it, which is a memory without
	// embeddings that nothing reports, so the database unit refuses to come up
	// without it.
	CompanyHostDatabaseRequiredExtension = "vector"
)

// CompanyHostAccountsThatReachTheDatabase are the service accounts given the
// database's group. A task user, which is what the agent's terminal runs as, is
// in none of them and cannot open the socket.
func CompanyHostAccountsThatReachTheDatabase() []string {
	return []string{BlueclawUser}
}

// IsDataService is true of the database and the cache. Restarting one restarts
// everything bound to it, so a package upgrade or an install that restarts the
// whole bundle in one transaction asks systemd to stop and start the same unit
// twice, and the job for each unit bound to it fails.
func (unit CompanyPackageUnit) IsDataService() bool {
	return unit.Name == CompanyHostDatabaseServiceName || unit.Name == CompanyHostCacheServiceName
}

func (layout CompanyHostLayout) OwnsItsDataServices() bool {
	return layout.DatabaseSocketDirectory != ""
}

// DatabaseURL is how a client reaches the company's database on this machine.
// The role and password are the company's own; the address is a socket
// directory when the host owns its database and loopback when a platform
// supervises one.
//
// The socket form names localhost and then overrides it with host=. A URL with
// credentials and an empty host is refused by the url crate sqlx parses with
// ("empty host"), and the ampersand a second parameter would need is the one
// character tools/render-company-runtime's old sed treated as the matched text.
// A Unix socket has no TLS to disable, so there is no second parameter.
func (layout CompanyHostLayout) DatabaseURL(role string, password string, database string) string {
	credentials := url.UserPassword(role, password).String()
	if layout.DatabaseSocketDirectory == "" {
		return fmt.Sprintf("postgres://%s@%s/%s?sslmode=disable", credentials, layout.DatabaseLoopbackAddress, database)
	}
	return fmt.Sprintf("postgres://%s@localhost/%s?host=%s", credentials, database, url.QueryEscape(layout.DatabaseSocketDirectory))
}

// CacheURL is what the messenger is told for REDIS_URL. The socket form is the
// one the client library documents as redis+unix.
func (layout CompanyHostLayout) CacheURL() string {
	if layout.CacheSocketPath == "" {
		return BuzzRelayRedisURL
	}
	return "redis+unix://" + layout.CacheSocketPath
}

func (layout CompanyHostLayout) DataServicePath() string {
	return layout.HelperRoot + "/" + CompanyHostDataServiceProgramName
}

// CompanyHostDataServiceUnits are the two units, systemd's alone: a Mac has
// Homebrew's own services and no counterpart for either.
func CompanyHostDataServiceUnits(layout CompanyHostLayout) []CompanyPackageUnit {
	if !layout.OwnsItsDataServices() {
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

type dataServiceUnitSettings struct {
	Description      string
	Account          string
	StateDirectory   string
	RuntimeDirectory string
	ExecStart        string
	ExecStartPost    string
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
TimeoutStartSec=120
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
		settings.ExecStart, settings.ExecStartPost, settings.StopTimeout)
}

// CompanyHostDataServiceScript is the one program both data units run. It finds
// the server binaries the distribution installed, makes the company's cluster on
// first start, and answers the readiness questions the units and the installer
// ask. It is rendered here rather than kept as a file so that the paths it names
// are the constants the units name.
func CompanyHostDataServiceScript() string {
	replacer := strings.NewReplacer(
		"@DATA@", CompanyHostDatabaseDataPath,
		"@SOCKET_DIRECTORY@", CompanyHostDatabaseSocketDirectory,
		"@SOCKET_MODE@", companyHostDataSocketMode,
		"@PORT@", companyHostDatabasePort,
		"@COLLATION@", companyHostDatabaseCollation,
		"@ROLE@", CompanyHostDatabaseUser,
		"@EXTENSION@", CompanyHostDatabaseRequiredExtension,
		"@CACHE_SOCKET@", CompanyHostCacheSocketPath,
		"@CACHE_DATA@", CompanyHostCacheDataPath,
	)
	return replacer.Replace(companyHostDataServiceTemplate)
}

const companyHostDataServiceTemplate = `#!/bin/sh
set -eu

data="@DATA@"
socket_directory="@SOCKET_DIRECTORY@"
cache_socket="@CACHE_SOCKET@"

fail() {
  echo "internkim-data-service: $1" >&2
  exit 1
}

major_of() {
  "$1/postgres" --version | sed -n 's/^postgres (PostgreSQL) \([0-9][0-9]*\).*/\1/p'
}

binary_directories() {
  for directory in /usr/lib/postgresql/*/bin /usr/lib/postgresql[0-9]*/bin /usr/pgsql-*/bin /usr/bin; do
    if [ -x "$directory/postgres" ]; then echo "$directory"; fi
  done
}

binaries_for_major() {
  for directory in $(binary_directories); do
    if [ "$(major_of "$directory")" = "$1" ]; then echo "$directory"; return 0; fi
  done
  return 1
}

major_offers_the_extension() {
  for directory in /usr/share/postgresql/"$1"/extension /usr/share/postgresql"$1"/extension /usr/share/pgsql/extension /usr/share/postgresql/extension; do
    if [ -r "$directory/@EXTENSION@.control" ]; then return 0; fi
  done
  return 1
}

installed_majors() {
  for directory in $(binary_directories); do
    major_of "$directory"
  done | sort -rn
}

best_installed_major() {
  for major in $(installed_majors); do
    if major_offers_the_extension "$major"; then echo "$major"; return 0; fi
  done
  installed_majors | head -n 1
}

postgres_directory() {
  if [ -r "$data/PG_VERSION" ]; then
    major="$(cat "$data/PG_VERSION")"
    binaries_for_major "$major" || fail "the company database was made by PostgreSQL $major and this machine has no PostgreSQL $major installed; install it again, or restore the version the cluster was made by"
    return
  fi
  major="$(best_installed_major)"
  [ -n "$major" ] || fail "no PostgreSQL server is installed; the package names the one to install"
  binaries_for_major "$major"
}

initialize_cluster() {
  directory="$1"
  if [ -r "$data/PG_VERSION" ]; then return 0; fi
  "$directory/initdb" --pgdata "$data" --username @ROLE@ --auth-local=peer --auth-host=reject \
    --encoding UTF8 --locale @COLLATION@ --data-checksums >/dev/null
  printf '%s\n' \
    'local all @ROLE@ peer' \
    'local all all scram-sha-256' > "$data/pg_hba.conf"
}

run_postgres() {
  directory="$(postgres_directory)"
  initialize_cluster "$directory"
  exec "$directory/postgres" -D "$data" \
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
  directory="$(postgres_directory)"
  "$directory/pg_isready" --quiet --host "$socket_directory"
}

postgres_offers_the_extension() {
  directory="$(postgres_directory)"
  offered="$("$directory/psql" --host "$socket_directory" --username @ROLE@ --dbname postgres --no-psqlrc --tuples-only --no-align \
    --command "select count(*) from pg_available_extensions where name = '@EXTENSION@'")"
  [ "$offered" = 1 ]
}

postgres_ready() {
  wait_until postgres_answers || fail "PostgreSQL did not accept connections on $socket_directory"
  directory="$(postgres_directory)"
  postgres_offers_the_extension || fail "PostgreSQL $(major_of "$directory") has no @EXTENSION@ extension installed, and the memory store needs it; install the pgvector package for that PostgreSQL version"
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
  psql) directory="$(postgres_directory)"; shift; exec "$directory/psql" --host "$socket_directory" --no-psqlrc "$@" ;;
  cache) run_cache ;;
  cache-ready) wait_until cache_answers || fail "the cache did not answer on $cache_socket" ;;
  cache-ping) client="$(cache_client)"; exec "$client" -s "$cache_socket" ping ;;
  *) fail "usage: data-service postgres|postgres-ready|psql|cache|cache-ready|cache-ping" ;;
esac
`
