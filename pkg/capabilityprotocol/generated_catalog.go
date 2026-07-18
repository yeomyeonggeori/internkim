package capabilityprotocol

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
)

const supportedGeneratedProtocolVersion = "0.4.0"

type generatedArtifactReference struct {
	FileName string `json:"fileName"`
	Hash     string `json:"hash"`
}

type generatedSchemaReference struct {
	generatedArtifactReference
	Name string `json:"name"`
}

type generatedProtocolManifest struct {
	AggregateHash         string                     `json:"aggregateHash"`
	CapabilityToolCatalog generatedArtifactReference `json:"capabilityToolCatalog"`
	ProtocolVersion       string                     `json:"protocolVersion"`
	Schemas               []generatedSchemaReference `json:"schemas"`
}

type generatedCapabilityCatalog struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Tools           []Descriptor `json:"tools"`
}

type loadedGeneratedCatalog struct {
	aggregateHash   string
	protocolVersion string
	tools           []Descriptor
}

//go:embed generated
var generatedCatalogFiles embed.FS

var canonicalGeneratedCatalog = mustLoadGeneratedCatalog(generatedCatalogFiles)

func GeneratedProtocolVersion() string {
	return canonicalGeneratedCatalog.protocolVersion
}

func GeneratedAggregateProtocolHash() string {
	return canonicalGeneratedCatalog.aggregateHash
}

func MustGeneratedToolDescriptors(names ...string) []Descriptor {
	descriptors := make([]Descriptor, 0, len(names))
	for _, name := range names {
		descriptor, isFound := generatedToolDescriptor(name)
		if !isFound {
			panic(fmt.Errorf("generated tool descriptor %q is missing", name))
		}
		descriptors = append(descriptors, descriptor)
	}
	return descriptors
}

func generatedToolDescriptor(name string) (Descriptor, bool) {
	for _, descriptor := range canonicalGeneratedCatalog.tools {
		if descriptor.Name == name {
			return cloneDescriptor(descriptor), true
		}
	}
	return Descriptor{}, false
}

func mustLoadGeneratedCatalog(fileSystem fs.FS) loadedGeneratedCatalog {
	catalog, errorValue := loadGeneratedCatalog(fileSystem)
	if errorValue != nil {
		panic(errorValue)
	}
	return catalog
}

func loadGeneratedCatalog(fileSystem fs.FS) (loadedGeneratedCatalog, error) {
	manifestDocument, errorValue := fs.ReadFile(fileSystem, "generated/manifest.json")
	if errorValue != nil {
		return loadedGeneratedCatalog{}, errorValue
	}
	var manifest generatedProtocolManifest
	if errorValue := decodeGeneratedJSON(manifestDocument, &manifest); errorValue != nil {
		return loadedGeneratedCatalog{}, fmt.Errorf("decode generated protocol manifest: %w", errorValue)
	}
	if errorValue := validateGeneratedProtocolVersion(manifest.ProtocolVersion); errorValue != nil {
		return loadedGeneratedCatalog{}, errorValue
	}
	catalogDocument, errorValue := fs.ReadFile(fileSystem, "generated/"+manifest.CapabilityToolCatalog.FileName)
	if errorValue != nil {
		return loadedGeneratedCatalog{}, errorValue
	}
	if errorValue := validateGeneratedArtifacts(fileSystem, manifest, catalogDocument); errorValue != nil {
		return loadedGeneratedCatalog{}, errorValue
	}
	var catalog generatedCapabilityCatalog
	if errorValue := decodeGeneratedJSON(catalogDocument, &catalog); errorValue != nil {
		return loadedGeneratedCatalog{}, fmt.Errorf("decode generated capability catalog: %w", errorValue)
	}
	if catalog.ProtocolVersion != manifest.ProtocolVersion {
		return loadedGeneratedCatalog{}, fmt.Errorf("generated capability catalog protocol version does not match manifest")
	}
	if errorValue := ValidateDescriptorSet(catalog.Tools); errorValue != nil {
		return loadedGeneratedCatalog{}, fmt.Errorf("validate generated capability catalog: %w", errorValue)
	}
	return loadedGeneratedCatalog{
		aggregateHash:   manifest.AggregateHash,
		protocolVersion: manifest.ProtocolVersion,
		tools:           cloneDescriptors(catalog.Tools),
	}, nil
}

func validateGeneratedArtifacts(fileSystem fs.FS, manifest generatedProtocolManifest, catalogDocument []byte) error {
	if manifest.CapabilityToolCatalog.FileName != "capability-tools.json" {
		return fmt.Errorf("generated capability catalog filename is invalid")
	}
	if errorValue := validateGeneratedHash(catalogDocument, manifest.CapabilityToolCatalog.Hash); errorValue != nil {
		return fmt.Errorf("generated capability catalog: %w", errorValue)
	}
	artifactHashes, expectedPaths, errorValue := validateGeneratedSchemaArtifacts(fileSystem, manifest.Schemas)
	if errorValue != nil {
		return errorValue
	}
	artifactHashes = append(artifactHashes, "capability-tool-catalog:"+manifest.CapabilityToolCatalog.FileName+":"+manifest.CapabilityToolCatalog.Hash)
	if calculateGeneratedHash(strings.Join(artifactHashes, "\n")) != manifest.AggregateHash {
		return fmt.Errorf("generated aggregate protocol hash does not match")
	}
	expectedPaths = append(expectedPaths, "generated/"+manifest.CapabilityToolCatalog.FileName, "generated/manifest.json")
	return validateGeneratedPaths(fileSystem, expectedPaths)
}

