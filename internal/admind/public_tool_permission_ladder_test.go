package admind

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

const descriptorSchemaPath = "../../pkg/capabilityprotocol/generated/json-schema/capability-descriptor.schema.json"

func canonicalSideEffectClasses(t *testing.T) []string {
	t.Helper()
	document, errorValue := os.ReadFile(filepath.FromSlash(descriptorSchemaPath))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var schema struct {
		Properties struct {
			SideEffectClass struct {
				Enum []string `json:"enum"`
			} `json:"sideEffectClass"`
		} `json:"properties"`
	}
	if errorValue := json.Unmarshal(document, &schema); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(schema.Properties.SideEffectClass.Enum) == 0 {
		t.Fatalf("%s names no side effect class", descriptorSchemaPath)
	}
	return schema.Properties.SideEffectClass.Enum
}

func TestEverySideEffectClassReachesAPermission(t *testing.T) {
	permissions := map[string]bool{"": true, publicAPIPermissionWrite: true, publicAPIPermissionDelete: true}
	for _, sideEffectClass := range canonicalSideEffectClasses(t) {
		permission := publicToolPermissionForDescriptor(capabilities.Descriptor{SideEffectClass: sideEffectClass})
		if !permissions[permission] {
			t.Errorf("%s reaches %q, which is not a permission", sideEffectClass, permission)
		}
	}
}
