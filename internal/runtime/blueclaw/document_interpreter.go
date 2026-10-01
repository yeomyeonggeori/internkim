package blueclaw

// DocumentInterpreterVersion is a version the carried uv has a pinned download
// for, so the two together name the interpreter's bytes.
const DocumentInterpreterVersion = "3.13.13"

const documentModulesTheConversionImports = "import plistlib, platform, xml.etree.ElementTree, " +
	"anydoc, bs4, markdownify, pypdf, pypdfium2"

// HostSetupCommand is one program the package's install step runs.
type HostSetupCommand struct {
	Purpose   string
	Arguments []string
}

// DocumentEnvironmentCommands build the conversion environment from the locked
// requirements. The install step runs them, so capabilityd never resolves,
// downloads or builds anything.
func (layout CompanyHostLayout) DocumentEnvironmentCommands() []HostSetupCommand {
	resolver := layout.BinaryPath(PackageResolverName)
	return []HostSetupCommand{
		{
			Purpose: "make " + layout.DocumentVirtualEnvironmentPath() + " on CPython " + DocumentInterpreterVersion + " in " + layout.DocumentInterpreterRoot(),
			Arguments: []string{
				"env", "UV_PYTHON_INSTALL_DIR=" + layout.DocumentInterpreterRoot(),
				resolver, "--no-cache", "venv", "--clear", "--managed-python",
				"--python", DocumentInterpreterVersion, layout.DocumentVirtualEnvironmentPath(),
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
