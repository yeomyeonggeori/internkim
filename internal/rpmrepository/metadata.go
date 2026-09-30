package rpmrepository

import (
	"fmt"
	"strings"
)

const (
	tagName                 = 1000
	tagVersion              = 1001
	tagRelease              = 1002
	tagEpoch                = 1003
	tagSummary              = 1004
	tagDescription          = 1005
	tagBuildTime            = 1006
	tagBuildHost            = 1007
	tagSize                 = 1009
	tagVendor               = 1011
	tagLicense              = 1014
	tagPackager             = 1015
	tagGroup                = 1016
	tagURL                  = 1020
	tagArchitecture         = 1022
	tagFileModes            = 1030
	tagFileFlags            = 1037
	tagSourceRPM            = 1044
	tagProvideName          = 1047
	tagRequireFlags         = 1048
	tagRequireName          = 1049
	tagRequireVersion       = 1050
	tagProvideFlags         = 1112
	tagProvideVersion       = 1113
	tagDirectoryIndex       = 1116
	tagBaseNames            = 1117
	tagDirectoryNames       = 1118
	signatureTagSize        = 1000
	signatureTagPayloadSize = 1007

	fileModeDirectoryMask = 0o170000
	fileModeDirectory     = 0o040000
	fileFlagGhost         = 1 << 6

	senseLess    = 1 << 1
	senseGreater = 1 << 2
	senseEqual   = 1 << 3
	sensePrepare = 1<<6 | 1<<9 | 1<<10
)

type dependency struct {
	name    string
	flags   uint64
	version string
	isPre   bool
}

type packageFile struct {
	path        string
	isDirectory bool
	isGhost     bool
}

type packageMetadata struct {
	name, architecture, version, release  string
	epoch                                 uint64
	summary, description, packager        string
	url, license, vendor, group           string
	buildHost, sourceRPM                  string
	buildTime, installedSize, archiveSize uint64
	headerStart, headerEnd                int
	provides, requires                    []dependency
	files                                 []packageFile
}

func readMetadata(contents []byte) (packageMetadata, error) {
	layout, main, errorValue := splitPackage(contents)
	if errorValue != nil {
		return packageMetadata{}, errorValue
	}
	start := len(contents) - len(layout.payload) - len(layout.header)
	metadata := packageMetadata{
		name:          main.text(tagName),
		architecture:  main.text(tagArchitecture),
		version:       main.text(tagVersion),
		release:       main.text(tagRelease),
		epoch:         main.integer(tagEpoch),
		summary:       main.text(tagSummary),
		description:   main.text(tagDescription),
		packager:      main.text(tagPackager),
		url:           main.text(tagURL),
		license:       main.text(tagLicense),
		vendor:        main.text(tagVendor),
		group:         main.text(tagGroup),
		buildHost:     main.text(tagBuildHost),
		sourceRPM:     main.text(tagSourceRPM),
		buildTime:     main.integer(tagBuildTime),
		installedSize: main.integer(tagSize),
		archiveSize:   layout.signature.integer(signatureTagPayloadSize),
		headerStart:   start,
		headerEnd:     start + len(layout.header),
		provides:      dependencies(main, tagProvideName, tagProvideFlags, tagProvideVersion),
		requires:      dependencies(main, tagRequireName, tagRequireFlags, tagRequireVersion),
		files:         packageFiles(main),
	}
	if metadata.name == "" || metadata.version == "" || metadata.architecture == "" {
		return packageMetadata{}, fmt.Errorf("the rpm header names no name, version and architecture")
	}
	return metadata, nil
}

func dependencies(main header, nameTag int, flagTag int, versionTag int) []dependency {
	names := main.strings(nameTag)
	flags := main.integers(flagTag)
	versions := main.strings(versionTag)
	found := make([]dependency, 0, len(names))
	for index, name := range names {
		entry := dependency{name: name}
		if index < len(flags) {
			entry.flags = flags[index]
			entry.isPre = flags[index]&sensePrepare != 0
		}
		if index < len(versions) {
			entry.version = versions[index]
		}
		found = append(found, entry)
	}
	return found
}

func packageFiles(main header) []packageFile {
	baseNames := main.strings(tagBaseNames)
	directoryNames := main.strings(tagDirectoryNames)
	directoryIndexes := main.integers(tagDirectoryIndex)
	modes := main.integers(tagFileModes)
	flags := main.integers(tagFileFlags)
	files := make([]packageFile, 0, len(baseNames))
	for index, baseName := range baseNames {
		if index >= len(directoryIndexes) || int(directoryIndexes[index]) >= len(directoryNames) {
			continue
		}
		entry := packageFile{path: directoryNames[directoryIndexes[index]] + baseName}
		if index < len(modes) {
			entry.isDirectory = modes[index]&fileModeDirectoryMask == fileModeDirectory
		}
		if index < len(flags) {
			entry.isGhost = flags[index]&fileFlagGhost != 0
		}
		files = append(files, entry)
	}
	return files
}

func (entry dependency) comparison() string {
	switch entry.flags & (senseLess | senseGreater | senseEqual) {
	case senseLess:
		return "LT"
	case senseGreater:
		return "GT"
	case senseEqual:
		return "EQ"
	case senseLess | senseEqual:
		return "LE"
	case senseGreater | senseEqual:
		return "GE"
	}
	return ""
}

func splitEpochVersionRelease(combined string) (string, string, string) {
	epoch := "0"
	if before, after, found := strings.Cut(combined, ":"); found {
		epoch, combined = before, after
	}
	version, release, found := strings.Cut(combined, "-")
	if !found {
		return epoch, version, ""
	}
	return epoch, version, release
}
