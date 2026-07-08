package admind

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validSiteDesignMarkdownWithColors(primaryColor string, backgroundColor string) string {
	return validSiteDesignMarkdownWithColorsAndMarker(primaryColor, backgroundColor, "Design")
}

func validSiteDesignMarkdownWithMarker(marker string) string {
	return validSiteDesignMarkdownWithColorsAndMarker("#112233", "#ffffff", marker)
}

func validSiteDesignMarkdownWithColorsAndMarker(primaryColor string, backgroundColor string, marker string) string {
	return "---\n" +
		"colors:\n" +
		"  primary: \"" + primaryColor + "\"\n" +
		"  background: \"" + backgroundColor + "\"\n" +
		"typography:\n" +
		"  heading:\n" +
		"    fontFamily: Inter\n" +
		"  body:\n" +
		"    fontFamily: Inter\n" +
		"rounded:\n" +
		"  md: 8px\n" +
		"spacing:\n" +
		"  md: 16px\n" +
		"components:\n" +
		"  button:\n" +
		"    padding: 12px\n" +
		"---\n\n# " + marker + "\n"
}

func TestParseSiteDesignThemeHappyPath(t *testing.T) {
	document := "---\n" +
		"colors:\n" +
		"  primary: \"#336699\"\n" +
		"  background: \"#FFFFFF\"\n" +
		"  foreground: \"#101010\"\n" +
		"  accent: \"#FF8800\"\n" +
		"typography:\n" +
		"  headline-display:\n" +
		"    fontFamily: ui-serif\n" +
		"  body-md:\n" +
		"    fontFamily: ui-sans-serif\n" +
		"rounded:\n" +
		"  sm: 4px\n" +
		"  md: 8px\n" +
		"  lg: 12px\n" +
		"spacing:\n" +
		"  md: 16px\n" +
		"components:\n" +
		"  button:\n" +
		"    padding: 12px\n" +
		"---\n\n# Title\n"
	theme, errorValue := parseSiteDesignTheme(document)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if theme.PrimaryColor != "#336699" {
		t.Fatalf("primary color = %q", theme.PrimaryColor)
	}
	if theme.BackgroundColor != "#ffffff" {
		t.Fatalf("background color = %q", theme.BackgroundColor)
	}
	if theme.ForegroundColor != "#101010" {
		t.Fatalf("foreground color = %q", theme.ForegroundColor)
	}
	if theme.AccentColor != "#ff8800" {
		t.Fatalf("accent color = %q", theme.AccentColor)
	}
	if theme.HeadingFontFamily != "에이투지체" {
		t.Fatalf("heading font family = %q", theme.HeadingFontFamily)
	}
	if theme.BodyFontFamily != "에이투지체" {
		t.Fatalf("body font family = %q", theme.BodyFontFamily)
	}
	if theme.RadiusValue != "8px" {
		t.Fatalf("radius value = %q", theme.RadiusValue)
	}
}

func TestParseSiteDesignThemeDerivesOptionalColors(t *testing.T) {
	document := "---\n" +
		"colors:\n" +
		"  primary: \"#111111\"\n" +
		"  background: \"#ffffff\"\n" +
		"typography:\n" +
		"  heading:\n" +
		"    fontFamily: Inter\n" +
		"rounded:\n" +
		"  md: 8px\n" +
		"spacing:\n" +
		"  md: 16px\n" +
		"components:\n" +
		"  button:\n" +
		"    padding: 12px\n" +
		"---\n"
	theme, errorValue := parseSiteDesignTheme(document)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if theme.ForegroundColor != "#000000" {
		t.Fatalf("expected foreground derived from white background, got %q", theme.ForegroundColor)
	}
	if theme.AccentColor != "#111111" {
		t.Fatalf("expected accent to fall back to primary, got %q", theme.AccentColor)
	}
	if theme.BodyFontFamily != "Inter" {
		t.Fatalf("expected body font family to reuse the only typography entry, got %q", theme.BodyFontFamily)
	}
}

func TestParseSiteDesignThemeMissingFrontMatterDelimiters(t *testing.T) {
	_, errorValue := parseSiteDesignTheme("not front matter at all")
	if errorValue == nil || !strings.Contains(errorValue.Error(), "must start with YAML front matter") {
		t.Fatalf("expected missing front matter delimiter error, got %v", errorValue)
	}
}

