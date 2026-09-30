package blueclaw

import "fmt"

// The Python the document skills run under.
//
// The distribution's python3 is not an answer on any platform. A .deb that
// depended on it had to bound itself to the minor version its venv was built
// against, and a machine whose distribution ships another minor could not
// install it. Homebrew's python@3.13 is worse: its bottle for macOS 26 links
// pyexpat against a /usr/lib/libexpat.1.dylib newer than macOS 26.1 ships, so
// on 26.1 the module will not load —
//
//	dlopen(…/pyexpat.cpython-313-darwin.so): Symbol not found:
//	_XML_SetAllocTrackerActivationThreshold
//	Expected in: /usr/lib/libexpat.1.dylib
//
// — and with it go plistlib, xml.etree, platform.mac_ver() and therefore every
// document skill that opens a .docx or an .xlsx.
//
// So every package carries its own: a pinned, relocatable CPython from
// python-build-standalone, which is where uv gets the interpreters it manages.
// The Mac keg, the arm64 .deb and the amd64 .deb all read the one version below.

// DocumentInterpreterDownload is the interpreter a package carries, pinned by
// URL and checksum rather than resolved by uv at build time, because a release
// that resolves is a release that can differ from the one before it.
type DocumentInterpreterDownload struct {
	Version string
	URL     string
	SHA256  string
	// DirectoryInsideArchive is what the tarball's single top-level directory
	// is called; the package keeps it under that name.
	DirectoryInsideArchive string
}

const (
	documentInterpreterVersion = "3.13.15+20260901"

	// DocumentInterpreterMinor is the minor version the wheels are built for and
	// the name of the executable inside the tree.
	DocumentInterpreterMinor = "3.13"

	// The URLs are whole literals rather than a version spliced into a
	// template, because tools/verify-vendored-downloads finds the URLs it
	// probes by reading them out of this package. A URL assembled from pieces
	// is one nobody asks upstream about, which is exactly the one that dies.
	documentInterpreterDarwinArm64URL = "https://github.com/astral-sh/python-build-standalone/releases/download/20260901/cpython-3.13.15%2B20260901-aarch64-apple-darwin-install_only_stripped.tar.gz"
	documentInterpreterLinuxArm64URL  = "https://github.com/astral-sh/python-build-standalone/releases/download/20260901/cpython-3.13.15%2B20260901-aarch64-unknown-linux-gnu-install_only_stripped.tar.gz"
	documentInterpreterLinuxAmd64URL  = "https://github.com/astral-sh/python-build-standalone/releases/download/20260901/cpython-3.13.15%2B20260901-x86_64-unknown-linux-gnu-install_only_stripped.tar.gz"

	// The directory the interpreter lands in inside a package, and the name the
	// venv's pyvenv.cfg will point its `home` at.
	documentInterpreterDirectoryName = "python"
)

var documentInterpreterChecksumByTarget = map[string]string{
	HostPayloadDarwinArm64: "d3904bd6a072246e07aa0bdadee9a14e80521e42a943c0848059feb16a2816dc",
	HostPayloadLinuxArm64:  "01ce0ce9189feaead3298abf10d4efe998c55a489b3d5d38ca4f83dda7e7977e",
	HostPayloadLinuxAmd64:  "8a689a077337bea6d1c4bc0b7df1d52fcaa28f5f67e50df8bf417c1e3f9d8874",
}

var documentInterpreterURLByTarget = map[string]string{
	HostPayloadDarwinArm64: documentInterpreterDarwinArm64URL,
	HostPayloadLinuxArm64:  documentInterpreterLinuxArm64URL,
	HostPayloadLinuxAmd64:  documentInterpreterLinuxAmd64URL,
}

// DocumentInterpreterForTarget is the pin for one machine. A target nobody
// pinned is an error, because a package with no interpreter writes no documents.
func DocumentInterpreterForTarget(target string) (DocumentInterpreterDownload, error) {
	url, isPinned := documentInterpreterURLByTarget[target]
	if !isPinned {
		return DocumentInterpreterDownload{}, fmt.Errorf("no document interpreter is pinned for %s; the targets are %v", target, hostPayloadTargets())
	}
	return DocumentInterpreterDownload{
		Version:                documentInterpreterVersion,
		URL:                    url,
		SHA256:                 documentInterpreterChecksumByTarget[target],
		DirectoryInsideArchive: documentInterpreterDirectoryName,
	}, nil
}

// DocumentInterpreterTargets is every machine the pin covers, for the tools that
// probe them.
func DocumentInterpreterTargets() []string {
	return hostPayloadTargets()
}

// DocumentModulesTheConversionImports is what file_read_helper.py imports, and
// the standard-library modules a broken CPython loses before it gets that far.
// It is checked three times against one list: by the release, against the
// interpreter it is about to put in the keg; by the formula's own `brew test`,
// against the virtualenv the install built; and by tools/test-macos-install,
// against what actually landed. A Python that loses one of them loses it
// silently until somebody asks for a file to be read.
const documentModulesTheConversionImports = "import plistlib, platform, xml.etree.ElementTree, " +
	"anydoc, bs4, markdownify, pypdf, pypdfium2"

// DocumentModulesTheConversionImports is the statement above, for anything
// outside this package that has to run the same check.
func DocumentModulesTheConversionImports() string {
	return documentModulesTheConversionImports
}

// MacDocumentWheelDirectoryName is where the keg keeps the wheels the release
// resolved, which is what lets the install build a virtualenv without reaching
// the network and without resolving anything a second time.
func MacDocumentWheelDirectoryName() string {
	return documentWheelDirectoryName
}
