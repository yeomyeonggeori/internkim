#!/usr/bin/env python3
import base64
import html
import mimetypes
import os
import pathlib
import re
import subprocess
import sys
import zipfile


SLIDE_WIDTH = 1600
SLIDE_HEIGHT = 900
PRESENTATION_WIDTH_EMU = 12192000
PRESENTATION_HEIGHT_EMU = 6858000
SLIDE_MASTER_RELATIONSHIP_ID = 2147483648


def main() -> int:
    if len(sys.argv) != 7:
        print("Usage: html_export.py <source.html> <deck-name> <build-dir> <formats> <render-review-script> <html-render-script>", file=sys.stderr)
        return 2
    source_path = pathlib.Path(sys.argv[1]).resolve()
    deck_name = sys.argv[2]
    build_path = pathlib.Path(sys.argv[3]).resolve()
    formats = enabled_formats(sys.argv[4])
    render_review_script = pathlib.Path(sys.argv[5]).resolve()
    html_render_script = pathlib.Path(sys.argv[6]).resolve()
    validate_source(source_path)
    build_path.mkdir(parents=True, exist_ok=True)
    html_output_path = build_path / f"{deck_name}.html"
    html_output_path.write_text(inline_local_resources(source_path), encoding="utf-8")
    slide_sources = extract_slide_sources(source_path)
    if not slide_sources:
        print("Error: slides.html must contain at least one <section> slide", file=sys.stderr)
        return 1
    slide_image_paths = []
    if needs_rendered_slides(formats):
        run_html_render(html_render_script, html_output_path, deck_name, build_path, formats)
        slide_image_paths = sorted((build_path / "review").glob(f"{deck_name}.[0-9][0-9][0-9].png"))
    if "pptx" in formats:
        write_image_backed_pptx(slide_image_paths, build_path / f"{deck_name}.pptx")
    if "notes" in formats:
        write_notes(slide_sources, build_path / f"{deck_name}-notes.txt")
    if "review" in formats:
        run_render_review(render_review_script, source_path, deck_name, build_path / "review")
    print_outputs(build_path, deck_name, formats)
    return 0


def enabled_formats(raw_formats: str) -> set[str]:
    return {value.strip() for value in raw_formats.split(",") if value.strip()}


def needs_rendered_slides(formats: set[str]) -> bool:
    return "pptx" in formats or "pdf" in formats or "review" in formats


def validate_source(source_path: pathlib.Path) -> None:
    if not source_path.exists():
        raise SystemExit(f"Error: {source_path.name} not found. Create slides.html or set SRC=yourfile.html")
    design_path = source_path.with_name("DESIGN.md")
    if not design_path.exists():
        raise SystemExit("Error: DESIGN.md not found. Create Stitch-compatible DESIGN.md before building.")
    source_text = source_path.read_text(encoding="utf-8")
    if "design-source: DESIGN.md" not in source_text:
        raise SystemExit(f"Error: {source_path.name} must include design-source: DESIGN.md")


def extract_slide_sources(source_path: pathlib.Path) -> list[str]:
    source_text = source_path.read_text(encoding="utf-8")
    sections = re.findall(r"<section\b[^>]*>.*?</section>", source_text, flags=re.IGNORECASE | re.DOTALL)
    return [section.strip() for section in sections if section.strip()]


def inline_local_resources(source_path: pathlib.Path) -> str:
    source_text = source_path.read_text(encoding="utf-8")
    source_text = inject_vendored_paperlogy_fallback(source_text)
    source_text = inline_local_images(source_text, source_path.parent)
    source_text = inline_local_fonts(source_text, source_path.parent)
    return inject_screen_slide_viewer(source_text)


def inject_vendored_paperlogy_fallback(source_text: str) -> str:
    source_text = add_paperlogy_local_to_font_family_lists(source_text)
    font_style = vendored_paperlogy_fallback_style()
    if "data-internkim-vendored-fonts" in source_text:
        return re.sub(
            r"<style\b[^>]*data-internkim-vendored-fonts[^>]*>.*?</style>",
            font_style,
            source_text,
            count=1,
            flags=re.IGNORECASE | re.DOTALL,
        )
    style_match = re.search(r"<style\b[^>]*>", source_text, flags=re.IGNORECASE)
    if style_match:
        insert_index = style_match.start()
        return source_text[:insert_index] + "\n" + font_style + "\n" + source_text[insert_index:]
    head_match = re.search(r"</head>", source_text, flags=re.IGNORECASE)
    if head_match:
        insert_index = head_match.start()
        return source_text[:insert_index] + "<style>\n" + font_style + "\n</style>\n" + source_text[insert_index:]
    return "<style>\n" + font_style + "\n</style>\n" + source_text


def add_paperlogy_local_to_font_family_lists(source_text: str) -> str:
    if "PaperlogyLocal" in source_text:
        return source_text
    source_text = re.sub(
        r'(["\']Paperlogy["\']\s*,)(?!\s*["\']PaperlogyLocal["\'])',
        r'\1 "PaperlogyLocal",',
        source_text,
    )
    return re.sub(
        r'(?<![-\w])Paperlogy\s*,(?!\s*["\']?PaperlogyLocal)',
        'Paperlogy, "PaperlogyLocal",',
        source_text,
    )


