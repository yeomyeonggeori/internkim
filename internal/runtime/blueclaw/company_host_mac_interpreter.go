package blueclaw

// The Python the document skills run under, on a Mac.
//
// Debian's answer is the distribution's python3 and the .deb bounds itself to
// the minor version its venv was resolved against. Homebrew's answer would have
// been `depends_on "python@3.13"`, and it does not work: Homebrew's python@3.13
// bottle for macOS 26 links pyexpat against a /usr/lib/libexpat.1.dylib newer
// than macOS 26.1 ships, so on 26.1 the module will not load —
//
//	dlopen(…/pyexpat.cpython-313-darwin.so): Symbol not found:
//	_XML_SetAllocTrackerActivationThreshold
//	Expected in: /usr/lib/libexpat.1.dylib
//
// — and with it go plistlib, xml.etree, platform.mac_ver() and therefore every
// document skill that opens a .docx or an .xlsx. python@3.13 does not depend on
// the expat formula (`brew deps python@3.13` names ca-certificates, mpdecimal,
// openssl@3, readline, sqlite and xz), so it takes whichever libexpat the
// running macOS holds, and a bottle built on a newer point release than the one
// installing it is broken by construction.
//
// So the package carries its own, the way it already carries bun, uv, moli,
// agent-browser and versitygw: a pinned, relocatable CPython from
// python-build-standalone, which is where uv gets the interpreters it manages.
// It costs about 20 MB and it is the difference between a Mac host whose
// documents work and one whose every .docx write fails on an import.

// MacDocumentInterpreterPin is the interpreter the macOS keg carries. It is
// pinned by URL and checksum here rather than resolved by uv at build time,
// because a release that resolves is a release that can differ from the one
// before it.
type MacDocumentInterpreterDownload struct {
	Version string
	URL     string
	SHA256  string
	// DirectoryInsideArchive is what the tarball's single top-level directory
	// is called; the keg keeps it under that name.
	DirectoryInsideArchive string
}

const (
	macDocumentInterpreterVersion = "3.13.15+20260901"
	macDocumentInterpreterRelease = "20260901"

	// The directory the interpreter lands in inside the keg, and the name the
	// venv's pyvenv.cfg will point its `home` at.
	macDocumentInterpreterDirectoryName = "python"
)

// MacDocumentInterpreter is the pin. `tools/verify-vendored-downloads` reads
// this file for the URL it probes, so a release whose upstream has withdrawn a
// file is found before a Mac is.
func MacDocumentInterpreter() MacDocumentInterpreterDownload {
	return MacDocumentInterpreterDownload{
		Version: macDocumentInterpreterVersion,
		URL: "https://github.com/astral-sh/python-build-standalone/releases/download/" +
			macDocumentInterpreterRelease +
			"/cpython-3.13.15%2B" + macDocumentInterpreterRelease + "-aarch64-apple-darwin-install_only_stripped.tar.gz",
		SHA256:                 "d3904bd6a072246e07aa0bdadee9a14e80521e42a943c0848059feb16a2816dc",
		DirectoryInsideArchive: macDocumentInterpreterDirectoryName,
	}
}

// MacDocumentInterpreterPath is the interpreter the keg carries, which is what
// the document virtualenv is resolved against and what its pyvenv.cfg names.
func (layout CompanyHostLayout) MacDocumentInterpreterPath() string {
	return layout.LibraryRoot + "/" + macDocumentInterpreterDirectoryName + "/bin/python3.13"
}

// DocumentModulesTheSkillsOpen is what a document skill imports before it
// imports anything of its own. It is checked three times against one list: by
// the release, against the interpreter it is about to put in the keg; by the
// formula's own `brew test`, against the virtualenv the install built; and by
// tools/test-macos-install, against what actually landed. A Python that loses
// one of them loses it silently until somebody asks for a document.
const documentModulesTheSkillsOpen = "import plistlib, platform, xml.etree.ElementTree, " +
	"docx, openpyxl, fpdf, pptx, lxml, PIL, pypdf, yaml, xlsxwriter, fontTools"

// DocumentModulesTheSkillsOpen is the statement above, for anything outside this
// package that has to run the same check.
func DocumentModulesTheSkillsOpen() string {
	return documentModulesTheSkillsOpen
}

// MacDocumentWheelDirectoryName is where the keg keeps the wheels the release
// resolved, which is what lets the install build a virtualenv without reaching
// the network and without resolving anything a second time.
func MacDocumentWheelDirectoryName() string {
	return documentWheelDirectoryName
}