func validateGeneratedProtocolVersion(protocolVersion string) error {
	if protocolVersion != supportedGeneratedProtocolVersion {
		return fmt.Errorf("generated protocol version %q is unsupported; expected %q", protocolVersion, supportedGeneratedProtocolVersion)
	}
	return nil
}

func validateGeneratedSchemaArtifacts(fileSystem fs.FS, references []generatedSchemaReference) ([]string, []string, error) {
	sortedReferences := append([]generatedSchemaReference{}, references...)
	sort.Slice(sortedReferences, func(leftIndex int, rightIndex int) bool {
		return sortedReferences[leftIndex].Name < sortedReferences[rightIndex].Name
	})
	artifactHashes := make([]string, 0, len(sortedReferences))
	expectedPaths := make([]string, 0, len(sortedReferences))
	seenNames := map[string]bool{}
	seenFiles := map[string]bool{}
	for _, reference := range sortedReferences {
		if errorValue := validateGeneratedSchemaArtifactReference(reference, seenNames, seenFiles); errorValue != nil {
			return nil, nil, errorValue
		}
		schemaPath := "generated/json-schema/" + reference.FileName
		document, errorValue := fs.ReadFile(fileSystem, schemaPath)
		if errorValue != nil {
			return nil, nil, errorValue
		}
		if errorValue := validateGeneratedHash(document, reference.Hash); errorValue != nil {
			return nil, nil, fmt.Errorf("generated schema %s: %w", reference.Name, errorValue)
		}
		artifactHashes = append(artifactHashes, reference.Name+":"+reference.FileName+":"+reference.Hash)
		expectedPaths = append(expectedPaths, schemaPath)
	}
	return artifactHashes, expectedPaths, nil
}

func validateGeneratedSchemaArtifactReference(reference generatedSchemaReference, seenNames map[string]bool, seenFiles map[string]bool) error {
	if strings.TrimSpace(reference.Name) == "" || reference.FileName != reference.Name+".schema.json" || path.Base(reference.FileName) != reference.FileName {
		return fmt.Errorf("generated schema reference is invalid")
	}
	if seenNames[reference.Name] || seenFiles[reference.FileName] {
		return fmt.Errorf("generated schema reference is duplicated")
	}
	seenNames[reference.Name] = true
	seenFiles[reference.FileName] = true
	return nil
}

func validateGeneratedPaths(fileSystem fs.FS, expectedPaths []string) error {
	actualPaths := make([]string, 0)
	errorValue := fs.WalkDir(fileSystem, "generated", func(filePath string, entry fs.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		if !entry.IsDir() {
			actualPaths = append(actualPaths, filePath)
		}
		return nil
	})
	if errorValue != nil {
		return errorValue
	}
	sort.Strings(actualPaths)
	sort.Strings(expectedPaths)
	if strings.Join(actualPaths, "\n") != strings.Join(expectedPaths, "\n") {
		return fmt.Errorf("generated protocol artifact paths do not match manifest")
	}
	return nil
}

func validateGeneratedHash(document []byte, expectedHash string) error {
	if len(expectedHash) != sha256.Size*2 {
		return fmt.Errorf("hash is invalid")
	}
	if _, errorValue := hex.DecodeString(expectedHash); errorValue != nil || strings.ToLower(expectedHash) != expectedHash {
		return fmt.Errorf("hash is invalid")
	}
	if calculateGeneratedHash(string(document)) != expectedHash {
		return fmt.Errorf("hash does not match")
	}
	return nil
}

func calculateGeneratedHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func decodeGeneratedJSON(document []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	if errorValue := decoder.Decode(target); errorValue != nil {
		return errorValue
	}
	if errorValue := decoder.Decode(&struct{}{}); errorValue != io.EOF {
		return fmt.Errorf("document must contain exactly one JSON value")
	}
	return nil
}

func cloneDescriptors(descriptors []Descriptor) []Descriptor {
	clonedDescriptors := make([]Descriptor, len(descriptors))
	for index, descriptor := range descriptors {
		clonedDescriptors[index] = cloneDescriptor(descriptor)
	}
	return clonedDescriptors
}

func cloneDescriptor(descriptor Descriptor) Descriptor {
	descriptor.InputSchema = append(json.RawMessage{}, descriptor.InputSchema...)
	descriptor.OutputSchema = append(json.RawMessage{}, descriptor.OutputSchema...)
	if descriptor.ResultContract != nil {
		descriptor.ResultContract = &ToolResultContract{
			Schema:  append(json.RawMessage{}, descriptor.ResultContract.Schema...),
			Effects: append([]ResourceEffectContract{}, descriptor.ResultContract.Effects...),
		}
	}
	if descriptor.CompletionEvidence != nil {
		completionEvidence := *descriptor.CompletionEvidence
		descriptor.CompletionEvidence = &completionEvidence
	}
	return descriptor
}