def vendored_paperlogy_fallback_style() -> str:
    fonts = [
        (400, "Paperlogy-4Regular.woff2"),
        (600, "Paperlogy-6SemiBold.woff2"),
        (700, "Paperlogy-7Bold.woff2"),
        (800, "Paperlogy-8ExtraBold.woff2"),
    ]
    rules = []
    for weight, file_name in fonts:
        font_path = pathlib.Path(__file__).resolve().parent.parent / "assets" / "fonts" / "paperlogy" / file_name
        rules.append(
            '@font-face { font-family: "PaperlogyLocal"; '
            f"font-weight: {weight}; font-style: normal; font-display: swap; "
            f'src: url("{base64_data_url("font/woff2", font_path)}") format("woff2"); }}'
        )
    return '<style data-internkim-vendored-fonts>' + "\n".join(rules) + "</style>"


def inject_screen_slide_viewer(source_text: str) -> str:
    if "data-internkim-slide-viewer" in source_text:
        return source_text
    viewer_style = """
<style data-internkim-slide-viewer>
@media screen {
  html { background: #111827; }
  body { min-height: 100vh; margin: 0; background: #111827; }
  body:not(.internkim-deck-ready) { padding: 32px; display: grid; gap: 32px; justify-items: center; align-content: start; }
  body:not(.internkim-deck-ready) section { width: 1600px; height: 900px; max-width: calc(100vw - 64px); max-height: calc((100vw - 64px) * 0.5625); aspect-ratio: 16 / 9; box-shadow: 0 24px 80px rgba(15, 23, 42, 0.28); }
  body.internkim-deck-ready { width: 100vw; height: 100vh; overflow: hidden; display: block; padding: 0; }
  .bespoke-marp-parent { position: absolute; inset: 0; overflow: hidden; }
  .marpit { position: absolute; left: 50%; top: 50%; width: 1600px; height: 900px; transform: translate(-50%, -50%) scale(var(--internkim-deck-scale, 1)); transform-origin: center center; }
  .marpit > section { position: absolute; inset: 0; width: 1600px !important; height: 900px !important; min-width: 1600px !important; max-width: none !important; min-height: 900px !important; max-height: none !important; margin: 0 !important; box-sizing: border-box; box-shadow: 0 24px 80px rgba(15, 23, 42, 0.28); display: none !important; pointer-events: none; }
  .marpit > section.bespoke-active { display: block !important; pointer-events: auto; z-index: 1; }
  .bespoke-marp-osc { position: fixed; right: 20px; bottom: 18px; display: flex; align-items: center; gap: 8px; padding: 8px; border: 1px solid rgba(255,255,255,0.16); border-radius: 999px; background: rgba(15,23,42,0.86); color: white; font: 600 13px system-ui, -apple-system, BlinkMacSystemFont, sans-serif; box-shadow: 0 16px 40px rgba(0,0,0,0.26); backdrop-filter: blur(12px); z-index: 9999; }
  .bespoke-marp-osc button { width: 32px; height: 32px; border: 0; border-radius: 999px; background: rgba(255,255,255,0.12); color: white; cursor: pointer; display: grid; place-items: center; font: inherit; }
  .bespoke-marp-osc button:disabled { opacity: 0.35; cursor: default; }
  .bespoke-marp-osc svg { width: 18px; height: 18px; stroke: currentColor; stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; fill: none; }
  .bespoke-marp-osc-status { min-width: 44px; text-align: center; }
  .bespoke-marp-tooltip { position: fixed; left: 0; top: 0; opacity: 0; pointer-events: none; transform: translate(-50%, calc(-100% - 12px)); padding: 6px 8px; border-radius: 6px; background: rgba(15,23,42,0.94); color: white; font: 600 12px system-ui, -apple-system, BlinkMacSystemFont, sans-serif; white-space: nowrap; z-index: 10002; transition: opacity 120ms ease; }
  .bespoke-marp-tooltip[data-open="true"] { opacity: 1; }
  body[data-bespoke-view="presenter"] .bespoke-marp-osc button:not([data-action="previous"]):not([data-action="next"]) { display: none; }
  .bespoke-progress-parent { position: fixed; left: 0; right: 0; bottom: 0; height: 4px; background: rgba(255,255,255,0.14); z-index: 9998; }
  .bespoke-progress-bar { display: block; height: 100%; width: 0; background: #60a5fa; transition: width 180ms ease; }
  .bespoke-marp-overview { position: fixed; inset: 0; display: none; overflow: auto; padding: 72px 48px; background: rgba(2, 6, 23, 0.92); z-index: 10000; }
  .bespoke-marp-overview[data-open="true"] { display: block; }
  .bespoke-marp-overview-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 24px; max-width: 1440px; margin: 0 auto; }
  .bespoke-marp-overview-card { display: grid; gap: 10px; border: 0; background: transparent; color: white; text-align: left; cursor: pointer; font: 600 13px system-ui, -apple-system, BlinkMacSystemFont, sans-serif; }
  .bespoke-marp-overview-card.is-current .bespoke-marp-overview-thumb { outline: 3px solid #60a5fa; }
  .bespoke-marp-overview-thumb { position: relative; width: 100%; aspect-ratio: 16 / 9; overflow: hidden; border-radius: 8px; background: #0f172a; box-shadow: 0 18px 46px rgba(0,0,0,0.35); }
  .bespoke-marp-overview-thumb > section { position: absolute !important; left: 0 !important; top: 0 !important; width: 1600px !important; height: 900px !important; min-width: 1600px !important; max-width: none !important; min-height: 900px !important; max-height: none !important; opacity: 1 !important; pointer-events: none !important; transform: scale(var(--internkim-thumb-scale, 0.16)) !important; transform-origin: 0 0 !important; box-shadow: none !important; }
  .bespoke-marp-overview-close { position: fixed; right: 24px; top: 20px; width: 36px; height: 36px; border: 0; border-radius: 999px; background: rgba(255,255,255,0.14); color: white; cursor: pointer; font: 700 16px system-ui, -apple-system, BlinkMacSystemFont, sans-serif; z-index: 10001; }
  .bespoke-marp-overview-close svg { width: 18px; height: 18px; stroke: currentColor; stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; fill: none; }
  body[data-bespoke-view="presenter"] { background: #161616; }
  body[data-bespoke-view="presenter"] .bespoke-marp-parent { right: 34%; bottom: 72px; }
  .bespoke-marp-presenter-panel { position: fixed; top: 0; right: 0; bottom: 0; width: 34%; display: grid; grid-template-rows: minmax(160px, 38%) 1fr 64px; gap: 0; background: #202020; color: white; font: 500 15px system-ui, -apple-system, BlinkMacSystemFont, sans-serif; z-index: 3; }
  .bespoke-marp-presenter-next { position: relative; overflow: hidden; margin: 20px; border-radius: 10px; background: #111827; box-shadow: 0 12px 34px rgba(0,0,0,0.35); }
  .bespoke-marp-presenter-next > section { position: absolute !important; left: 0 !important; top: 0 !important; width: 1600px !important; height: 900px !important; min-width: 1600px !important; max-width: none !important; min-height: 900px !important; max-height: none !important; opacity: 1 !important; pointer-events: none !important; transform: scale(var(--internkim-presenter-next-scale, 0.2)) !important; transform-origin: 0 0 !important; box-shadow: none !important; }
  .bespoke-marp-presenter-note { overflow: auto; padding: 0 24px 24px; color: #d1d5db; line-height: 1.55; white-space: pre-wrap; }
  .bespoke-marp-presenter-info { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 0 24px; border-top: 1px solid rgba(255,255,255,0.08); color: #f9fafb; }
}
@media print {
  body.internkim-deck-ready { display: block; width: auto; height: auto; overflow: visible; }
  .bespoke-marp-parent { position: static; inset: auto; overflow: visible; }
  .marpit { position: static; width: auto; height: auto; transform: none; }
  .marpit > section { position: relative; inset: auto; opacity: 1; pointer-events: auto; transform: none; box-shadow: none; page-break-after: always; }
  .bespoke-marp-osc,
  .bespoke-progress-parent,
  .bespoke-marp-overview,
  .bespoke-marp-presenter-panel { display: none !important; }
}
</style>
<script data-internkim-slide-viewer>
(() => {
  let activeIndex = 0;
  let slides = [];
  let deck = null;
  let parent = null;
  let osc = null;
  let progress = null;
  let overview = null;
  let presenterPanel = null;
  let presenterWindow = null;
  let tooltip = null;
  let isSyncingSlide = false;

  document.addEventListener("DOMContentLoaded", () => {
    slides = Array.from(document.querySelectorAll("section"));
    if (slides.length === 0) return;
    activeIndex = initialIndex();
    parent = ensureBespokeParent(slides);
    deck = parent.querySelector(".marpit");
    osc = document.createElement("nav");
    osc.className = "bespoke-marp-osc";
    osc.setAttribute("aria-label", "Slide controls");
    osc.innerHTML = `
      <button type="button" data-action="previous" aria-label="Previous slide" data-tooltip="Previous slide (← / PageUp)">${lucideIcon("chevron-left")}</button>
      <span class="bespoke-marp-osc-status" data-role="status"></span>
      <button type="button" data-action="next" aria-label="Next slide" data-tooltip="Next slide (→ / Space / PageDown)">${lucideIcon("chevron-right")}</button>
      <button type="button" data-action="fullscreen" aria-label="Fullscreen" data-tooltip="Fullscreen (F)">${lucideIcon("maximize")}</button>
      <button type="button" data-action="overview" aria-label="Overview" data-tooltip="Overview (O)">${lucideIcon("layout-grid")}</button>
      <button type="button" data-action="presenter" aria-label="Presenter view" data-tooltip="Presenter view (P)">${lucideIcon("presentation")}</button>
    `;
    progress = document.createElement("div");
    progress.className = "bespoke-progress-parent";
    progress.setAttribute("aria-hidden", "true");
    progress.innerHTML = `<span class="bespoke-progress-bar"></span>`;
    osc.addEventListener("click", (event) => {
      const button = event.target.closest("button[data-action]");
      if (!button) return;
      runControlAction(button.dataset.action);
    });
    osc.addEventListener("mouseover", (event) => {
      const button = event.target.closest("button[data-tooltip]");
      if (button) showTooltip(button);
    });
    osc.addEventListener("mouseout", (event) => {
      if (event.target.closest("button[data-tooltip]")) hideTooltip();
    });
    document.body.dataset.bespokeView = currentView();
    document.body.classList.add("internkim-deck-ready");
    parent.appendChild(osc);
    document.body.insertBefore(progress, parent);
    tooltip = document.createElement("div");
    tooltip.className = "bespoke-marp-tooltip";
    document.body.appendChild(tooltip);
    if (currentView() === "presenter") createPresenterPanel();
    updateScale();
    update();
  });

  window.addEventListener("resize", () => {
    if (deck) updateScale();
  });

  document.addEventListener("fullscreenchange", updateFullscreenButton);

  document.addEventListener("keydown", (event) => {
    if (slides.length === 0) return;
    if (event.defaultPrevented || editableTarget(event.target)) return;
    if (["ArrowRight", "PageDown", " "].includes(event.key)) {
      event.preventDefault();
      go(1);
    }
    if (["ArrowLeft", "PageUp", "Backspace"].includes(event.key)) {
      event.preventDefault();
      go(-1);
    }
    if (event.key === "Home") {
      event.preventDefault();
      show(0);
    }
    if (event.key === "End") {
      event.preventDefault();
      show(slides.length - 1);
    }
    if (event.key === "Escape" && overview?.dataset.open === "true") {
      event.preventDefault();
      closeOverview();
    }
    if ((event.key === "o" || event.key === "O") && !event.altKey && !event.ctrlKey && !event.metaKey) {
      event.preventDefault();
      toggleOverview();
    }
    if ((event.key === "f" || event.key === "F") && !event.altKey && !event.ctrlKey && !event.metaKey) {
      event.preventDefault();
      toggleFullscreen();
    }
    if ((event.key === "p" || event.key === "P") && !event.altKey && !event.ctrlKey && !event.metaKey) {
      event.preventDefault();
      openPresenterView();
    }
  });

  window.addEventListener("hashchange", () => {
    if (slides.length > 0) show(initialIndex(), false);
  });

  window.addEventListener("message", (event) => {
    if (event.data?.type !== "internkim-slide") return;
    isSyncingSlide = true;
    show(Number(event.data.index), false);
    isSyncingSlide = false;
  });

  function initialIndex() {
    const match = location.hash.match(/(?:slide-)?(\\d+)/);
    const parsed = match ? Number(match[1]) - 1 : 0;
    return clamp(parsed);
  }

  function editableTarget(target) {
    return target && target.closest && target.closest("input, textarea, select, button, [contenteditable='true']");
  }

  function go(delta) {
    show(activeIndex + delta);
  }

  function show(index, updateHash = true) {
    activeIndex = clamp(index);
    update();
    if (updateHash) history.replaceState(null, "", `#slide-${activeIndex + 1}`);
    broadcastSlideChange();
  }

  function update() {
    if (!osc || !progress) return;
    slides.forEach((slide, index) => {
      slide.classList.toggle("bespoke-active", index === activeIndex);
      slide.classList.toggle("bespoke-before", index < activeIndex);
      slide.classList.toggle("bespoke-after", index > activeIndex);
      slide.setAttribute("aria-hidden", index === activeIndex ? "false" : "true");
    });
    osc.querySelector("[data-action='previous']").disabled = activeIndex === 0;
    osc.querySelector("[data-action='next']").disabled = activeIndex === slides.length - 1;
    osc.querySelector("[data-role='status']").textContent = `${activeIndex + 1} / ${slides.length}`;
    progress.querySelector(".bespoke-progress-bar").style.width = `${((activeIndex + 1) / slides.length) * 100}%`;
    updateFullscreenButton();
    updateOverviewState();
    updatePresenterPanel();
  }

  function runControlAction(action) {
    if (action === "next") go(1);
    if (action === "previous") go(-1);
    if (action === "fullscreen") toggleFullscreen();
    if (action === "overview") toggleOverview();
    if (action === "presenter") openPresenterView();
  }

  function ensureBespokeParent(currentSlides) {
    const existingDeck = currentSlides[0].closest(".marpit");
    if (existingDeck) {
      const existingParent = existingDeck.closest(".bespoke-marp-parent");
      if (existingParent) return existingParent;
      const createdParent = document.createElement("div");
      createdParent.className = "bespoke-marp-parent";
      existingDeck.parentNode.insertBefore(createdParent, existingDeck);
      createdParent.appendChild(existingDeck);
      return createdParent;
    }
    const createdParent = document.createElement("div");
    createdParent.className = "bespoke-marp-parent";
    const createdDeck = document.createElement("div");
    createdDeck.className = "marpit";
    currentSlides[0].parentNode.insertBefore(createdParent, currentSlides[0]);
    createdParent.appendChild(createdDeck);
    currentSlides.forEach((slide) => createdDeck.appendChild(slide));
    return createdParent;
  }

  function updateScale() {
    const horizontalPadding = 64;
    const verticalPadding = 96;
    const bounds = parent.getBoundingClientRect();
    const availableWidth = Math.max(320, bounds.width - horizontalPadding);
    const availableHeight = Math.max(180, bounds.height - verticalPadding);
    const scale = Math.min(availableWidth / 1600, availableHeight / 900);
    deck.style.setProperty("--internkim-deck-scale", String(Math.max(0.1, scale)));
    updateOverviewScale();
    updatePresenterNextScale();
  }

  function clamp(index) {
    return Math.max(0, Math.min(slides.length - 1, Number.isFinite(index) ? index : 0));
  }

  function currentView() {
    return new URLSearchParams(location.search).get("view") === "presenter" ? "presenter" : "slide";
  }

  async function toggleFullscreen() {
    if (document.fullscreenElement) {
      await document.exitFullscreen();
      return;
    }
    await document.documentElement.requestFullscreen?.();
  }

  function updateFullscreenButton() {
    const button = osc?.querySelector("[data-action='fullscreen']");
    if (!button) return;
    const isFullscreen = !!document.fullscreenElement;
    button.innerHTML = lucideIcon(isFullscreen ? "minimize" : "maximize");
    button.setAttribute("aria-label", isFullscreen ? "Exit fullscreen" : "Fullscreen");
    button.dataset.tooltip = isFullscreen ? "Exit fullscreen (F)" : "Fullscreen (F)";
  }

  function toggleOverview() {
    if (overview?.dataset.open === "true") {
      closeOverview();
      return;
    }
    openOverview();
  }

  function openOverview() {
    if (!overview) overview = createOverview();
    overview.dataset.open = "true";
    updateOverviewScale();
    updateOverviewState();
  }

  function closeOverview() {
    if (overview) overview.dataset.open = "false";
  }

  function createOverview() {
    const createdOverview = document.createElement("div");
    createdOverview.className = "bespoke-marp-overview";
    createdOverview.innerHTML = `<button type="button" class="bespoke-marp-overview-close" aria-label="Close overview" title="Close overview (Esc)">${lucideIcon("x")}</button><div class="bespoke-marp-overview-grid"></div>`;
    const grid = createdOverview.querySelector(".bespoke-marp-overview-grid");
    slides.forEach((slide, index) => {
      const card = document.createElement("button");
      card.type = "button";
      card.className = "bespoke-marp-overview-card";
      card.dataset.index = String(index);
      const thumb = document.createElement("div");
      thumb.className = "bespoke-marp-overview-thumb";
      thumb.appendChild(slide.cloneNode(true));
      card.appendChild(thumb);
      card.insertAdjacentHTML("beforeend", `<span>${index + 1}</span>`);
      card.addEventListener("click", () => {
        show(index);
        closeOverview();
      });
      grid.appendChild(card);
    });
    createdOverview.querySelector(".bespoke-marp-overview-close").addEventListener("click", closeOverview);
    document.body.appendChild(createdOverview);
    return createdOverview;
  }

  function updateOverviewState() {
    overview?.querySelectorAll(".bespoke-marp-overview-card").forEach((card) => {
      card.classList.toggle("is-current", Number(card.dataset.index) === activeIndex);
    });
  }

  function updateOverviewScale() {
    overview?.querySelectorAll(".bespoke-marp-overview-thumb").forEach((thumb) => {
      thumb.style.setProperty("--internkim-thumb-scale", String(thumb.clientWidth / 1600));
    });
  }

  function openPresenterView() {
    const url = new URL(location.href);
    url.searchParams.set("view", "presenter");
    url.hash = `slide-${activeIndex + 1}`;
    presenterWindow = window.open(url.toString(), "internkim-presenter", "width=1280,height=720,menubar=no,toolbar=no");
  }

  function broadcastSlideChange() {
    if (isSyncingSlide) return;
    const message = { type: "internkim-slide", index: activeIndex };
    presenterWindow?.postMessage(message, "*");
    if (window.opener && !window.opener.closed) {
      window.opener.postMessage(message, "*");
    }
  }

  function createPresenterPanel() {
    presenterPanel = document.createElement("aside");
    presenterPanel.className = "bespoke-marp-presenter-panel";
    presenterPanel.innerHTML = `<div class="bespoke-marp-presenter-next"></div><div class="bespoke-marp-presenter-note"></div><div class="bespoke-marp-presenter-info"><span data-role="presenter-status"></span><time data-role="presenter-clock"></time></div>`;
    document.body.appendChild(presenterPanel);
    setInterval(updatePresenterClock, 1000);
    updatePresenterClock();
  }

  function updatePresenterPanel() {
    if (!presenterPanel) return;
    const nextSlide = slides[Math.min(activeIndex + 1, slides.length - 1)];
    const nextContainer = presenterPanel.querySelector(".bespoke-marp-presenter-next");
    nextContainer.replaceChildren(nextSlide.cloneNode(true));
    presenterPanel.querySelector(".bespoke-marp-presenter-note").textContent = notesForSlide(slides[activeIndex]);
    presenterPanel.querySelector("[data-role='presenter-status']").textContent = `${activeIndex + 1} / ${slides.length}`;
    updatePresenterNextScale();
  }

  function updatePresenterNextScale() {
    const nextContainer = presenterPanel?.querySelector(".bespoke-marp-presenter-next");
    if (!nextContainer) return;
    const scale = Math.min(nextContainer.clientWidth / 1600, nextContainer.clientHeight / 900);
    nextContainer.style.setProperty("--internkim-presenter-next-scale", String(Math.max(0.1, scale)));
  }

  function updatePresenterClock() {
    const clock = presenterPanel?.querySelector("[data-role='presenter-clock']");
    if (clock) clock.textContent = new Date().toLocaleTimeString();
  }

  function notesForSlide(slide) {
    const note = slide.querySelector("aside.notes, aside[role='note'], [data-speaker-notes]");
    if (note?.textContent?.trim()) return note.textContent.trim();
    const heading = slide.querySelector("h1, h2, h3")?.textContent?.trim() || `Slide ${activeIndex + 1}`;
    return `현재 슬라이드: ${heading}`;
  }

  function showTooltip(button) {
    if (!tooltip) return;
    tooltip.textContent = button.dataset.tooltip || "";
    const bounds = button.getBoundingClientRect();
    tooltip.style.left = `${bounds.left + bounds.width / 2}px`;
    tooltip.style.top = `${bounds.top}px`;
    tooltip.dataset.open = "true";
  }

  function hideTooltip() {
    if (tooltip) tooltip.dataset.open = "false";
  }

  function lucideIcon(name) {
    const icons = {
      "chevron-left": `<svg data-lucide="chevron-left" viewBox="0 0 24 24" aria-hidden="true"><path d="m15 18-6-6 6-6"></path></svg>`,
      "chevron-right": `<svg data-lucide="chevron-right" viewBox="0 0 24 24" aria-hidden="true"><path d="m9 18 6-6-6-6"></path></svg>`,
      "layout-grid": `<svg data-lucide="layout-grid" viewBox="0 0 24 24" aria-hidden="true"><rect width="7" height="7" x="3" y="3" rx="1"></rect><rect width="7" height="7" x="14" y="3" rx="1"></rect><rect width="7" height="7" x="14" y="14" rx="1"></rect><rect width="7" height="7" x="3" y="14" rx="1"></rect></svg>`,
      "maximize": `<svg data-lucide="maximize" viewBox="0 0 24 24" aria-hidden="true"><path d="M8 3H5a2 2 0 0 0-2 2v3"></path><path d="M21 8V5a2 2 0 0 0-2-2h-3"></path><path d="M3 16v3a2 2 0 0 0 2 2h3"></path><path d="M16 21h3a2 2 0 0 0 2-2v-3"></path></svg>`,
      "minimize": `<svg data-lucide="minimize" viewBox="0 0 24 24" aria-hidden="true"><path d="M8 3v3a2 2 0 0 1-2 2H3"></path><path d="M21 8h-3a2 2 0 0 1-2-2V3"></path><path d="M3 16h3a2 2 0 0 1 2 2v3"></path><path d="M16 21v-3a2 2 0 0 1 2-2h3"></path></svg>`,
      "presentation": `<svg data-lucide="presentation" viewBox="0 0 24 24" aria-hidden="true"><path d="M2 3h20"></path><path d="M21 3v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V3"></path><path d="m7 21 5-5 5 5"></path></svg>`,
      "x": `<svg data-lucide="x" viewBox="0 0 24 24" aria-hidden="true"><path d="M18 6 6 18"></path><path d="m6 6 12 12"></path></svg>`,
    };
    return icons[name] || "";
  }
})();
</script>
""".strip()
    if "</head>" in source_text.lower():
        return re.sub(r"</head>", lambda _: viewer_style + "\n</head>", source_text, count=1, flags=re.IGNORECASE)
    return viewer_style + "\n" + source_text


