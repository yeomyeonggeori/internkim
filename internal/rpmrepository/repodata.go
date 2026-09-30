package rpmrepository

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const xmlDeclaration = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"

var primaryFilePattern = regexp.MustCompile(`^(.*bin/.*|/etc/.*|/usr/lib/sendmail)$`)

type indexedPackage struct {
	metadata  packageMetadata
	fileName  string
	checksum  string
	fileSize  int
	timestamp int64
}

func escape(text string) string {
	var escaped bytes.Buffer
	xml.EscapeText(&escaped, []byte(text))
	return escaped.String()
}

func versionElement(entry indexedPackage) string {
	return fmt.Sprintf(`<version epoch="%d" ver="%s" rel="%s"/>`,
		entry.metadata.epoch, escape(entry.metadata.version), escape(entry.metadata.release))
}

func renderPrimary(entries []indexedPackage) []byte {
	var document bytes.Buffer
	document.WriteString(xmlDeclaration)
	fmt.Fprintf(&document, `<metadata xmlns="http://linux.duke.edu/metadata/common" xmlns:rpm="http://linux.duke.edu/metadata/rpm" packages="%d">`+"\n", len(entries))
	for _, entry := range entries {
		renderPrimaryPackage(&document, entry)
	}
	document.WriteString("</metadata>\n")
	return document.Bytes()
}

func renderPrimaryPackage(document *bytes.Buffer, entry indexedPackage) {
	metadata := entry.metadata
	document.WriteString(`<package type="rpm">` + "\n")
	fmt.Fprintf(document, "  <name>%s</name>\n", escape(metadata.name))
	fmt.Fprintf(document, "  <arch>%s</arch>\n", escape(metadata.architecture))
	fmt.Fprintf(document, "  %s\n", versionElement(entry))
	fmt.Fprintf(document, "  <checksum type=\"sha256\" pkgid=\"YES\">%s</checksum>\n", entry.checksum)
	fmt.Fprintf(document, "  <summary>%s</summary>\n", escape(metadata.summary))
	fmt.Fprintf(document, "  <description>%s</description>\n", escape(metadata.description))
	fmt.Fprintf(document, "  <packager>%s</packager>\n", escape(metadata.packager))
	fmt.Fprintf(document, "  <url>%s</url>\n", escape(metadata.url))
	fmt.Fprintf(document, "  <time file=\"%d\" build=\"%d\"/>\n", entry.timestamp, metadata.buildTime)
	fmt.Fprintf(document, "  <size package=\"%d\" installed=\"%d\" archive=\"%d\"/>\n", entry.fileSize, metadata.installedSize, metadata.archiveSize)
	fmt.Fprintf(document, "  <location href=\"%s\"/>\n", escape(PackagesDirectory+"/"+entry.fileName))
	document.WriteString("  <format>\n")
	fmt.Fprintf(document, "    <rpm:license>%s</rpm:license>\n", escape(metadata.license))
	fmt.Fprintf(document, "    <rpm:vendor>%s</rpm:vendor>\n", escape(metadata.vendor))
	fmt.Fprintf(document, "    <rpm:group>%s</rpm:group>\n", escape(metadata.group))
	fmt.Fprintf(document, "    <rpm:buildhost>%s</rpm:buildhost>\n", escape(metadata.buildHost))
	fmt.Fprintf(document, "    <rpm:sourcerpm>%s</rpm:sourcerpm>\n", escape(metadata.sourceRPM))
	fmt.Fprintf(document, "    <rpm:header-range start=\"%d\" end=\"%d\"/>\n", metadata.headerStart, metadata.headerEnd)
	renderDependencies(document, "provides", metadata.provides)
	renderDependencies(document, "requires", requiresWithoutRPMLibrary(metadata.requires))
	for _, file := range metadata.files {
		if !primaryFilePattern.MatchString(file.path) {
			continue
		}
		fmt.Fprintf(document, "    <file%s>%s</file>\n", fileTypeAttribute(file), escape(file.path))
	}
	document.WriteString("  </format>\n</package>\n")
}

func requiresWithoutRPMLibrary(requires []dependency) []dependency {
	kept := make([]dependency, 0, len(requires))
	for _, entry := range requires {
		if !strings.HasPrefix(entry.name, "rpmlib(") {
			kept = append(kept, entry)
		}
	}
	return kept
}

