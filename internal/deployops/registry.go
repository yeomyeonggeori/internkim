package deployops

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func registryPath(repositoryRootPath string) string {
	return filepath.Join(repositoryRootPath, ".local", "ops", "targets.json")
}

func LoadRegistry(repositoryRootPath string, internKimHomePath string) (TargetRegistry, error) {
	path := registryPath(repositoryRootPath)
	document, errorValue := os.ReadFile(path)
	if errorValue == nil {
		return decodeRegistry(document)
	}
	if !errors.Is(errorValue, os.ErrNotExist) {
		return TargetRegistry{}, errorValue
	}
	return discoverDefaultRegistry(internKimHomePath), nil
}

func SaveRegistry(repositoryRootPath string, registry TargetRegistry) error {
	path := registryPath(repositoryRootPath)
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o755); errorValue != nil {
		return errorValue
	}
	document, errorValue := json.MarshalIndent(normalizeRegistry(registry), "", "\t")
	if errorValue != nil {
		return errorValue
	}
	return os.WriteFile(path, append(document, '\n'), 0o600)
}

func decodeRegistry(document []byte) (TargetRegistry, error) {
	var registry TargetRegistry
	if errorValue := json.Unmarshal(document, &registry); errorValue != nil {
		return TargetRegistry{}, errorValue
	}
	return normalizeRegistry(registry), nil
}

func normalizeRegistry(registry TargetRegistry) TargetRegistry {
	seenTargets := map[string]bool{}
	targets := make([]Target, 0, len(registry.Targets))
	for _, target := range registry.Targets {
		normalizedTarget := normalizeTarget(target)
		if !shouldKeepTarget(normalizedTarget) || seenTargets[normalizedTarget.ID] {
			continue
		}
		seenTargets[normalizedTarget.ID] = true
		targets = append(targets, normalizedTarget)
	}
	sort.SliceStable(targets, func(leftIndex int, rightIndex int) bool {
		return targets[leftIndex].Name < targets[rightIndex].Name
	})
	return TargetRegistry{Targets: targets}
}

func shouldKeepTarget(target Target) bool {
	if target.ID == "" {
		return false
	}
	if target.AdminURL != "" {
		return true
	}
	return target.ResolvedKind() == "poc-container" && target.SSHHost != ""
}

func normalizeTarget(target Target) Target {
	target.Name = strings.TrimSpace(target.Name)
	target.AdminURL = normalizeAdminURL(target.AdminURL)
	target.Kind = strings.TrimSpace(target.Kind)
	target.Profile = strings.TrimSpace(target.Profile)
	target.NodeArgument = strings.TrimSpace(target.NodeArgument)
	target.NodeID = strings.TrimSpace(target.NodeID)
	target.StatePath = strings.TrimSpace(target.StatePath)
	target.SecretSource = strings.TrimSpace(target.SecretSource)
	target.SSHHost = strings.TrimSpace(target.SSHHost)
	target.SSHUser = strings.TrimSpace(target.SSHUser)
	target.SSHProxyCommand = strings.TrimSpace(target.SSHProxyCommand)
	target.Workdir = strings.TrimSpace(target.Workdir)
	target.ImageTag = strings.TrimSpace(target.ImageTag)
	target.ComposeFile = strings.TrimSpace(target.ComposeFile)
	if target.SSHProxyCommand == "" {
		target.SSHProxyCommand = defaultTargetSSHProxyCommand(target)
	}
	if target.Name == "" {
		target.Name = firstNonEmptyTargetName(hostName(target.AdminURL), target.SSHHost)
	}
	target.ID = sanitizeID(target.ID)
	if target.ID == "" {
		target.ID = sanitizeID(target.Name)
	}
	if target.ID == "" {
		target.ID = hashID(target.AdminURL)
	}
	return target
}

func defaultTargetSSHProxyCommand(target Target) string {
	if target.ResolvedKind() != "poc-container" {
		return ""
	}
	hostname := strings.ToLower(strings.TrimSuffix(target.SSHHost, "."))
	if strings.HasPrefix(hostname, "ssh-") && strings.HasSuffix(hostname, ".example.test") {
		return "cloudflared access ssh --hostname %h"
	}
	return ""
}

func firstNonEmptyTargetName(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func discoverDefaultRegistry(internKimHomePath string) TargetRegistry {
	targets := []Target{}
	targets = append(targets, discoverBoardTargets(filepath.Join(internKimHomePath, "devices", "jetson-orin-nano", "boards"), "")...)
	targets = append(targets, discoverBoardTargets(filepath.Join(internKimHomePath, "profiles", "pilot", "devices", "jetson-orin-nano", "boards"), "pilot")...)
	return normalizeRegistry(TargetRegistry{Targets: targets})
}

func discoverBoardTargets(boardsPath string, profile string) []Target {
	entries, errorValue := os.ReadDir(boardsPath)
	if errorValue != nil {
		return nil
	}
	targets := []Target{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		statePath := filepath.Join(boardsPath, entry.Name())
		adminURL := strings.TrimSpace(readStateFile(statePath, "device_url"))
		if adminURL == "" {
			continue
		}
		fleetRole := strings.TrimSpace(readStateFile(statePath, "fleet_role"))
		isAbandonedFleetRegistration := strings.TrimSpace(readStateFile(statePath, "fleet_id")) != "" &&
			fleetRole != "active" && fleetRole != "pending"
		if isAbandonedFleetRegistration {
			continue
		}
		nodeID := strings.TrimSpace(readStateFile(statePath, "node_id"))
		targets = append(targets, Target{
			ID:           targetID(profile, entry.Name(), adminURL),
			Name:         targetName(profile, nodeID, adminURL),
			AdminURL:     adminURL,
			Profile:      profile,
			NodeArgument: entry.Name(),
			NodeID:       nodeID,
			StatePath:    statePath,
			SecretSource: statePath,
		})
	}
	return targets
}

func targetID(profile string, nodeArgument string, adminURL string) string {
	if profile != "" && nodeArgument != "" {
		return sanitizeID(profile + "-" + nodeArgument)
	}
	if nodeArgument != "" {
		return sanitizeID("default-" + nodeArgument)
	}
	return sanitizeID(hostName(adminURL))
}

func targetName(profile string, nodeID string, adminURL string) string {
	if nodeID != "" {
		return nodeID
	}
	if profile != "" {
		return profile + "-" + hostName(adminURL)
	}
	return hostName(adminURL)
}

func readStateFile(statePath string, name string) string {
	document, errorValue := os.ReadFile(filepath.Join(statePath, name))
	if errorValue != nil {
		return ""
	}
	return strings.TrimSpace(string(document))
}

func normalizeAdminURL(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	return "https://" + value
}

func hostName(rawURL string) string {
	parsedURL, errorValue := url.Parse(normalizeAdminURL(rawURL))
	if errorValue != nil || parsedURL.Hostname() == "" {
		return strings.TrimSpace(rawURL)
	}
	return parsedURL.Hostname()
}

func sanitizeID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	builder := strings.Builder{}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			builder.WriteRune(character)
			continue
		}
		if character == '-' || character == '_' {
			builder.WriteRune('-')
		}
	}
	return strings.Trim(builder.String(), "-")
}

func hashID(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])[:12]
}
