package blueclaw

// The Hangul face the document skills embed into a PDF. Without one fpdf2 falls
// back to DejaVu, which has no Hangul, and writes the file anyway. The package
// carries NanumGothic under the SIL Open Font License 1.1, which allows
// redistribution with the license text; the font file itself is unmodified.
//
// The pin is the google/fonts repository at one commit. The URLs are whole
// literals for tools/verify-vendored-downloads, which reads them out of this
// package.
const (
	documentFontURL        = "https://raw.githubusercontent.com/google/fonts/23e54b51ddffbc7713c583748e3bd86f62b1fa4a/ofl/nanumgothic/NanumGothic-Regular.ttf"
	documentFontLicenseURL = "https://raw.githubusercontent.com/google/fonts/23e54b51ddffbc7713c583748e3bd86f62b1fa4a/ofl/nanumgothic/OFL.txt"
)

// DocumentFontDownload is one pinned file of the font the package carries.
type DocumentFontDownload struct {
	Name   string
	URL    string
	SHA256 string
}

// DocumentFontDownloads is the font and the license that travels with it.
func DocumentFontDownloads() []DocumentFontDownload {
	return []DocumentFontDownload{
		{Name: "NanumGothic", URL: documentFontURL, SHA256: "76f45ef4a6bcff344c837c95a7dcc26e017e38b5846d5ae0cdcb5b86be2e2d31"},
		{Name: "NanumGothic license", URL: documentFontLicenseURL, SHA256: "eeacf16032901d0ed0456876ec77b8f0fda6b3fecec7d972f8543eb602e6c30f"},
	}
}
