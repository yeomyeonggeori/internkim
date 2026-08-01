package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"gitlab.com/eastriver/internkim/internal/mattermostdefaults"
)

const mattermostDatabasePasswordPath = "/root/.internkim/secrets/mm-db-pass"
const mattermostServiceUnitName = "mattermost"

func mattermostDesiredDataSource(databasePassword string) string {
	return fmt.Sprintf("postgres://mmuser:%s@localhost/mattermost?sslmode=disable&connect_timeout=10", databasePassword)
}

// ensureMattermostConfig reconciles, on boot, the Mattermost config invariants
// admind and the whole product depend on: the postgres data source and the
// token/bot/managed-path/internal-connection service settings. A crash that
// resets config.json toward Mattermost's defaults (wrong database name, tokens
// disabled) otherwise takes down login and every server-side Mattermost call
// until a human re-runs setup on the device LAN. mm-db-pass is the source of
// truth for the credential. Drift-gated: a correct, serving instance is left
// untouched.
func (service *Service) ensureMattermostConfig(ctx context.Context) {
	go func() {
		if errorValue := service.reconcileMattermostConfig(ctx); errorValue != nil {
			log.Printf("mattermost config reconcile failed: %v", errorValue)
		}
	}()
}

func (service *Service) reconcileMattermostConfig(ctx context.Context) error {
	configPath := strings.TrimSpace(service.Configuration.MattermostConfigFilePath)
	databasePassword := strings.TrimSpace(readTrimmedFile(mattermostDatabasePasswordPath))
	if configPath == "" || databasePassword == "" {
		return nil
	}
	document, errorValue := os.ReadFile(configPath)
	if errorValue != nil {
		return errorValue
	}
	var configuration map[string]any
	if errorValue := json.Unmarshal(document, &configuration); errorValue != nil {
		return fmt.Errorf("parse mattermost config: %w", errorValue)
	}
	changed := applyMattermostConfigInvariants(configuration, databasePassword)
	if !changed && service.mattermostIsResponding(ctx) {
		return nil
	}
	if changed {
		if errorValue := service.writeMattermostConfigFile(ctx, configPath, configuration); errorValue != nil {
			return errorValue
		}
		log.Printf("mattermost config reconciled: restoring invariants and restarting")
	}
	service.alignMattermostDatabasePassword(ctx, databasePassword)
	return service.restartMattermostService(ctx)
}

func applyMattermostConfigInvariants(configuration map[string]any, databasePassword string) bool {
	changed := false
	sqlSettings := mattermostConfigSection(configuration, "SqlSettings")
	changed = setMattermostStringField(sqlSettings, "DriverName", "postgres") || changed
	changed = setMattermostStringField(sqlSettings, "DataSource", mattermostDesiredDataSource(databasePassword)) || changed
	serviceSettings := mattermostConfigSection(configuration, "ServiceSettings")
	changed = setMattermostBoolField(serviceSettings, "EnableUserAccessTokens", true) || changed
	changed = setMattermostBoolField(serviceSettings, "EnableBotAccountCreation", true) || changed
	changed = setMattermostStringField(serviceSettings, "ManagedResourcePaths", mattermostdefaults.ManagedResourcePathSetting()) || changed
	changed = ensureInternalConnectionsAllowed(serviceSettings) || changed
	return changed
}

func mattermostConfigSection(configuration map[string]any, name string) map[string]any {
	section, ok := configuration[name].(map[string]any)
	if !ok {
		section = map[string]any{}
		configuration[name] = section
	}
	return section
}

func setMattermostStringField(section map[string]any, key, value string) bool {
	if current, ok := section[key].(string); ok && current == value {
		return false
	}
	section[key] = value
	return true
}

func setMattermostBoolField(section map[string]any, key string, value bool) bool {
	if current, ok := section[key].(bool); ok && current == value {
		return false
	}
	section[key] = value
	return true
}

func ensureInternalConnectionsAllowed(serviceSettings map[string]any) bool {
	current, _ := serviceSettings["AllowedUntrustedInternalConnections"].(string)
	hosts := strings.Fields(current)
	present := map[string]bool{}
	for _, host := range hosts {
		present[host] = true
	}
	changed := false
	for _, required := range []string{"127.0.0.1", "localhost"} {
		if !present[required] {
			hosts = append(hosts, required)
			changed = true
		}
	}
	if changed {
		serviceSettings["AllowedUntrustedInternalConnections"] = strings.Join(hosts, " ")
	}
	return changed
}

func (service *Service) writeMattermostConfigFile(ctx context.Context, configPath string, configuration map[string]any) error {
	document, errorValue := json.MarshalIndent(configuration, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.WriteFile(configPath, append(document, '\n'), 0o600); errorValue != nil {
		return errorValue
	}
	if output, errorValue := service.runCommand(ctx, "chown", "mattermost:mattermost", configPath); errorValue != nil {
		log.Printf("chown mattermost config failed: %v: %s", errorValue, strings.TrimSpace(string(output)))
	}
	return nil
}

// alignMattermostDatabasePassword makes the mmuser role's password equal the
// mm-db-pass secret, so config (which is derived from the same secret) and
// postgres always agree even after a crash desynchronises them. The secret is a
// generated "mmpass_<hex>" token; the single-quote escape keeps it safe anyway.
func (service *Service) alignMattermostDatabasePassword(ctx context.Context, databasePassword string) {
	statement := "ALTER USER mmuser WITH PASSWORD '" + strings.ReplaceAll(databasePassword, "'", "''") + "'"
	command := "psql -X -qAt -d postgres -c " + shellQuote(statement)
	if output, errorValue := service.runCommand(ctx, "su", "-", "postgres", "-c", command); errorValue != nil {
		log.Printf("align mattermost db password failed: %v: %s", errorValue, strings.TrimSpace(string(output)))
	}
}

func (service *Service) mattermostIsResponding(ctx context.Context) bool {
	output, errorValue := service.runCommand(ctx, "sh", "-lc", "curl -fsS --max-time 4 http://127.0.0.1:8065/api/v4/system/ping >/dev/null 2>&1 && echo ok || echo no")
	return errorValue == nil && strings.TrimSpace(string(output)) == "ok"
}

func (service *Service) restartMattermostService(ctx context.Context) error {
	if output, errorValue := service.runCommand(ctx, "systemctl", "restart", mattermostServiceUnitName); errorValue != nil {
		return fmt.Errorf("restart mattermost: %s: %w", strings.TrimSpace(string(output)), errorValue)
	}
	return nil
}