def inline_local_images(source_text: str, base_path: pathlib.Path) -> str:
    image_pattern = r'src="([^"]+\.(?:png|jpg|jpeg|gif|webp|svg))"'

    def replace_image(match: re.Match[str]) -> str:
        image_url = html.unescape(match.group(1))
        if image_url.startswith(("data:", "http:", "https:")):
            return match.group(0)
        image_path = resolve_resource_path(image_url, base_path)
        if image_path is None or not image_path.exists():
            return match.group(0)
        extension = image_path.suffix.lower().removeprefix(".")
        mime_type = {
            "jpg": "image/jpeg",
            "jpeg": "image/jpeg",
            "png": "image/png",
            "gif": "image/gif",
            "webp": "image/webp",
            "svg": "image/svg+xml",
        }.get(extension, mimetypes.guess_type(image_path)[0] or "application/octet-stream")
        data_url = base64_data_url(mime_type, image_path)
        return f'src="{data_url}"'

    return re.sub(image_pattern, replace_image, source_text, flags=re.IGNORECASE)


def inline_local_fonts(source_text: str, base_path: pathlib.Path) -> str:
    font_pattern = r"url\((['\"]?)([^)'\"]+\.(?:woff2|woff|otf|ttf)(?:[?#][^)'\"]*)?)\1\)"

    def replace_font(match: re.Match[str]) -> str:
        quote = match.group(1) or ""
        font_url = html.unescape(match.group(2))
        if font_url.startswith(("data:", "http:", "https:")):
            return match.group(0)
        font_path = resolve_resource_path(font_url, base_path)
        if font_path is None or not font_path.exists():
            return match.group(0)
        return f"url({quote}{base64_data_url(font_mime_type(font_path), font_path)}{quote})"

    return re.sub(font_pattern, replace_font, source_text, flags=re.IGNORECASE)


