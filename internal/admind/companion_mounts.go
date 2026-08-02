package admind

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	companionruntime "gitlab.com/eastriver/internkim/internal/companion"
)

var companionMountPersistenceMutex sync.Mutex

type CompanionMountRecord struct {
	MountID     string    `json:"mountID"`
	CompanionID string    `json:"companionID"`
	DisplayName string    `json:"displayName"`
	GuestPath   string    `json:"guestPath"`
	Mode        string    `json:"mode"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	LastSeenAt  time.Time `json:"lastSeenAt"`
	RevokedAt   time.Time `json:"revokedAt,omitempty"`
}

type CompanionMountSnapshot struct {
	MountID     string    `json:"mountID"`
	DisplayName string    `json:"displayName"`
	GuestPath   string    `json:"guestPath"`
	Mode        string    `json:"mode"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	LastSeenAt  time.Time `json:"lastSeenAt"`
}

func (service *Service) updateCompanionMounts(companionID string, mounts []CompanionMountSnapshot) error {
	if len(mounts) == 0 {
		return nil
	}
	now := time.Now().UTC()
	service.mutex.Lock()
	for _, mount := range mounts {
		if strings.TrimSpace(mount.MountID) == "" {
			continue
		}
		service.companionMounts[mount.MountID] = companionMountRecord(companionID, mount, now)
	}
	service.mutex.Unlock()
	return service.saveCompanionMounts()
}

func (service *Service) updateCompanionMountsFromJob(companionID string, jobID string, response *capabilities.ToolInvokeResponse) error {
	if response == nil {
		return nil
	}
	switch response.ToolName {
	case "filesystem_mount_create", "filesystem_mount_pause", "filesystem_mount_resume", "filesystem_mount_revoke", "filesystem_mount_status":
		var mount CompanionMountSnapshot
		if errorValue := json.Unmarshal(response.Result, &mount); errorValue == nil && mount.MountID != "" {
			return service.updateCompanionMounts(companionID, []CompanionMountSnapshot{mount})
		}
	case "filesystem_mount_list":
		var document struct {
			Mounts []CompanionMountSnapshot `json:"mounts"`
		}
		if errorValue := json.Unmarshal(response.Result, &document); errorValue == nil {
			return service.updateCompanionMounts(companionID, document.Mounts)
		}
	}
	_ = jobID
	return nil
}

func companionMountRecord(companionID string, mount CompanionMountSnapshot, now time.Time) *CompanionMountRecord {
	status := firstNonEmpty(mount.Status, companionruntime.MountStatusOnline)
	record := &CompanionMountRecord{
		MountID:     strings.TrimSpace(mount.MountID),
		CompanionID: companionID,
		DisplayName: strings.TrimSpace(mount.DisplayName),
		GuestPath:   strings.TrimSpace(mount.GuestPath),
		Mode:        firstNonEmpty(mount.Mode, companionruntime.MountModeReadWrite),
		Status:      status,
		CreatedAt:   firstNonZeroTime(mount.CreatedAt, now),
		LastSeenAt:  firstNonZeroTime(mount.LastSeenAt, now),
	}
	if status == companionruntime.MountStatusRevoked {
		record.RevokedAt = now
	}
	return record
}

func companionMountResourceScope(request capabilities.ToolInvokeRequest) capabilities.ResourceScope {
	if request.ResourceScope.Kind == companionruntime.MountResourceScopeKind && request.ResourceScope.Value != "" {
		return request.ResourceScope
	}
	var input struct {
		MountID string `json:"mountID"`
	}
	if errorValue := json.Unmarshal(request.Input, &input); errorValue != nil {
		return capabilities.ResourceScope{Kind: companionruntime.MountResourceScopeKind}
	}
	return capabilities.ResourceScope{Kind: companionruntime.MountResourceScopeKind, Value: strings.TrimSpace(input.MountID)}
}

func (service *Service) validateCompanionMountRequest(request capabilities.ToolInvokeRequest) error {
	if !capabilities.IsToolInNamespace(request.ToolName, "filesystem") {
		return nil
	}
	switch request.ToolName {
	case "filesystem_mount_create", "filesystem_mount_list", "filesystem_mount_status":
		return nil
	}
	mountID := companionMountResourceScope(request).Value
	if strings.TrimSpace(mountID) == "" {
		return nil
	}
	service.mutex.Lock()
	defer service.mutex.Unlock()
	mount := service.companionMounts[mountID]
	if mount == nil || mount.Status == companionruntime.MountStatusRevoked {
		return errors.New("companion mount is unavailable")
	}
	if mount.Status != companionruntime.MountStatusOnline && request.ToolName != "filesystem_mount_resume" && request.ToolName != "filesystem_mount_revoke" {
		return errors.New("companion mount is not online")
	}
	return nil
}

func (service *Service) companionCanClaimMountJobLocked(companion *CompanionRecord, job *CompanionJob) bool {
	if companion == nil || job == nil || job.ResourceScope.Kind != companionruntime.MountResourceScopeKind || strings.TrimSpace(job.ResourceScope.Value) == "" {
		return true
	}
	mount := service.companionMounts[job.ResourceScope.Value]
	if mount == nil {
		return job.ToolName == "filesystem_mount_create" || job.ToolName == "filesystem_mount_list" || job.ToolName == "filesystem_mount_status"
	}
	return mount.CompanionID == companion.CompanionID && mount.Status != companionruntime.MountStatusRevoked
}

func (service *Service) saveCompanionMounts() error {
	companionMountPersistenceMutex.Lock()
	defer companionMountPersistenceMutex.Unlock()
	document, errorValue := service.companionMountsDocument()
	if errorValue != nil {
		return errorValue
	}
	path := service.companionMountPath()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return errorValue
	}
	temporaryPath := path + ".tmp"
	if errorValue := os.WriteFile(temporaryPath, document, 0o600); errorValue != nil {
		return errorValue
	}
	return os.Rename(temporaryPath, path)
}

func (service *Service) companionMountsDocument() ([]byte, error) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	mounts := []*CompanionMountRecord{}
	for _, mount := range service.companionMounts {
		if mount != nil {
			mounts = append(mounts, mount)
		}
	}
	return json.MarshalIndent(map[string]any{"mounts": mounts}, "", "  ")
}

func (service *Service) loadCompanionMounts() {
	document, errorValue := os.ReadFile(service.companionMountPath())
	if errorValue != nil {
		return
	}
	var state struct {
		Mounts []*CompanionMountRecord `json:"mounts"`
	}
	if errorValue := json.Unmarshal(document, &state); errorValue != nil {
		return
	}
	service.mutex.Lock()
	defer service.mutex.Unlock()
	for _, mount := range state.Mounts {
		if mount != nil && mount.MountID != "" {
			service.companionMounts[mount.MountID] = mount
		}
	}
}

func (service *Service) companionMountPath() string {
	return filepath.Join(service.Configuration.StateDirectory, "companion-mounts.json")
}

func firstNonZeroTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}
