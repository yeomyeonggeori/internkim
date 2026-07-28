package admind

import "testing"

func TestVersionedSubjectKeepsVersionOneAsBareEmail(t *testing.T) {
	if got := versionedSubject("Lee@Dawn.kim", 1); got != "lee@dawn.kim" {
		t.Fatalf("version 1 must be the bare normalized email, got %q", got)
	}
	if got := versionedSubject("lee@dawn.kim", 0); got != "lee@dawn.kim" {
		t.Fatalf("version 0 must also be the bare email, got %q", got)
	}
}

func TestVersionedSubjectSuffixesLaterVersions(t *testing.T) {
	if got := versionedSubject("lee@dawn.kim", 2); got != "lee@dawn.kim|v2" {
		t.Fatalf("expected v2 suffix, got %q", got)
	}
	if versionedSubject("lee@dawn.kim", 2) == versionedSubject("lee@dawn.kim", 3) {
		t.Fatal("different versions must produce different subjects")
	}
}

func TestBuzzIdentityVersionDefaultsToOneAndBumps(t *testing.T) {
	service := newIdentityVaultService(t)
	if version := service.buzzIdentityVersion("user-abc"); version != 1 {
		t.Fatalf("expected default version 1, got %d", version)
	}
	next, errorValue := service.bumpBuzzIdentityVersion("user-abc")
	if errorValue != nil {
		t.Fatalf("bump: %v", errorValue)
	}
	if next != 2 {
		t.Fatalf("expected 2 after bump, got %d", next)
	}
	if version := service.buzzIdentityVersion("user-abc"); version != 2 {
		t.Fatalf("expected persisted version 2, got %d", version)
	}
}
