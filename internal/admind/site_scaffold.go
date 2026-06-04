package admind

import (
	"embed"
	"html"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
)

//go:embed site_scaffold/react-vite-ts
var siteScaffoldFS embed.FS

const siteScaffoldRoot = "site_scaffold/react-vite-ts"

func siteAppScaffoldTemplateFiles(site *SiteRecord) []siteTemplateFile {
	files := []siteTemplateFile{}
	_ = fs.WalkDir(siteScaffoldFS, siteScaffoldRoot, func(path string, directoryEntry fs.DirEntry, walkError error) error {
		if walkError != nil || directoryEntry.IsDir() {
			return nil
		}
		document, readError := siteScaffoldFS.ReadFile(path)
		if readError != nil {
			return nil
		}
		relativePath, relativeError := filepath.Rel(siteScaffoldRoot, path)
		if relativeError != nil {
			return nil
		}
		files = append(files, siteTemplateFile{
			Path:     filepath.ToSlash(filepath.Join("app", relativePath)),
			Document: siteScaffoldContent(site, string(document)),
		})
		return nil
	})
	return files
}

func siteScaffoldContent(site *SiteRecord, content string) string {
	replacements := map[string]string{
		"__SITE_PACKAGE_NAME__": normalizeSiteSlug(site.Slug),
		"__SITE_TITLE__":        html.EscapeString(firstNonEmpty(site.Title, site.Slug)),
	}
	result := content
	for key, value := range replacements {
		result = strings.ReplaceAll(result, key, value)
	}
	return result
}

func siteDesignMD(site *SiteRecord) string {
	title := firstNonEmpty(strings.TrimSpace(site.Title), site.Slug)
	quotedTitle := strconv.Quote(title)
	return `---
version: alpha
name: ` + quotedTitle + `
description: Beautiful default prototype design system for a shadcn React site.
colors:
  primary: "#111111"
  primary-foreground: "#FFFFFF"
  secondary: "#F4F4F5"
  tertiary: "#E5E7EB"
  neutral: "#6B7280"
  background: "#FFFFFF"
  surface: "#FFFFFF"
  surface-muted: "#F7F7F8"
  border: "#E5E7EB"
  destructive: "#B42318"
typography:
  headline-display:
    fontFamily: ui-serif
    fontSize: 56px
    fontWeight: 650
    lineHeight: 1.02
    letterSpacing: 0px
  headline-lg:
    fontFamily: ui-serif
    fontSize: 38px
    fontWeight: 650
    lineHeight: 1.08
    letterSpacing: 0px
  body-md:
    fontFamily: ui-sans-serif
    fontSize: 16px
    fontWeight: 400
    lineHeight: 1.6
    letterSpacing: 0px
  label-md:
    fontFamily: ui-sans-serif
    fontSize: 13px
    fontWeight: 650
    lineHeight: 1.1
    letterSpacing: 0px
rounded:
  sm: 4px
  md: 8px
  lg: 12px
  full: 9999px
spacing:
  xs: 4px
  sm: 8px
  md: 16px
  lg: 24px
  xl: 40px
  page: 32px
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.primary-foreground}"
    typography: "{typography.label-md}"
    rounded: "{rounded.md}"
    padding: 12px
  button-primary-hover:
    backgroundColor: "{colors.secondary}"
  card:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.primary}"
    rounded: "{rounded.lg}"
    padding: 24px
  input:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.primary}"
    rounded: "{rounded.md}"
    padding: 12px
---

# ` + title + ` DESIGN.md

## Overview

The interface should feel like a polished prototype made for immediate idea validation: useful on the first screen, composed with confident spacing, and refined without looking like a generic SaaS landing page. Default to a black-on-white minimal utility style unless the user request clearly calls for another archetype.

## Colors

The default palette is black-on-white: white background, near-black text, quiet gray borders, and restrained monochrome controls. Use color only when the domain clearly benefits from it, and keep any accent small enough that navy, blue, purple, or gradient themes do not become the default.

## Typography

Use a serif display voice for high-level narrative headings and a clean system sans for product UI, labels, forms, and dense data. Keep letter spacing at 0px unless a specific brand direction requires otherwise.

## Layout

Start from the requested workflow instead of a decorative introduction. App-like requests should open with a usable shell, dashboard, form, board, or editor. Landing requests may use a hero, but the next section must be visible in the first viewport.

## Elevation & Depth

Prefer tonal layers, borders, and restrained shadows. Cards should frame repeated items or tools only; avoid nesting cards inside cards.

## Shapes

Use 8px as the default radius for controls and cards. Use full rounding only for avatars, pills, meters, and compact status indicators.

## Components

Build with shadcn-style primitives: buttons, inputs, labels, cards, badges, tabs, dialogs, tables, and separators. Use lucide icons in icon buttons and compact actions when the meaning is familiar.

## Do's and Don'ts

- Do make the first screen functional for the user's actual request.
- Do include realistic fake data where it helps the workflow feel usable.
- Do verify desktop and mobile layouts before publishing.
- Don't publish placeholder feature-card pages.
- Don't use meaningless gradient blobs, empty hero sections, or decorative filler.
- Don't allow text, buttons, or cards to overlap at mobile widths.
`
}

func siteBuiltIndexHTML(site *SiteRecord) string {
	title := html.EscapeString(firstNonEmpty(site.Title, site.Slug))
	return "<!doctype html>\n<html lang=\"ko\">\n<head>\n<meta charset=\"UTF-8\" />\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\" />\n<title>" + title + "</title>\n<style>body{margin:0;font-family:ui-sans-serif,system-ui;background:#fff;color:#111827}.shell{display:grid;min-height:100vh;place-items:center;padding:24px}</style>\n</head>\n<body><main class=\"shell\" data-starter-marker=\"INTERNKIM_SITE_STARTER_REPLACE_ME\"><h1>" + title + "</h1></main></body>\n</html>\n"
}
