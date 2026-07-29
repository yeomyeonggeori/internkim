package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadBuzzDatabaseURLStripsEnvironmentFilePrefix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "buzz-relay-db")
	if errorValue := os.WriteFile(path, []byte("DATABASE_URL=postgres://buzz:secret@localhost/buzz?sslmode=disable\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	value, errorValue := readBuzzDatabaseURL(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if value != "postgres://buzz:secret@localhost/buzz?sslmode=disable" {
		t.Fatalf("unexpected url: %q", value)
	}
}

func TestReadBuzzDatabaseURLExtractsOnlyDatabaseLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "buzz-relay-db")
	document := "DATABASE_URL=postgres://buzz:secret@localhost/buzz?sslmode=disable\nRELAY_OWNER_PUBKEY=67ac03f0\n"
	if errorValue := os.WriteFile(path, []byte(document), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	value, errorValue := readBuzzDatabaseURL(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if value != "postgres://buzz:secret@localhost/buzz?sslmode=disable" {
		t.Fatalf("second line leaked into url: %q", value)
	}
}

func TestReadBuzzDatabaseURLRejectsEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty")
	if errorValue := os.WriteFile(path, []byte("DATABASE_URL=\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := readBuzzDatabaseURL(path); errorValue == nil {
		t.Fatal("expected error for empty database url")
	}
}
