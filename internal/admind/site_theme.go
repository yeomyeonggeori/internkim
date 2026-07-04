package admind

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const siteDesignDocumentPath = "DESIGN.md"
const siteThemeCSSFileName = "theme.css"

var siteDesignFrontMatterRequiredKeyPatterns = map[string]*regexp.Regexp{
	"colors":     regexp.MustCompile(`(?m)^colors\s*:`),
	"typography": regexp.MustCompile(`(?m)^typography\s*:`),
	"rounded":    regexp.MustCompile(`(?m)^rounded\s*:`),
	"spacing":    regexp.MustCompile(`(?m)^spacing\s*:`),
	"components": regexp.MustCompile(`(?m)^components\s*:`),
}

var siteDesignFrontMatterRequiredKeyOrder = []string{"colors", "typography", "rounded", "spacing", "components"}

var siteDesignRoundedKeywordValues = map[string]string{
	"none": "0px",
	"sm":   "4px",
	"md":   "8px",
	"lg":   "12px",
	"xl":   "16px",
	"full": "9999px",
}

var siteDesignHexColorPattern = regexp.MustCompile(`^#?([0-9a-fA-F]{6}|[0-9a-fA-F]{3})$`)
var siteDesignLengthPattern = regexp.MustCompile(`^([0-9]*\.?[0-9]+)(px|rem)?$`)

type siteTheme struct {
	PrimaryColor      string
	BackgroundColor   string
	ForegroundColor   string
	AccentColor       string
	HeadingFontFamily string
	BodyFontFamily    string
	RadiusValue       string
}

// applySiteDesignTheme renders DESIGN.md's front matter into theme.css at the
// root of a published frontend dist. A DESIGN.md that does not exist is a
// legacy site and publishes without a theme.css. A DESIGN.md that exists but
// carries invalid or unparseable front matter fails the publish outright —
// the model authors DESIGN.md as the site's design contract, so a broken
// contract must never fall back to a silent default theme.
func applySiteDesignTheme(hostSourcePath string, frontendDistPath string) error {
	document, errorValue := os.ReadFile(filepath.Join(hostSourcePath, siteDesignDocumentPath))
	if errorValue != nil {
		return nil
	}
	theme, errorValue := parseSiteDesignTheme(string(document))
	if errorValue != nil {
		return fmt.Errorf("DESIGN.md front matter is invalid: %s; fix colors/typography and publish again — no build is needed", errorValue.Error())
	}
	themeCSSPath := filepath.Join(frontendDistPath, siteThemeCSSFileName)
	return os.WriteFile(themeCSSPath, []byte(renderSiteThemeCSS(theme)), 0o644)
}

// parseSiteDesignTheme validates the entire front-matter contract in one
// pass and joins every violation into a single error. A model fixing
// DESIGN.md gets the complete list of what is still wrong on its very next
// retry instead of discovering problems one at a time as each fix uncovers
// the next incremental validation failure.
func parseSiteDesignTheme(document string) (siteTheme, error) {
	frontMatter, errorValue := extractSiteDesignFrontMatter(document)
	if errorValue != nil {
		return siteTheme{}, errorValue
	}

	missingKeys := missingSiteDesignFrontMatterKeys(frontMatter)
	validationErrors := []string{}
	if len(missingKeys) > 0 {
		validationErrors = append(validationErrors, fmt.Sprintf("missing required front matter keys: %s", strings.Join(missingKeys, ", ")))
	}

	colors := parseSiteDesignNestedBlock(frontMatter, "colors")
	primaryColor, primaryColorError := normalizeSiteDesignHexColor(colors["primary"])
	if primaryColorError != nil && !containsSiteDesignKey(missingKeys, "colors") {
		validationErrors = append(validationErrors, fmt.Sprintf("colors.primary %s", primaryColorError.Error()))
	}
	backgroundColor, backgroundColorError := normalizeSiteDesignHexColor(colors["background"])
	if backgroundColorError != nil && !containsSiteDesignKey(missingKeys, "colors") {
		validationErrors = append(validationErrors, fmt.Sprintf("colors.background %s", backgroundColorError.Error()))
	}

	headingFontFamily, bodyFontFamily, typographyError := parseSiteDesignTypographyFontFamilies(frontMatter)
	if typographyError != nil && !containsSiteDesignKey(missingKeys, "typography") {
		validationErrors = append(validationErrors, typographyError.Error())
	}

	radiusValue, radiusError := parseSiteDesignRadiusValue(frontMatter)
	if radiusError != nil && !containsSiteDesignKey(missingKeys, "rounded") {
		validationErrors = append(validationErrors, "rounded "+radiusError.Error())
	}

	if len(validationErrors) > 0 {
		return siteTheme{}, errors.New(strings.Join(validationErrors, "; "))
	}

	foregroundColor, hasExplicitForeground := normalizeSiteDesignOptionalHexColor(colors["foreground"])
	if !hasExplicitForeground {
		foregroundColor = readableForegroundColor(backgroundColor)
	}
	accentColor, hasExplicitAccent := normalizeSiteDesignOptionalHexColor(colors["accent"])
	if !hasExplicitAccent {
		accentColor = primaryColor
	}
	return siteTheme{
		PrimaryColor:      primaryColor,
		BackgroundColor:   backgroundColor,
		ForegroundColor:   foregroundColor,
		AccentColor:       accentColor,
		HeadingFontFamily: headingFontFamily,
		BodyFontFamily:    bodyFontFamily,
		RadiusValue:       radiusValue,
	}, nil
}