def resolve_resource_path(resource_url: str, base_path: pathlib.Path) -> pathlib.Path | None:
    clean_url = resource_url.split("#", 1)[0].split("?", 1)[0]
    if clean_url.startswith("file://"):
        return pathlib.Path(clean_url.removeprefix("file://"))
    resource_path = pathlib.Path(clean_url)
    if resource_path.is_absolute():
        resolved_skill_asset_path = resolve_skill_asset_path(resource_path)
        if resolved_skill_asset_path is not None:
            return resolved_skill_asset_path
        return resource_path
    resolved_path = (base_path / resource_path).resolve()
    if resolved_path.exists():
        return resolved_path
    return resolve_skill_asset_path(resource_path)


def resolve_skill_asset_path(resource_path: pathlib.Path) -> pathlib.Path | None:
    path_text = str(resource_path)
    marker = "presentation/assets/"
    if marker not in path_text:
        return None
    asset_relative_text = path_text.split(marker, 1)[1].lstrip("/")
    asset_path = pathlib.Path(__file__).resolve().parent.parent / "assets" / asset_relative_text
    if asset_path.exists():
        return asset_path
    return resolve_paperlogy_alias(asset_relative_text)


def resolve_paperlogy_alias(asset_relative_text: str) -> pathlib.Path | None:
    aliases = {
        "fonts/Paperlogy-Regular.woff2": "fonts/paperlogy/Paperlogy-4Regular.woff2",
        "fonts/Paperlogy-Medium.woff2": "fonts/paperlogy/Paperlogy-6SemiBold.woff2",
        "fonts/Paperlogy-SemiBold.woff2": "fonts/paperlogy/Paperlogy-6SemiBold.woff2",
        "fonts/Paperlogy-Bold.woff2": "fonts/paperlogy/Paperlogy-7Bold.woff2",
        "fonts/Paperlogy-ExtraBold.woff2": "fonts/paperlogy/Paperlogy-8ExtraBold.woff2",
    }
    aliased_path = aliases.get(asset_relative_text)
    if aliased_path is None:
        return None
    resolved_path = pathlib.Path(__file__).resolve().parent.parent / "assets" / aliased_path
    if resolved_path.exists():
        return resolved_path
    return None


