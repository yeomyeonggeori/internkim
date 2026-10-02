package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestBlueclawHistoryResetTruncatesOnlyTablesTheMigrationsLeaveBehind(t *testing.T) {
	live := liveTableNames(t)
	for _, tableName := range truncatedTableNames(t, blueclawHistoryResetScript()) {
		if !live[tableName] {
			t.Errorf("the reset truncates %q, which no migration leaves behind; one missing relation fails the whole statement and the reset then clears nothing", tableName)
		}
	}
}

// A reset that cannot finish must not say it did.
func TestBlueclawHistoryResetStopsOnTheFirstFailure(t *testing.T) {
	script := blueclawHistoryResetScript()
	if !strings.Contains(script, "ON_ERROR_STOP=1") {
		t.Error("the reset runs psql without ON_ERROR_STOP, so a failed statement leaves it exiting zero")
	}
	if !strings.HasPrefix(script, "set -euo pipefail") {
		t.Error("the reset script does not stop on a failing command")
	}
}

// liveTableNames reads what blueclaw's migrations create, drop and rename, in
// order, and returns what is left standing. The reset names tables in one
// statement, so a name the migrations no longer leave behind takes the whole
// statement down with it.
func liveTableNames(t *testing.T) map[string]bool {
	t.Helper()
	migrationPaths, errorValue := filepath.Glob(filepath.Join("..", "..", ".dependency", "blueclaw", "migrations", "*.sql"))
	if errorValue != nil || len(migrationPaths) == 0 {
		t.Fatalf("no blueclaw migrations to read: %v", errorValue)
	}
	sort.Strings(migrationPaths)
	created := regexp.MustCompile(`(?i)CREATE TABLE (?:IF NOT EXISTS )?([a-z_][a-z0-9_]*)`)
	dropped := regexp.MustCompile(`(?i)DROP TABLE (?:IF EXISTS )?([a-z_][a-z0-9_]*)`)
	renamed := regexp.MustCompile(`(?i)ALTER TABLE ([a-z_][a-z0-9_]*) RENAME TO ([a-z_][a-z0-9_]*)`)
	live := map[string]bool{}
	for _, migrationPath := range migrationPaths {
		body, errorValue := os.ReadFile(migrationPath)
		if errorValue != nil {
			t.Fatalf("read %s: %v", migrationPath, errorValue)
		}
		for _, match := range created.FindAllStringSubmatch(string(body), -1) {
			live[match[1]] = true
		}
		for _, match := range dropped.FindAllStringSubmatch(string(body), -1) {
			live[match[1]] = false
		}
		for _, match := range renamed.FindAllStringSubmatch(string(body), -1) {
			live[match[1]] = false
			live[match[2]] = true
		}
	}
	return live
}

func truncatedTableNames(t *testing.T, script string) []string {
	t.Helper()
	start := strings.Index(script, "TRUNCATE TABLE")
	if start < 0 {
		t.Fatal("the reset script truncates nothing")
	}
	end := strings.Index(script[start:], "RESTART IDENTITY")
	if end < 0 {
		t.Fatal("the truncate statement does not end where expected")
	}
	names := []string{}
	for _, line := range strings.Split(script[start+len("TRUNCATE TABLE"):start+end], "\n") {
		name := strings.Trim(strings.TrimSpace(line), ",")
		if name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		t.Fatal("the truncate statement names no table")
	}
	return names
}
func TestBlueclawHistoryResetScriptKeepsIdentityAndPolicyState(t *testing.T) {
	script := blueclawHistoryResetScript()
	forbiddenFragments := []string{
		"TRUNCATE TABLE person",
		"TRUNCATE TABLE person_email",
		"TRUNCATE TABLE platform_account",
		"TRUNCATE TABLE policy_revision",
		"TRUNCATE TABLE policy_channel_rule",
	}
	for _, fragment := range forbiddenFragments {
		if strings.Contains(script, fragment) {
			t.Fatalf("expected reset script to keep %q", fragment)
		}
	}
}

func TestBlueclawHistoryResetScriptLeavesTheMessengerAlone(t *testing.T) {
	script := blueclawHistoryResetScript()
	forbiddenFragments := []string{
		"UPDATE posts",
		"DELETE FROM reactions",
		"DELETE FROM threadmemberships",
		"DELETE FROM threads",
	}
	for _, fragment := range forbiddenFragments {
		if strings.Contains(script, fragment) {
			t.Fatalf("expected reset script to omit %q", fragment)
		}
	}
}
