package releaseset

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

const ManifestVersion = 2

type Manifest struct {
	capabilityprotocol.ProtocolIdentity
	ManifestVersion         int                  `json:"manifestVersion"`
	ReleaseID               string               `json:"releaseID"`
	Channel                 string               `json:"channel"`
	CreatedAt               string               `json:"createdAt"`
	MinimumInstallerVersion string               `json:"minimumInstallerVersion"`
	Components              map[string]Component `json:"components"`
	Signature               string               `json:"signature,omitempty"`
}

type Component struct {
	Name         string `json:"name"`
	Revision     string `json:"revision"`
	SHA256       string `json:"sha256"`
	Size         int64  `json:"size"`
	BlobPath     string `json:"blobPath"`
	RestartGroup string `json:"restartGroup"`
	HealthCheck  string `json:"healthCheck"`
}

type StablePointer struct {
	ReleaseID   string `json:"releaseID"`
	ManifestURL string `json:"manifestURL"`
	UpdatedAt   string `json:"updatedAt"`
}

type ChannelHistory struct {
	Channel   string                `json:"channel"`
	Entries   []ChannelHistoryEntry `json:"entries"`
	UpdatedAt string                `json:"updatedAt"`
}

type ChannelHistoryEntry struct {
	ReleaseID   string `json:"releaseID"`
	ManifestURL string `json:"manifestURL"`
	CreatedAt   string `json:"createdAt"`
}

func NewManifest(releaseID string, channel string, components map[string]Component) Manifest {
	return Manifest{
		ProtocolIdentity:        capabilityprotocol.GeneratedProtocolIdentity(),
		ManifestVersion:         ManifestVersion,
		ReleaseID:               strings.TrimSpace(releaseID),
		Channel:                 firstNonEmpty(channel, "stable"),
		CreatedAt:               time.Now().UTC().Format(time.RFC3339),
		MinimumInstallerVersion: "1",
		Components:              components,
	}
}

func (manifest Manifest) Validate() error {
	if manifest.ManifestVersion != ManifestVersion {
		return fmt.Errorf("unsupported release manifest version %d", manifest.ManifestVersion)
	}
	if errorValue := manifest.ProtocolIdentity.Validate(); errorValue != nil {
		return fmt.Errorf("release protocol identity is invalid: %w", errorValue)
	}
	if strings.TrimSpace(manifest.ReleaseID) == "" {
		return errors.New("release id is required")
	}
	if strings.TrimSpace(manifest.Channel) == "" {
		return errors.New("release channel is required")
	}
	if len(manifest.Components) == 0 {
		return errors.New("release components are required")
	}
	for componentName, component := range manifest.Components {
		if errorValue := component.Validate(componentName); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (component Component) Validate(componentName string) error {
	if strings.TrimSpace(component.Name) == "" {
		return fmt.Errorf("release component %q has empty name", componentName)
	}
	if strings.TrimSpace(component.Revision) == "" {
		return fmt.Errorf("release component %q has empty revision", componentName)
	}
	if !isSHA256(component.SHA256) {
		return fmt.Errorf("release component %q has invalid sha256", componentName)
	}
	if component.Size <= 0 {
		return fmt.Errorf("release component %q has invalid size", componentName)
	}
	if strings.TrimSpace(component.BlobPath) == "" {
		return fmt.Errorf("release component %q has empty blob path", componentName)
	}
	return nil
}

func (manifest Manifest) Sign(secret string) (Manifest, error) {
	if strings.TrimSpace(secret) == "" {
		return manifest, nil
	}
	manifest.Signature = ""
	document, errorValue := manifest.CanonicalDocument()
	if errorValue != nil {
		return manifest, errorValue
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(document)
	manifest.Signature = hex.EncodeToString(mac.Sum(nil))
	return manifest, nil
}

func (manifest Manifest) VerifySignature(secret string) error {
	if strings.TrimSpace(secret) == "" {
		return nil
	}
	signature := strings.TrimSpace(manifest.Signature)
	if !isSHA256(signature) {
		return errors.New("release manifest signature is missing or invalid")
	}
	unsignedManifest := manifest
	unsignedManifest.Signature = ""
	document, errorValue := unsignedManifest.CanonicalDocument()
	if errorValue != nil {
		return errorValue
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(document)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return errors.New("release manifest signature mismatch")
	}
	return nil
}

func (manifest Manifest) CanonicalDocument() ([]byte, error) {
	if errorValue := manifest.Validate(); errorValue != nil {
		return nil, errorValue
	}
	orderedComponents := map[string]Component{}
	componentNames := make([]string, 0, len(manifest.Components))
	for componentName := range manifest.Components {
		componentNames = append(componentNames, componentName)
	}
	sort.Strings(componentNames)
	for _, componentName := range componentNames {
		orderedComponents[componentName] = manifest.Components[componentName]
	}
	manifest.Components = orderedComponents
	return json.Marshal(manifest)
}

func isSHA256(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 64 {
		return false
	}
	_, errorValue := hex.DecodeString(value)
	return errorValue == nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