def base64_data_url(mime_type: str, path: pathlib.Path) -> str:
    import base64

    encoded = base64.b64encode(path.read_bytes()).decode()
    return f"data:{mime_type};base64,{encoded}"


def font_mime_type(path: pathlib.Path) -> str:
    suffix = path.suffix.lower()
    if suffix == ".woff2":
        return "font/woff2"
    if suffix == ".woff":
        return "font/woff"
    if suffix == ".otf":
        return "font/otf"
    if suffix == ".ttf":
        return "font/ttf"
    return mimetypes.guess_type(path)[0] or "application/octet-stream"


def run_html_render(html_render_script: pathlib.Path, source_path: pathlib.Path, deck_name: str, build_path: pathlib.Path, formats: set[str]) -> None:
    if not html_render_script.exists():
        raise SystemExit("Error: html_render.mjs not found. Cannot export HTML-first deck.")
    command = [
        "bun",
        str(html_render_script),
        str(source_path),
        deck_name,
        str(build_path),
        ",".join(sorted(formats)),
    ]
    subprocess.run(command, check=True)


def write_notes(slide_sources: list[str], notes_path: pathlib.Path) -> None:
    note_blocks = []
    for index, slide_source in enumerate(slide_sources, start=1):
        notes = extract_notes(slide_source)
        if notes:
            note_blocks.append(f"Slide {index}\n{notes}")
    notes_path.write_text("\n\n".join(note_blocks) + ("\n" if note_blocks else ""), encoding="utf-8")


