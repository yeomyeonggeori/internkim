package blueclaw

// HostPythonVersion is a version the carried uv has a pinned download for, so
// the two together name the interpreter's bytes. It is the host's only Python:
// a requester's `python3`, every skill environment built on it, and the
// conversion environment capabilityd runs.
const HostPythonVersion = "3.13.13"

const documentModulesTheConversionImports = "import plistlib, platform, xml.etree.ElementTree, " +
	"anydoc, bs4, markdownify, pypdf, pypdfium2"

// HostSetupCommand is one program the package's install step runs.
type HostSetupCommand struct {
	Purpose   string
	Arguments []string
}

// PythonSetupCommands install the interpreter with python3 and python in
// PythonCommandsPath, then build the conversion environment on it from the
// locked requirements. The install step runs them, so capabilityd never
// resolves, downloads or builds anything, and nothing a requester runs fetches
// an interpreter.
//
// --default is what writes the unversioned python3 and python; uv 0.11 calls it
// a preview, so the preview is asked for by name rather than warned about.
func (layout CompanyHostLayout) PythonSetupCommands() []HostSetupCommand {
	resolver := layout.BinaryPath(PackageResolverName)
	return []HostSetupCommand{
		{
			Purpose: "install CPython " + HostPythonVersion + " in " + layout.PythonRoot() + " as " + layout.PythonPath(),
			Arguments: []string{
				"env", "UV_PYTHON_INSTALL_DIR=" + layout.PythonRoot(), "UV_PYTHON_BIN_DIR=" + layout.PythonCommandsPath(),
				resolver, "--no-cache", "python", "install", "--default", "--preview-features", "python-install-default",
				HostPythonVersion,
			},
		},
		{
			Purpose: "make " + layout.DocumentVirtualEnvironmentPath() + " on " + layout.PythonPath(),
			Arguments: []string{
				resolver, "--no-cache", "venv", "--clear", "--no-python-downloads",
				"--python", layout.PythonPath(), layout.DocumentVirtualEnvironmentPath(),
			},
		},
		{
			Purpose: "install the packages " + layout.DocumentRequirementsPath() + " locks",
			Arguments: []string{
				resolver, "--no-cache", "pip", "sync", "--require-hashes", "--no-build",
				"--python", layout.DocumentPythonPath(), layout.DocumentRequirementsPath(),
			},
		},
		{
			Purpose:   "import every module file_read conversion opens",
			Arguments: []string{layout.DocumentPythonPath(), "-c", documentModulesTheConversionImports},
		},
	}
}