func TestParseSiteDesignThemeUnterminatedFrontMatter(t *testing.T) {
	_, errorValue := parseSiteDesignTheme("---\ncolors:\n  primary: \"#111111\"\n")
	if errorValue == nil || !strings.Contains(errorValue.Error(), "must end with ---") {
		t.Fatalf("expected unterminated front matter error, got %v", errorValue)
	}
}

func TestParseSiteDesignThemeMissingRequiredKeys(t *testing.T) {
	document := "---\ncolors:\n  primary: \"#111111\"\n  background: \"#ffffff\"\n---\n"
	_, errorValue := parseSiteDesignTheme(document)
	if errorValue == nil {
		t.Fatal("expected missing required keys error")
	}
	for _, key := range []string{"typography", "rounded", "spacing", "components"} {
		if !strings.Contains(errorValue.Error(), key) {
			t.Fatalf("expected error to mention missing key %q, got %v", key, errorValue)
		}
	}
}

func TestParseSiteDesignThemeCollectsAllErrorsInOneShot(t *testing.T) {
	document := "---\n" +
		"colors:\n" +
		"  foreground: \"#000000\"\n" +
		"typography:\n" +
		"  heading:\n" +
		"    fontSize: 24px\n" +
		"rounded: not-a-length\n" +
		"---\n"
	_, errorValue := parseSiteDesignTheme(document)
	if errorValue == nil {
		t.Fatal("expected a validation error")
	}
	for _, expectedSubstring := range []string{
		"missing required front matter keys: spacing, components",
		"colors.primary",
		"colors.background",
		"fontFamily",
		"rounded",
	} {
		if !strings.Contains(errorValue.Error(), expectedSubstring) {
			t.Fatalf("expected one-shot error to mention %q, got %v", expectedSubstring, errorValue)
		}
	}
}

func TestParseSiteDesignThemeInvalidPrimaryColor(t *testing.T) {
	document := "---\n" +
		"colors:\n" +
		"  primary: not-a-color\n" +
		"  background: \"#ffffff\"\n" +
		"typography:\n" +
		"  heading:\n" +
		"    fontFamily: Inter\n" +
		"rounded:\n" +
		"  md: 8px\n" +
		"spacing:\n" +
		"  md: 16px\n" +
		"components:\n" +
		"  button:\n" +
		"    padding: 12px\n" +
		"---\n"
	_, errorValue := parseSiteDesignTheme(document)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "colors.primary") {
		t.Fatalf("expected colors.primary validation error, got %v", errorValue)
	}
}

func TestParseSiteDesignThemeTypographyWithoutFontFamily(t *testing.T) {
	document := "---\n" +
		"colors:\n" +
		"  primary: \"#111111\"\n" +
		"  background: \"#ffffff\"\n" +
		"typography:\n" +
		"  heading:\n" +
		"    fontSize: 24px\n" +
		"rounded:\n" +
		"  md: 8px\n" +
		"spacing:\n" +
		"  md: 16px\n" +
		"components:\n" +
		"  button:\n" +
		"    padding: 12px\n" +
		"---\n"
	_, errorValue := parseSiteDesignTheme(document)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "fontFamily") {
		t.Fatalf("expected typography fontFamily error, got %v", errorValue)
	}
}

func TestParseSiteDesignThemeDefaultScaffoldTemplateParses(t *testing.T) {
	document := siteDesignMD(&SiteRecord{Title: "Acme", Slug: "acme"})
	theme, errorValue := parseSiteDesignTheme(document)
	if errorValue != nil {
		t.Fatalf("expected the shipped default DESIGN.md template to parse, got %v", errorValue)
	}
	if theme.PrimaryColor != "#111111" {
		t.Fatalf("primary color = %q", theme.PrimaryColor)
	}
	if theme.BackgroundColor != "#ffffff" {
		t.Fatalf("background color = %q", theme.BackgroundColor)
	}
	if theme.ForegroundColor != "#000000" {
		t.Fatalf("expected foreground derived from white background, got %q", theme.ForegroundColor)
	}
	if theme.AccentColor != "#111111" {
		t.Fatalf("expected accent to fall back to primary, got %q", theme.AccentColor)
	}
	if theme.HeadingFontFamily != "에이투지체" {
		t.Fatalf("heading font family = %q", theme.HeadingFontFamily)
	}
	if theme.BodyFontFamily != "에이투지체" {
		t.Fatalf("body font family = %q", theme.BodyFontFamily)
	}
	if theme.RadiusValue != "8px" {
		t.Fatalf("radius value = %q", theme.RadiusValue)
	}
}