def extract_notes(slide_source: str) -> str:
    matches = re.findall(
        r"<(?:aside|div)\b[^>]*class=[\"'][^\"']*(?:speaker-notes|notes)[^\"']*[\"'][^>]*>(.*?)</(?:aside|div)>",
        slide_source,
        flags=re.IGNORECASE | re.DOTALL,
    )
    return "\n".join(visible_text(match) for match in matches if visible_text(match)).strip()


def visible_text(source: str) -> str:
    source = re.sub(r"<script[^>]*>.*?</script>", " ", source, flags=re.IGNORECASE | re.DOTALL)
    source = re.sub(r"<style[^>]*>.*?</style>", " ", source, flags=re.IGNORECASE | re.DOTALL)
    source = re.sub(r"<[^>]+>", "\n", source)
    lines = [re.sub(r"\s+", " ", html.unescape(line)).strip() for line in source.splitlines()]
    return "\n".join(line for line in lines if line)


def run_render_review(render_review_script: pathlib.Path, source_path: pathlib.Path, deck_name: str, review_path: pathlib.Path) -> None:
    if not render_review_script.exists():
        raise SystemExit("Error: render_review.py not found. Cannot review HTML-first deck.")
    result = subprocess.run([sys.executable, str(render_review_script), str(source_path), deck_name, str(review_path)])
    if result.returncode != 0:
        raise SystemExit(f"Error: slide render review failed; see {review_path / 'slide-review.json'}")


