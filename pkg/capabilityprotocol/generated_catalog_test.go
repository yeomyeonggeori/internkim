package capabilityprotocol

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func TestGeneratedCatalogLoadsCanonicalToolDescriptors(t *testing.T) {
	catalog, errorValue := loadGeneratedCatalog(generatedCatalogFiles)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if catalog.protocolVersion == "" || len(catalog.aggregateHash) != 64 {
		t.Fatalf("generated protocol identity is incomplete: %+v", catalog)
	}
	if len(catalog.tools) != 26 {
		t.Fatalf("expected twenty-six generated tool descriptors, got %d", len(catalog.tools))
	}
	if errorValue := ValidateDescriptorSet(catalog.tools); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestGeneratedProtocolVersionCompatibility(t *testing.T) {
	if errorValue := validateGeneratedProtocolVersion(supportedGeneratedProtocolVersion); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, protocolVersion := range []string{"", "0.3.0", "0.4.1", "1.0.0"} {
		if errorValue := validateGeneratedProtocolVersion(protocolVersion); errorValue == nil {
			t.Fatalf("expected protocol version %q to be rejected", protocolVersion)
		}
	}
}

func TestGeneratedToolDescriptorsReturnClones(t *testing.T) {
	names := []string{
		"task_add",
		"task_list",
		"task_definitions",
		"task_update",
		"task_delete",
		"calendar_add",
		"calendar_list",
		"calendar_update",
		"calendar_delete",
		"message_context",
		"message_search",
		"message_send",
		"message_update",
		"message_delete",
		"channel_update",
		"site_serve",
		"site_list",
		"site_unserve",
		"document_read",
		"image_read",
		"browser_open",
		"browser_snapshot",
		"browser_screenshot",
		"browser_click",
		"artifact_review",
		"web_search",
	}
	firstDescriptors := MustGeneratedToolDescriptors(names...)
	if len(firstDescriptors) != 26 {
		t.Fatalf("expected twenty-six tool descriptors, got %d", len(firstDescriptors))
	}
	firstDescriptors[0].InputSchema[0] = 'x'
	firstDescriptors[0].InputIntentSchema[0] = 'x'
	firstDescriptors[0].ResultContract.Effects[0].Effect = "changed"

	secondDescriptors := MustGeneratedToolDescriptors(names...)
	if secondDescriptors[0].InputSchema[0] == 'x' ||
		secondDescriptors[0].InputIntentSchema[0] == 'x' ||
		secondDescriptors[0].ResultContract.Effects[0].Effect == "changed" {
		t.Fatal("generated descriptors share mutable catalog state")
	}
}

func TestGeneratedToolDescriptorsRejectMissingNames(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected missing generated descriptor to panic")
		}
	}()
	MustGeneratedToolDescriptors("task.missing")
}

func TestGeneratedCatalogRejectsCorruptOrExtraReleaseArtifacts(t *testing.T) {
	testCases := []struct {
		name             string
		changeFileSystem func(fstest.MapFS)
		errorFragment    string
	}{
		{
			name: "unsupported protocol version",
			changeFileSystem: func(fileSystem fstest.MapFS) {
				document := string(fileSystem["generated/manifest.json"].Data)
				supportedVersion := `"protocolVersion": "` + supportedGeneratedProtocolVersion + `"`
				document = strings.Replace(document, supportedVersion, `"protocolVersion": "1.0.0"`, 1)
				fileSystem["generated/manifest.json"].Data = []byte(document)
			},
			errorFragment: "protocol version",
		},
		{
			name: "catalog hash",
			changeFileSystem: func(fileSystem fstest.MapFS) {
				fileSystem["generated/capability-tools.json"].Data = []byte("{}")
			},
			errorFragment: "hash does not match",
		},
		{
			name: "schema release artifact hash",
			changeFileSystem: func(fileSystem fstest.MapFS) {
				fileSystem["generated/json-schema/agent-action.schema.json"].Data = []byte("{}")
			},
			errorFragment: "generated schema agent-action: hash does not match",
		},
		{
			name: "aggregate hash",
			changeFileSystem: func(fileSystem fstest.MapFS) {
				document := string(fileSystem["generated/manifest.json"].Data)
				document = strings.Replace(document, `"aggregateHash": "`, `"aggregateHash": "0`, 1)
				fileSystem["generated/manifest.json"].Data = []byte(document)
			},
			errorFragment: "aggregate protocol hash",
		},
		{
			name: "extra file",
			changeFileSystem: func(fileSystem fstest.MapFS) {
				fileSystem["generated/extra.json"] = &fstest.MapFile{Data: []byte("{}")}
			},
			errorFragment: "paths do not match",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fileSystem := copyGeneratedFileSystem(t)
			testCase.changeFileSystem(fileSystem)
			_, errorValue := loadGeneratedCatalog(fileSystem)
			if errorValue == nil || !strings.Contains(errorValue.Error(), testCase.errorFragment) {
				t.Fatalf("expected %q error, got %v", testCase.errorFragment, errorValue)
			}
		})
	}
}

func copyGeneratedFileSystem(t *testing.T) fstest.MapFS {
	t.Helper()
	fileSystem := fstest.MapFS{}
	errorValue := fs.WalkDir(generatedCatalogFiles, "generated", func(filePath string, entry fs.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		if entry.IsDir() {
			return nil
		}
		document, errorValue := fs.ReadFile(generatedCatalogFiles, filePath)
		if errorValue != nil {
			return errorValue
		}
		fileSystem[filePath] = &fstest.MapFile{Data: append([]byte{}, document...)}
		return nil
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return fileSystem
}