func TestRenderSiteThemeCSSContract(t *testing.T) {
	theme := siteTheme{
		PrimaryColor:      "#111111",
		BackgroundColor:   "#ffffff",
		ForegroundColor:   "#000000",
		AccentColor:       "#ff8800",
		HeadingFontFamily: "ui-serif",
		BodyFontFamily:    "ui-sans-serif",
		RadiusValue:       "8px",
	}
	css := renderSiteThemeCSS(theme)
	expectedLines := []string{
		":root:root {",
		"--primary: #111111;",
		"--primary-foreground: #ffffff;",
		"--background: #ffffff;",
		"--foreground: #000000;",
		"--accent: #ff8800;",
		"--radius: 8px;",
		"--font-heading: ui-serif, \"에이투지체\", Pretendard, system-ui, sans-serif;",
		"--font-body: ui-sans-serif, \"에이투지체\", Pretendard, system-ui, sans-serif;",
		"}",
	}
	for _, expectedLine := range expectedLines {
		if !strings.Contains(css, expectedLine) {
			t.Fatalf("expected theme.css to contain %q, got:\n%s", expectedLine, css)
		}
	}
}

func TestReadableForegroundColorPicksContrastingText(t *testing.T) {
	if readableForegroundColor("#ffffff") != "#000000" {
		t.Fatal("expected black text over a white background")
	}
	if readableForegroundColor("#111111") != "#ffffff" {
		t.Fatal("expected white text over a near-black background")
	}
}

func TestNormalizeSiteDesignHexColorExpandsAndLowercases(t *testing.T) {
	normalizedColor, errorValue := normalizeSiteDesignHexColor("#FFF")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if normalizedColor != "#ffffff" {
		t.Fatalf("expected shorthand hex to expand, got %q", normalizedColor)
	}
	normalizedColor, errorValue = normalizeSiteDesignHexColor("AABBCC")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if normalizedColor != "#aabbcc" {
		t.Fatalf("expected missing # to be added, got %q", normalizedColor)
	}
	if _, errorValue := normalizeSiteDesignHexColor("not-a-color"); errorValue == nil {
		t.Fatal("expected an error for an invalid hex color")
	}
}