// renderSiteThemeCSS emits the theme.css contract consumed by the scaffold:
// shadcn-convention custom properties on :root, in the same raw-hex color
// format the scaffold's src/index.css already uses. --primary-foreground is
// always derived from --primary by luminance, never read from DESIGN.md,
// so buttons and other primary-colored surfaces stay legible regardless of
// what the author wrote.
func renderSiteThemeCSS(theme siteTheme) string {
	primaryForegroundColor := readableForegroundColor(theme.PrimaryColor)
	headingFontStack := siteDesignFontStack(theme.HeadingFontFamily)
	bodyFontStack := siteDesignFontStack(theme.BodyFontFamily)
	return ":root {\n" +
		"  --primary: " + theme.PrimaryColor + ";\n" +
		"  --primary-foreground: " + primaryForegroundColor + ";\n" +
		"  --background: " + theme.BackgroundColor + ";\n" +
		"  --foreground: " + theme.ForegroundColor + ";\n" +
		"  --accent: " + theme.AccentColor + ";\n" +
		"  --radius: " + theme.RadiusValue + ";\n" +
		"  --font-heading: " + headingFontStack + ";\n" +
		"  --font-body: " + bodyFontStack + ";\n" +
		"}\n"
}

func siteDesignFontStack(requestedFontFamily string) string {
	return requestedFontFamily + ", Pretendard, system-ui, sans-serif"
}

// extractSiteDesignFrontMatter mirrors the tolerance of the scaffold's
// scripts/build.ts: front matter must start the document with "---\n" and
// end at the first following "\n---".
func extractSiteDesignFrontMatter(document string) (string, error) {
	if !strings.HasPrefix(document, "---\n") {
		return "", errors.New("must start with YAML front matter (---)")
	}
	relativeEndIndex := strings.Index(document[4:], "\n---")
	if relativeEndIndex < 0 {
		return "", errors.New("front matter must end with ---")
	}
	return document[4 : 4+relativeEndIndex], nil
}

func missingSiteDesignFrontMatterKeys(frontMatter string) []string {
	missingKeys := []string{}
	for _, key := range siteDesignFrontMatterRequiredKeyOrder {
		if !siteDesignFrontMatterRequiredKeyPatterns[key].MatchString(frontMatter) {
			missingKeys = append(missingKeys, key)
		}
	}
	return missingKeys
}

func containsSiteDesignKey(keys []string, key string) bool {
	for _, candidateKey := range keys {
		if candidateKey == key {
			return true
		}
	}
	return false
}

