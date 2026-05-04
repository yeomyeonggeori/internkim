package admind

import (
	"encoding/json"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	companionruntime "gitlab.com/eastriver/internkim/internal/companion"
)

func TestCompanionMountRegistryPersistsSanitizedRecords(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	mount := CompanionMountSnapshot{
		MountID:     "mount-1",
		DisplayName: "Work",
		GuestPath:   "/workspace/mounts/Work-mount-1",
		Mode:        companionruntime.MountModeReadWrite,
		Status:      companionruntime.MountStatusOnline,
		CreatedAt:   time.Now().UTC(),
		LastSeenAt:  time.Now().UTC(),
	}

	service.updateCompanionMounts("companion-1", []CompanionMountSnapshot{mount})
	reloadedService := NewService(service.Configuration)
	record := reloadedService.companionMounts["mount-1"]

	if record == nil || record.CompanionID != "companion-1" || record.GuestPath != mount.GuestPath {
		t.Fatalf("unexpected mount record: %+v", record)
	}
}

func TestCompanionMountJobClaimRequiresOwningCompanion(t *testing.T) {
	service := NewService(Configuration{StateDirectory: t.TempDir(), AdminEmailPath: writeTestFile(t, "admin@example.com")})
	service.updateCompanionMounts("owner-companion", []CompanionMountSnapshot{
		{MountID: "mount-1", GuestPath: "/workspace/mounts/work-12345678", Mode: companionruntime.MountModeReadWrite, Status: companionruntime.MountStatusOnline},
	})
	job := &CompanionJob{
		ToolName:      "filesystem.mount.read",
		ResourceScope: capabilities.ResourceScope{Kind: companionruntime.MountResourceScopeKind, Value: "mount-1"},
	}

	service.mutex.Lock()
	ownerCanClaim := service.companionCanClaimMountJobLocked(&CompanionRecord{CompanionID: "owner-companion"}, job)
	otherCanClaim := service.companionCanClaimMountJobLocked(&CompanionRecord{CompanionID: "other-companion"}, job)
	service.mutex.Unlock()

	if !ownerCanClaim {
		t.Fatal("expected owning companion to claim mount job")
	}
	if otherCanClaim {
		t.Fatal("expected other companion not to claim mount job")
	}
}

func TestCompanionMountResourceScopeComesFromInput(t *testing.T) {
	input, errorValue := json.Marshal(map[string]string{"mountID": "mount-1"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	scope := companionMountResourceScope(capabilities.ToolInvokeRequest{
		ToolName: "filesystem.mount.read",
		Input:    input,
	})
	if scope.Kind != companionruntime.MountResourceScopeKind || scope.Value != "mount-1" {
		t.Fatalf("unexpected mount scope: %+v", scope)
	}
}