func TestSitePublishRendersThemeCSSMatchingDesignDocument(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(siteCreateRequest{Slug: "themed-site", Title: "Themed Site"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.materializeSiteSourceWorkspace(context.Background(), site, nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(site.HostSourcePath, "DESIGN.md"), validSiteDesignMarkdownWithColors("#336699", "#ffffff"))

	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:             site.SiteID,
		SourceBundleBase64: testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat: "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	response := serveSiteRequest(service, "themed-site.device.example.test", "/theme.css")
	if response.Code != 200 {
		t.Fatalf("theme.css status = %d body = %q", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "--primary: #336699;") {
		t.Fatalf("expected theme.css --primary to match DESIGN.md, got %q", response.Body.String())
	}
}

func TestSitePublishRejectsInvalidDesignDocument(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(siteCreateRequest{Slug: "invalid-design", Title: "Invalid Design"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.materializeSiteSourceWorkspace(context.Background(), site, nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(site.HostSourcePath, "DESIGN.md"), "not a design document")

	_, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:             site.SiteID,
		SourceBundleBase64: testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat: "tar.gz",
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "DESIGN.md front matter is invalid") {
		t.Fatalf("expected invalid DESIGN.md rejection, got %v", errorValue)
	}
}

func TestSitePublishInvalidDesignDocumentReportsAllErrorsInOneShot(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(siteCreateRequest{Slug: "fully-invalid-design", Title: "Fully Invalid Design"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.materializeSiteSourceWorkspace(context.Background(), site, nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeFile(t, filepath.Join(site.HostSourcePath, "DESIGN.md"), "---\n"+
		"colors:\n"+
		"  foreground: \"#000000\"\n"+
		"typography:\n"+
		"  heading:\n"+
		"    fontSize: 24px\n"+
		"rounded: not-a-length\n"+
		"---\n")

	_, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:             site.SiteID,
		SourceBundleBase64: testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat: "tar.gz",
	})
	if errorValue == nil {
		t.Fatal("expected publish to reject the fully-invalid DESIGN.md")
	}
	for _, expectedSubstring := range []string{
		"missing required front matter keys: spacing, components",
		"colors.primary",
		"colors.background",
		"fontFamily",
		"rounded",
	} {
		if !strings.Contains(errorValue.Error(), expectedSubstring) {
			t.Fatalf("expected the single publish error to mention %q, got %v", expectedSubstring, errorValue)
		}
	}
}

func TestSitePublishWithoutDesignDocumentSkipsThemeCSS(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(siteCreateRequest{Slug: "legacy-no-design", Title: "Legacy No Design"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.materializeSiteSourceWorkspace(context.Background(), site, nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.Remove(filepath.Join(site.HostSourcePath, "DESIGN.md")); errorValue != nil {
		t.Fatal(errorValue)
	}

	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:             site.SiteID,
		SourceBundleBase64: testSourceBundleBase64(t, site.HostSourcePath),
		SourceBundleFormat: "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if site.Status != SiteStatusPublished {
		t.Fatalf("published status = %q", site.Status)
	}
	if isRegularFile(publishedSiteThemeCSSPath(t, service, site)) {
		t.Fatal("expected no theme.css to be materialized for a site without DESIGN.md")
	}
}

func TestSiteLegacyFreshnessPublishPathIsUnaffectedByTheme(t *testing.T) {
	service, _ := newTestSiteService(t)
	site, errorValue := service.createSiteRecord(siteCreateRequest{
		Slug:        "legacy-freshness",
		Title:       "Legacy Freshness",
		RequestedBy: "owner@example.com",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	sourceWorkspacePath := t.TempDir()
	writeTestSourceBuild(t, sourceWorkspacePath, "freshness publish")

	site, errorValue = service.publishSite(context.Background(), sitePublishRequest{
		SiteID:              site.SiteID,
		RequestedBy:         "owner@example.com",
		Message:             "Publish fresh build without DESIGN.md",
		SourceWorkspacePath: site.SourceWorkspacePath,
		SourceBundleBase64:  testSourceBundleBase64(t, sourceWorkspacePath),
		SourceBundleFormat:  "tar.gz",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	response := serveSiteRequest(service, "legacy-freshness.device.example.test", "/")
	if !strings.Contains(response.Body.String(), "freshness publish") {
		t.Fatalf("published body = %q", response.Body.String())
	}
	if isRegularFile(publishedSiteThemeCSSPath(t, service, site)) {
		t.Fatal("expected no theme.css on the legacy freshness path without DESIGN.md")
	}
}

func publishedSiteThemeCSSPath(t *testing.T, service *Service, site *SiteRecord) string {
	t.Helper()
	return filepath.Join(service.sitePublishedVersionPath(site, site.CurrentVersionID), "frontend", "dist", "theme.css")
}

func TestCatalogFontFamilyOrDefaultReplacesGenericKeywords(t *testing.T) {
	cases := map[string]string{
		"ui-sans-serif": "에이투지체",
		"system-ui":     "에이투지체",
		"Sans-Serif":    "에이투지체",
		"":              "에이투지체",
		"마루부리":          "마루부리",
		"Paperlogy":     "Paperlogy",
	}
	for input, expected := range cases {
		if actual := catalogFontFamilyOrDefault(input); actual != expected {
			t.Fatalf("catalogFontFamilyOrDefault(%q) = %q, expected %q", input, actual, expected)
		}
	}
}

func TestParseSiteDesignStylePreset(t *testing.T) {
	cases := map[string]string{
		"":                      "editorial",
		"style: brutalist":      "brutalist",
		"style: \"soft\"":       "soft",
		"style: PLAYFUL":        "playful",
	}
	for frontMatter, expected := range cases {
		preset, errorValue := parseSiteDesignStylePreset(frontMatter)
		if errorValue != nil || preset != expected {
			t.Fatalf("parseSiteDesignStylePreset(%q) = %q, %v; expected %q", frontMatter, preset, errorValue, expected)
		}
	}
	if _, errorValue := parseSiteDesignStylePreset("style: vaporwave"); errorValue == nil {
		t.Fatal("unknown preset must fail validation")
	}
	css := renderSiteThemeCSS(siteTheme{PrimaryColor: "#111111", BackgroundColor: "#ffffff", ForegroundColor: "#111111", AccentColor: "#111111", HeadingFontFamily: "에이투지체", BodyFontFamily: "에이투지체", RadiusValue: "8px", StylePreset: "brutalist"})
	if !strings.Contains(css, "--hero-background: var(--primary);") || !strings.Contains(css, "--shadow-card: 4px 4px 0 var(--foreground);") {
		t.Fatalf("brutalist preset variables missing from theme css:\n%s", css)
	}
}
