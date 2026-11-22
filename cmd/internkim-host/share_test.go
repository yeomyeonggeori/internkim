package main

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var logFoldersDefinition = regexp.MustCompile(`(?s)create (?:or replace )?function internal\.log_folders\(\)[^$]*\$\$\s*select array\[([^\]]*)\]`)

func TestTheRecordJudgesReadinessOverTheFoldersTheBoxesKeep(t *testing.T) {
	migrations, errorValue := filepath.Glob(filepath.Join("..", "..", "supabase", "migrations", "*.sql"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	slices.Sort(migrations)
	recorded := []string{}
	for _, migration := range migrations {
		contents, errorValue := os.ReadFile(migration)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		for _, defined := range logFoldersDefinition.FindAllStringSubmatch(string(contents), -1) {
			recorded = foldersOf(defined[1])
		}
	}
	if !slices.Equal(recorded, logFolders) {
		t.Fatalf("internal.log_folders() names %v, and the boxes keep %v", recorded, logFolders)
	}
}

func foldersOf(arrayLiteral string) []string {
	folders := []string{}
	for _, quoted := range strings.Split(arrayLiteral, ",") {
		folders = append(folders, strings.Trim(strings.TrimSpace(quoted), "'"))
	}
	return folders
}