func renderDependencies(document *bytes.Buffer, element string, entries []dependency) {
	if len(entries) == 0 {
		return
	}
	fmt.Fprintf(document, "    <rpm:%s>\n", element)
	for _, entry := range entries {
		fmt.Fprintf(document, "      <rpm:entry name=\"%s\"", escape(entry.name))
		if comparison := entry.comparison(); comparison != "" {
			epoch, version, release := splitEpochVersionRelease(entry.version)
			fmt.Fprintf(document, " flags=\"%s\" epoch=\"%s\" ver=\"%s\"", comparison, epoch, escape(version))
			if release != "" {
				fmt.Fprintf(document, " rel=\"%s\"", escape(release))
			}
		}
		if element == "requires" && entry.isPre {
			document.WriteString(` pre="1"`)
		}
		document.WriteString("/>\n")
	}
	fmt.Fprintf(document, "    </rpm:%s>\n", element)
}

func fileTypeAttribute(file packageFile) string {
	switch {
	case file.isDirectory:
		return ` type="dir"`
	case file.isGhost:
		return ` type="ghost"`
	}
	return ""
}

func renderFileLists(entries []indexedPackage) []byte {
	var document bytes.Buffer
	document.WriteString(xmlDeclaration)
	fmt.Fprintf(&document, `<filelists xmlns="http://linux.duke.edu/metadata/filelists" packages="%d">`+"\n", len(entries))
	for _, entry := range entries {
		fmt.Fprintf(&document, "<package pkgid=\"%s\" name=\"%s\" arch=\"%s\">\n", entry.checksum, escape(entry.metadata.name), escape(entry.metadata.architecture))
		fmt.Fprintf(&document, "  %s\n", versionElement(entry))
		for _, file := range entry.metadata.files {
			fmt.Fprintf(&document, "  <file%s>%s</file>\n", fileTypeAttribute(file), escape(file.path))
		}
		document.WriteString("</package>\n")
	}
	document.WriteString("</filelists>\n")
	return document.Bytes()
}

func renderOther(entries []indexedPackage) []byte {
	var document bytes.Buffer
	document.WriteString(xmlDeclaration)
	fmt.Fprintf(&document, `<otherdata xmlns="http://linux.duke.edu/metadata/other" packages="%d">`+"\n", len(entries))
	for _, entry := range entries {
		fmt.Fprintf(&document, "<package pkgid=\"%s\" name=\"%s\" arch=\"%s\">\n", entry.checksum, escape(entry.metadata.name), escape(entry.metadata.architecture))
		fmt.Fprintf(&document, "  %s\n", versionElement(entry))
		document.WriteString("</package>\n")
	}
	document.WriteString("</otherdata>\n")
	return document.Bytes()
}

type metadataFile struct {
	kind         string
	compressed   []byte
	openChecksum string
	openSize     int
}

func compressMetadata(kind string, document []byte) metadataFile {
	var compressed bytes.Buffer
	writer, _ := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	writer.Write(document)
	writer.Close()
	return metadataFile{kind: kind, compressed: compressed.Bytes(), openChecksum: sha256Hex(document), openSize: len(document)}
}

func (file metadataFile) location() string {
	return "repodata/" + file.kind + ".xml.gz"
}

func renderRepositoryMetadata(files []metadataFile, timestamp int64) []byte {
	var document bytes.Buffer
	document.WriteString(xmlDeclaration)
	document.WriteString(`<repomd xmlns="http://linux.duke.edu/metadata/repo" xmlns:rpm="http://linux.duke.edu/metadata/rpm">` + "\n")
	fmt.Fprintf(&document, "  <revision>%s</revision>\n", strconv.FormatInt(timestamp, 10))
	for _, file := range files {
		fmt.Fprintf(&document, "  <data type=\"%s\">\n", file.kind)
		fmt.Fprintf(&document, "    <checksum type=\"sha256\">%s</checksum>\n", sha256Hex(file.compressed))
		fmt.Fprintf(&document, "    <open-checksum type=\"sha256\">%s</open-checksum>\n", file.openChecksum)
		fmt.Fprintf(&document, "    <location href=\"%s\"/>\n", file.location())
		fmt.Fprintf(&document, "    <timestamp>%d</timestamp>\n", timestamp)
		fmt.Fprintf(&document, "    <size>%d</size>\n", len(file.compressed))
		fmt.Fprintf(&document, "    <open-size>%d</open-size>\n", file.openSize)
		document.WriteString("  </data>\n")
	}
	document.WriteString("</repomd>\n")
	return document.Bytes()
}

func sha256Hex(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}
