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

//go:embed site_scaffold_dist/react-vite-ts
var siteScaffoldDistFS embed.FS

const siteScaffoldRoot = "site_scaffold/react-vite-ts"
const siteScaffoldDistRoot = "site_scaffold_dist/react-vite-ts/dist"

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

// siteScaffoldCanonicalDistFiles returns the embedded prebuilt dist as
// dist-relative paths (e.g. "index.html", "assets/app.js"), title-substituted
// for the given site. Basic sites publish directly from this canonical dist
// without running a build.
func siteScaffoldCanonicalDistFiles(site *SiteRecord) []siteTemplateFile {
	files := []siteTemplateFile{}
	_ = fs.WalkDir(siteScaffoldDistFS, siteScaffoldDistRoot, func(path string, directoryEntry fs.DirEntry, walkError error) error {
		if walkError != nil || directoryEntry.IsDir() {
			return nil
		}
		document, readError := siteScaffoldDistFS.ReadFile(path)
		if readError != nil {
			return nil
		}
		relativePath, relativeError := filepath.Rel(siteScaffoldDistRoot, path)
		if relativeError != nil {
			return nil
		}
		files = append(files, siteTemplateFile{
			Path:     filepath.ToSlash(relativePath),
			Document: siteScaffoldContent(site, string(document)),
		})
		return nil
	})
	return files
}

// siteScaffoldDistTemplateFiles returns the embedded prebuilt dist as
// app/dist/... workspace template files, for materializing the canonical
// scaffold build into a site's editable workspace at create time.
func siteScaffoldDistTemplateFiles(site *SiteRecord) []siteTemplateFile {
	canonicalFiles := siteScaffoldCanonicalDistFiles(site)
	files := make([]siteTemplateFile, 0, len(canonicalFiles))
	for _, file := range canonicalFiles {
		files = append(files, siteTemplateFile{
			Path:     filepath.ToSlash(filepath.Join("app", "dist", file.Path)),
			Document: file.Document,
		})
	}
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

> TODO(design): decide this site's palette, typefaces, and style preset from the request's mood, replace the placeholder front matter values with those decisions, and delete this marker — publish refuses to ship it.

## Overview

This file is a decision brief, not a template to keep: derive palette, typefaces, texture, and the style preset from what the request is for and who visits it, then record those decisions here. The interface should feel like a polished prototype made for immediate idea validation, refined without looking like a generic SaaS landing page. A black-on-white minimal utility style is one valid choice among many — choose it deliberately, not by leaving defaults.

## Colors

State the palette you chose and why it fits the request. Keep any accent deliberate; navy, blue, purple, or gradient themes must never appear by default.

## Typography

Pick heading and body typefaces from the platform font catalog to match the site's voice, and say why. Generic keyword families (ui-serif, ui-sans-serif) are treated as no choice and replaced by the platform default.

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