def write_image_backed_pptx(image_paths: list[pathlib.Path], pptx_path: pathlib.Path) -> None:
    with zipfile.ZipFile(pptx_path, "w", zipfile.ZIP_DEFLATED) as archive:
        write_pptx_static_files(archive, len(image_paths))
        for index, image_path in enumerate(image_paths, start=1):
            archive.write(image_path, f"ppt/media/image{index}.png")
            archive.writestr(f"ppt/slides/slide{index}.xml", slide_xml(index))
            archive.writestr(f"ppt/slides/_rels/slide{index}.xml.rels", slide_relationship_xml(index))




def write_pptx_static_files(archive: zipfile.ZipFile, slide_count: int) -> None:
    archive.writestr("[Content_Types].xml", content_types_xml(slide_count))
    archive.writestr("_rels/.rels", package_relationships_xml())
    archive.writestr("docProps/core.xml", core_properties_xml())
    archive.writestr("docProps/app.xml", app_properties_xml(slide_count))
    archive.writestr("ppt/presentation.xml", presentation_xml(slide_count))
    archive.writestr("ppt/_rels/presentation.xml.rels", presentation_relationships_xml(slide_count))
    archive.writestr("ppt/slideMasters/slideMaster1.xml", slide_master_xml())
    archive.writestr("ppt/slideMasters/_rels/slideMaster1.xml.rels", slide_master_relationships_xml())
    archive.writestr("ppt/slideLayouts/slideLayout1.xml", slide_layout_xml())
    archive.writestr("ppt/slideLayouts/_rels/slideLayout1.xml.rels", slide_layout_relationships_xml())
    archive.writestr("ppt/theme/theme1.xml", theme_xml())


def content_types_xml(slide_count: int) -> str:
    overrides = [
        '<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>',
        '<Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>',
        '<Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>',
        '<Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>',
        '<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>',
        '<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>',
    ]
    overrides.extend(
        f'<Override PartName="/ppt/slides/slide{index}.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>'
        for index in range(1, slide_count + 1)
    )
    return xml_document(
        '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">'
        '<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>'
        '<Default Extension="xml" ContentType="application/xml"/>'
        '<Default Extension="png" ContentType="image/png"/>'
        + "".join(overrides)
        + "</Types>"
    )


def package_relationships_xml() -> str:
    return xml_document(
        '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
        '<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>'
        '<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>'
        '<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>'
        "</Relationships>"
    )


def presentation_relationships_xml(slide_count: int) -> str:
    relationships = [
        '<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>',
    ]
    relationships.extend(
        f'<Relationship Id="rId{index + 1}" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide{index}.xml"/>'
        for index in range(1, slide_count + 1)
    )
    return xml_document(f'<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">{"".join(relationships)}</Relationships>')