// parseSiteDesignNestedBlock extracts a flat two-space-indented key/value
// map from a top-level YAML-ish section, e.g. the colors: block.
func parseSiteDesignNestedBlock(frontMatter string, sectionKey string) map[string]string {
	values := map[string]string{}
	insideSection := false
	for _, line := range strings.Split(frontMatter, "\n") {
		trimmedLine := strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(trimmedLine) == "" {
			continue
		}
		if leadingSpaceCount(trimmedLine) == 0 {
			insideSection = strings.HasPrefix(strings.TrimSpace(trimmedLine), sectionKey+":")
			continue
		}
		if !insideSection || leadingSpaceCount(trimmedLine) != 2 {
			continue
		}
		key, value, isKeyValue := splitSiteDesignKeyValueLine(trimmedLine)
		if isKeyValue && value != "" {
			values[key] = value
		}
	}
	return values
}

// parseSiteDesignTypographyFontFamilies scans the typography: section for
// nested entries (any key naming shape, e.g. headline-display, body-md) and
// picks a representative fontFamily for headings and body text by matching
// the entry name against heading- and body-shaped substrings. When only one
// side is found in the document, the other reuses it rather than falling
// back to an unrelated hardcoded default.
func parseSiteDesignTypographyFontFamilies(frontMatter string) (string, string, error) {
	headingFontFamily := ""
	bodyFontFamily := ""
	insideTypography := false
	currentEntryName := ""
	for _, line := range strings.Split(frontMatter, "\n") {
		trimmedLine := strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(trimmedLine) == "" {
			continue
		}
		if leadingSpaceCount(trimmedLine) == 0 {
			insideTypography = strings.HasPrefix(strings.TrimSpace(trimmedLine), "typography:")
			currentEntryName = ""
			continue
		}
		if !insideTypography {
			continue
		}
		key, value, isKeyValue := splitSiteDesignKeyValueLine(trimmedLine)
		if !isKeyValue {
			continue
		}
		if value == "" {
			currentEntryName = key
			continue
		}
		if key != "fontFamily" {
			continue
		}
		lowerEntryName := strings.ToLower(currentEntryName)
		if headingFontFamily == "" && siteDesignEntryNameMatchesAny(lowerEntryName, "head", "title", "display") {
			headingFontFamily = value
		}
		if bodyFontFamily == "" && siteDesignEntryNameMatchesAny(lowerEntryName, "body") {
			bodyFontFamily = value
		}
	}
	if headingFontFamily == "" && bodyFontFamily == "" {
		return "", "", errors.New("typography section has no fontFamily values")
	}
	if headingFontFamily == "" {
		headingFontFamily = bodyFontFamily
	}
	if bodyFontFamily == "" {
		bodyFontFamily = headingFontFamily
	}
	return headingFontFamily, bodyFontFamily, nil
}

func siteDesignEntryNameMatchesAny(entryName string, substrings ...string) bool {
	for _, substring := range substrings {
		if strings.Contains(entryName, substring) {
			return true
		}
	}
	return false
}

// parseSiteDesignRadiusValue reads the rounded: section, accepting either a
// flat scalar ("rounded: 8px") or a nested scale map ("rounded:\n  md: 8px"),
// preferring the md entry when a scale map is used.
func parseSiteDesignRadiusValue(frontMatter string) (string, error) {
	lines := strings.Split(frontMatter, "\n")
	for lineIndex, line := range lines {
		trimmedLine := strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(trimmedLine) == "" || leadingSpaceCount(trimmedLine) != 0 {
			continue
		}
		key, value, isKeyValue := splitSiteDesignKeyValueLine(trimmedLine)
		if !isKeyValue || key != "rounded" {
			continue
		}
		if value != "" {
			return normalizeSiteDesignLengthValue(value)
		}
		return radiusValueFromNestedScale(lines[lineIndex+1:])
	}
	return "", errors.New("value could not be determined")
}

func radiusValueFromNestedScale(remainingLines []string) (string, error) {
	nestedValues := map[string]string{}
	firstNestedKey := ""
	for _, line := range remainingLines {
		trimmedLine := strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(trimmedLine) == "" {
			continue
		}
		if leadingSpaceCount(trimmedLine) == 0 {
			break
		}
		if leadingSpaceCount(trimmedLine) != 2 {
			continue
		}
		nestedKey, nestedValue, isNestedKeyValue := splitSiteDesignKeyValueLine(trimmedLine)
		if !isNestedKeyValue || nestedValue == "" {
			continue
		}
		nestedValues[nestedKey] = nestedValue
		if firstNestedKey == "" {
			firstNestedKey = nestedKey
		}
	}
	if mediumValue, hasMediumValue := nestedValues["md"]; hasMediumValue {
		return normalizeSiteDesignLengthValue(mediumValue)
	}
	if firstNestedKey != "" {
		return normalizeSiteDesignLengthValue(nestedValues[firstNestedKey])
	}
	return "", errors.New("value could not be determined")
}

