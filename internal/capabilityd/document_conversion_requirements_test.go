package capabilityd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var helperThirdPartyModulePackages = map[string]string{
	"anydoc":       "firecrawl-anydoc",
	"bs4":          "beautifulsoup4",
	"markdownify":  "markdownify",
	"pypdf":        "pypdf",
	"pypdfium2":    "pypdfium2",
	"openai":       "openai",
	"PIL":          "pillow",
	"reportlab":    "reportlab",
	"fpdf":         "fpdf2",
	"docx":         "python-docx",
	"pptx":         "python-pptx",
	"openpyxl":     "openpyxl",
	"fontTools":    "fonttools",
	"numpy":        "numpy",
	"requests":     "requests",
	"lxml":         "lxml",
	"yaml":         "pyyaml",
	"dateutil":     "python-dateutil",
	"bs4.element":  "beautifulsoup4",
	"markdown":     "markdown",
	"charset_norm": "charset-normalizer",
}

var importPattern = regexp.MustCompile(`(?m)^\s*(?:from\s+([A-Za-z_][A-Za-z0-9_.]*)\s+import|import\s+([A-Za-z_][A-Za-z0-9_.]*))`)

func TestDocumentConversionRequirementsCoverEveryHelperImport(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	requirementsDocument, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "assets", "document-conversion", "requirements.in"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	declared := map[string]bool{}
	for _, line := range strings.Split(string(requirementsDocument), "\n") {
		name := strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		if name == "" {
			continue
		}
		declared[strings.ToLower(name)] = true
	}

	for _, match := range importPattern.FindAllStringSubmatch(fileReadHelperDocument, -1) {
		moduleName := match[1]
		if moduleName == "" {
			moduleName = match[2]
		}
		rootModule := strings.SplitN(moduleName, ".", 2)[0]
		packageName, isThirdParty := helperThirdPartyModulePackages[rootModule]
		if !isThirdParty {
			continue
		}
		if !declared[packageName] {
			t.Fatalf("file_read_helper.py imports %q but assets/document-conversion/requirements.in does not declare %q; the interpreter capabilityd runs would fail on the device", rootModule, packageName)
		}
	}
}
