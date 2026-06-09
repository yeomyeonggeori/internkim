package tenantruntime

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	ProfileCloudShared            = "cloud-shared"
	ProfileEdgeAppliance          = "edge-appliance"
	ContainerRuntimeSystemdNspawn = "systemd-nspawn"
)

type Manifest struct {
	TenantID           string             `json:"tenantID"`
	DisplayName        string             `json:"displayName"`
	AssignedHost       string             `json:"assignedHost"`
	PublicURL          string             `json:"publicURL"`
	Profile            string             `json:"profile"`
	QuotaPlan          string             `json:"quotaPlan"`
	ContainerName      string             `json:"containerName"`
	ContainerRuntime   string             `json:"containerRuntime"`
	MattermostInstance MattermostInstance `json:"mattermostInstance"`
	BackupPolicy       BackupPolicy       `json:"backupPolicy"`
}

type MattermostInstance struct {
	PublicURL    string `json:"publicURL"`
	InternalURL  string `json:"internalURL"`
	Port         int    `json:"port"`
	DatabaseName string `json:"databaseName"`
}

type BackupPolicy struct {
	IsEnabled      bool          `json:"isEnabled"`
	Interval       time.Duration `json:"interval"`
	MirrorHost     string        `json:"mirrorHost"`
	RetentionCount int           `json:"retentionCount"`
}

type RuntimePaths struct {
	TenantRootPath        string
	ContainerRootPath     string
	InternKimPath         string
	InternKimSecretsPath  string
	BlueclawRootPath      string
	BlueclawWorkspacePath string
	BackupPath            string
}

var tenantIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`)

func NewCloudSharedManifest(tenantID string, displayName string, assignedHost string, publicURL string, mirrorHost string) (Manifest, error) {
	normalizedTenantID := strings.TrimSpace(strings.ToLower(tenantID))
	if !tenantIDPattern.MatchString(normalizedTenantID) {
		return Manifest{}, errors.New("tenant id must be lowercase letters, numbers, and dashes")
	}
	return Manifest{
		TenantID:         normalizedTenantID,
		DisplayName:      strings.TrimSpace(displayName),
		AssignedHost:     strings.TrimSpace(assignedHost),
		PublicURL:        strings.TrimSpace(publicURL),
		Profile:          ProfileCloudShared,
		QuotaPlan:        "poc-free",
		ContainerName:    "internkim-" + normalizedTenantID,
		ContainerRuntime: ContainerRuntimeSystemdNspawn,
		BackupPolicy: BackupPolicy{
			IsEnabled:      true,
			Interval:       6 * time.Hour,
			MirrorHost:     strings.TrimSpace(mirrorHost),
			RetentionCount: 28,
		},
	}, nil
}

func NewEdgeApplianceManifest(tenantID string, displayName string, deviceID string, publicURL string) (Manifest, error) {
	manifest, errorValue := NewCloudSharedManifest(tenantID, displayName, deviceID, publicURL, "")
	if errorValue != nil {
		return Manifest{}, errorValue
	}
	manifest.Profile = ProfileEdgeAppliance
	manifest.QuotaPlan = "edge-standard"
	manifest.ContainerName = ""
	manifest.ContainerRuntime = ""
	manifest.BackupPolicy.MirrorHost = ""
	return manifest, nil
}

func (manifest Manifest) Validate() error {
	if !tenantIDPattern.MatchString(strings.TrimSpace(manifest.TenantID)) {
		return errors.New("tenant id must be lowercase letters, numbers, and dashes")
	}
	switch manifest.Profile {
	case ProfileCloudShared:
		if manifest.ContainerRuntime != ContainerRuntimeSystemdNspawn {
			return errors.New("cloud-shared tenant runtime must use systemd-nspawn")
		}
		if strings.TrimSpace(manifest.ContainerName) == "" {
			return errors.New("cloud-shared tenant container name is required")
		}
	case ProfileEdgeAppliance:
		return nil
	default:
		return errors.New("tenant profile is not supported")
	}
	return nil
}

func BuildRuntimePaths(basePath string, tenantID string) (RuntimePaths, error) {
	manifest, errorValue := NewCloudSharedManifest(tenantID, "", "", "", "")
	if errorValue != nil {
		return RuntimePaths{}, errorValue
	}
	tenantRootPath := filepath.Join(filepath.Clean(basePath), manifest.TenantID)
	return RuntimePaths{
		TenantRootPath:        tenantRootPath,
		ContainerRootPath:     filepath.Join(tenantRootPath, "rootfs"),
		InternKimPath:         filepath.Join(tenantRootPath, "internkim"),
		InternKimSecretsPath:  filepath.Join(tenantRootPath, "internkim", "secrets"),
		BlueclawRootPath:      filepath.Join(tenantRootPath, "blueclaw"),
		BlueclawWorkspacePath: filepath.Join(tenantRootPath, "blueclaw", "workspace"),
		BackupPath:            filepath.Join(tenantRootPath, "backups"),
	}, nil
}

func RuntimePathsOverlap(first RuntimePaths, second RuntimePaths) bool {
	return pathContains(first.TenantRootPath, second.TenantRootPath) || pathContains(second.TenantRootPath, first.TenantRootPath)
}

func pathContains(parentPath string, childPath string) bool {
	cleanParentPath := filepath.Clean(parentPath)
	cleanChildPath := filepath.Clean(childPath)
	if cleanParentPath == cleanChildPath {
		return true
	}
	relativePath, errorValue := filepath.Rel(cleanParentPath, cleanChildPath)
	if errorValue != nil {
		return false
	}
	return relativePath != "." && !strings.HasPrefix(relativePath, "..")
}