func normalizeSiteDesignLengthValue(value string) (string, error) {
	trimmedValue := strings.TrimSpace(value)
	if keywordValue, isKeyword := siteDesignRoundedKeywordValues[strings.ToLower(trimmedValue)]; isKeyword {
		return keywordValue, nil
	}
	matches := siteDesignLengthPattern.FindStringSubmatch(trimmedValue)
	if matches == nil {
		return "", fmt.Errorf("must be a CSS length or scale keyword, got %q", trimmedValue)
	}
	unit := matches[2]
	if unit == "" {
		unit = "px"
	}
	return matches[1] + unit, nil
}

func splitSiteDesignKeyValueLine(line string) (string, string, bool) {
	trimmedLine := strings.TrimSpace(line)
	colonIndex := strings.Index(trimmedLine, ":")
	if colonIndex < 0 {
		return "", "", false
	}
	key := strings.TrimSpace(trimmedLine[:colonIndex])
	value := strings.Trim(strings.TrimSpace(trimmedLine[colonIndex+1:]), `"'`)
	if key == "" {
		return "", "", false
	}
	return key, value, true
}

func leadingSpaceCount(line string) int {
	count := 0
	for _, character := range line {
		if character != ' ' {
			break
		}
		count++
	}
	return count
}

func normalizeSiteDesignHexColor(value string) (string, error) {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return "", errors.New("is required")
	}
	matches := siteDesignHexColorPattern.FindStringSubmatch(trimmedValue)
	if matches == nil {
		return "", fmt.Errorf("must be a hex color, got %q", trimmedValue)
	}
	hexDigits := matches[1]
	if len(hexDigits) == 3 {
		expandedDigits := make([]byte, 0, 6)
		for _, digit := range []byte(hexDigits) {
			expandedDigits = append(expandedDigits, digit, digit)
		}
		hexDigits = string(expandedDigits)
	}
	return "#" + strings.ToLower(hexDigits), nil
}

func normalizeSiteDesignOptionalHexColor(value string) (string, bool) {
	if strings.TrimSpace(value) == "" {
		return "", false
	}
	normalizedColor, errorValue := normalizeSiteDesignHexColor(value)
	if errorValue != nil {
		return "", false
	}
	return normalizedColor, true
}

// readableForegroundColor picks black or white text over backgroundColor
// using WCAG relative luminance, so DESIGN.md never needs to hand-pick a
// contrasting foreground for every color it declares.
func readableForegroundColor(backgroundColor string) string {
	if relativeLuminance(backgroundColor) > 0.5 {
		return "#000000"
	}
	return "#ffffff"
}

func relativeLuminance(hexColor string) float64 {
	redChannel, greenChannel, blueChannel := hexColorChannels(hexColor)
	return 0.2126*linearizeColorChannel(redChannel) + 0.7152*linearizeColorChannel(greenChannel) + 0.0722*linearizeColorChannel(blueChannel)
}

func hexColorChannels(hexColor string) (float64, float64, float64) {
	hexDigits := strings.TrimPrefix(hexColor, "#")
	redComponent, _ := strconv.ParseInt(hexDigits[0:2], 16, 0)
	greenComponent, _ := strconv.ParseInt(hexDigits[2:4], 16, 0)
	blueComponent, _ := strconv.ParseInt(hexDigits[4:6], 16, 0)
	return float64(redComponent) / 255, float64(greenComponent) / 255, float64(blueComponent) / 255
}

func linearizeColorChannel(channelValue float64) float64 {
	if channelValue <= 0.03928 {
		return channelValue / 12.92
	}
	return math.Pow((channelValue+0.055)/1.055, 2.4)
}