def presentation_xml(slide_count: int) -> str:
    slide_ids = "".join(f'<p:sldId id="{255 + index}" r:id="rId{index + 1}"/>' for index in range(1, slide_count + 1))
    return xml_document(
        '<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" '
        'xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" '
        'xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">'
        f'<p:sldMasterIdLst><p:sldMasterId id="{SLIDE_MASTER_RELATIONSHIP_ID}" r:id="rId1"/></p:sldMasterIdLst>'
        f"<p:sldIdLst>{slide_ids}</p:sldIdLst>"
        f'<p:sldSz cx="{PRESENTATION_WIDTH_EMU}" cy="{PRESENTATION_HEIGHT_EMU}" type="wide"/>'
        '<p:notesSz cx="6858000" cy="9144000"/>'
        "</p:presentation>"
    )


def slide_xml(index: int) -> str:
    return xml_document(
        '<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" '
        'xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" '
        'xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">'
        "<p:cSld><p:spTree>"
        '<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/>'
        '<p:pic><p:nvPicPr><p:cNvPr id="2" name="Rendered slide"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr>'
        '<p:blipFill><a:blip r:embed="rId1"/><a:stretch><a:fillRect/></a:stretch></p:blipFill>'
        f'<p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="{PRESENTATION_WIDTH_EMU}" cy="{PRESENTATION_HEIGHT_EMU}"/></a:xfrm>'
        '<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>'
        "</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>"
    )


def slide_relationship_xml(index: int) -> str:
    return xml_document(
        '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
        f'<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/image{index}.png"/>'
        '<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>'
        "</Relationships>"
    )


def slide_master_relationships_xml() -> str:
    return xml_document(
        '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
        '<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>'
        '<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>'
        "</Relationships>"
    )


def slide_layout_relationships_xml() -> str:
    return xml_document(
        '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
        '<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/>'
        "</Relationships>"
    )


def slide_master_xml() -> str:
    return xml_document(
        '<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" '
        'xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" '
        'xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">'
        '<p:cSld><p:bg><p:bgPr><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill></p:bgPr></p:bg>'
        '<p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld>'
        '<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>'
        '<p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst>'
        '<p:txStyles><p:titleStyle/><p:bodyStyle/><p:otherStyle/></p:txStyles>'
        "</p:sldMaster>"
    )


def slide_layout_xml() -> str:
    return xml_document(
        '<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" '
        'xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" '
        'xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" type="blank" preserve="1">'
        '<p:cSld name="Blank"><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld>'
        '<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>'
        "</p:sldLayout>"
    )


def theme_xml() -> str:
    return xml_document(
        '<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="InternKim">'
        '<a:themeElements><a:clrScheme name="InternKim">'
        '<a:dk1><a:srgbClr val="111827"/></a:dk1><a:lt1><a:srgbClr val="FFFFFF"/></a:lt1>'
        '<a:dk2><a:srgbClr val="374151"/></a:dk2><a:lt2><a:srgbClr val="F8FAFC"/></a:lt2>'
        '<a:accent1><a:srgbClr val="2563EB"/></a:accent1><a:accent2><a:srgbClr val="64748B"/></a:accent2>'
        '<a:accent3><a:srgbClr val="CBD5E1"/></a:accent3><a:accent4><a:srgbClr val="0F172A"/></a:accent4>'
        '<a:accent5><a:srgbClr val="475569"/></a:accent5><a:accent6><a:srgbClr val="E2E8F0"/></a:accent6>'
        '<a:hlink><a:srgbClr val="2563EB"/></a:hlink><a:folHlink><a:srgbClr val="7C3AED"/></a:folHlink>'
        '</a:clrScheme><a:fontScheme name="InternKim"><a:majorFont><a:latin typeface="Aptos Display"/></a:majorFont><a:minorFont><a:latin typeface="Aptos"/></a:minorFont></a:fontScheme>'
        '<a:fmtScheme name="InternKim"><a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:fillStyleLst>'
        '<a:lnStyleLst><a:ln w="63500"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln></a:lnStyleLst>'
        '<a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst><a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst></a:fmtScheme>'
        "</a:themeElements><a:objectDefaults/><a:extraClrSchemeLst/></a:theme>"
    )


def core_properties_xml() -> str:
    return xml_document(
        '<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" '
        'xmlns:dc="http://purl.org/dc/elements/1.1/" '
        'xmlns:dcterms="http://purl.org/dc/terms/" '
        'xmlns:dcmitype="http://purl.org/dc/dcmitype/" '
        'xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">'
        "<dc:title>HTML-first slide deck</dc:title>"
        "<dc:creator>InternKim</dc:creator>"
        "</cp:coreProperties>"
    )


def app_properties_xml(slide_count: int) -> str:
    return xml_document(
        '<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" '
        'xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">'
        "<Application>InternKim HTML Slides</Application>"
        f"<Slides>{slide_count}</Slides>"
        "</Properties>"
    )


def xml_document(body: str) -> str:
    return '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>' + body


def print_outputs(build_path: pathlib.Path, deck_name: str, formats: set[str]) -> None:
    print("")
    print("Done.")
    if "html" in formats:
        print(f"  {build_path.name}/{deck_name}.html            (HTML source rendered as deck)")
    if "pptx" in formats:
        print(f"  {build_path.name}/{deck_name}.pptx            (image-backed PowerPoint / Keynote)")
    if "pdf" in formats:
        print(f"  {build_path.name}/{deck_name}.pdf             (browser-rendered PDF)")
    if "notes" in formats:
        print(f"  {build_path.name}/{deck_name}-notes.txt       (speaker notes)")
    if "review" in formats:
        print(f"  {build_path.name}/review/slide-review.json (per-slide render review)")


if __name__ == "__main__":
    raise SystemExit(main())
