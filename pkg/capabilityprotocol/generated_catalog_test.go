package capabilityprotocol

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func TestGeneratedCatalogLoadsCanonicalTaskDescriptors(t *testing.T) {
	catalog, errorValue := loadGeneratedCatalog(generatedCatalogFiles)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if catalog.protocolVersion == "" || len(catalog.aggregateHash) != 64 {
		t.Fatalf("generated protocol identity is incomplete: %+v", catalog)
	}
	if len(catalog.tools) != 4 {
		t.Fatalf("expected four generated task descriptors, got %d", len(catalog.tools))
	}
	if errorValue := ValidateDescriptorSet(catalog.tools); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestGeneratedToolDescriptorsReturnClones(t *testing.T) {
	names := []string{"task.add", "task.list", "task.update", "task.delete"}
	firstDescriptors := MustGeneratedToolDescriptors(names...)
	if len(firstDescriptors) != 4 {
		t.Fatalf("expected four task descriptors, got %d", len(firstDescriptors))
	}
	firstDescriptors[0].InputSchema[0] = 'x'
	firstDescriptors[0].ResultContract.Effects[0].Effect = "changed"

	secondDescriptors := MustGeneratedToolDescriptors(names...)
	if secondDescriptors[0].InputSchema[0] == 'x' || secondDescriptors[0].ResultContract.Effects[0].Effect == "changed" {
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

func TestGeneratedCatalogRejectsCorruptOrExtraArtifacts(t *testing.T) {
	testCases := []struct {
		name             string
		changeFileSystem func(fstest.MapFS)
		errorFragment    string
	}{
		{
			name: "catalog hash",
			changeFileSystem: func(fileSystem fstest.MapFS) {
				fileSystem["generated/capability-tools.json"].Data = []byte("{}")
			},
			errorFragment: "hash does not match",
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
