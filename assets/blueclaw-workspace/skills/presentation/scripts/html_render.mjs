import { chromium } from "playwright-core";
import fs from "node:fs/promises";
import { existsSync } from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";

const slideWidth = 1600;
const slideHeight = 900;


function renderProgress(label) {
  process.stderr.write(`[render] ${label} ${Math.floor(Date.now() / 1000)}\n`);
}

async function main() {
  const [sourcePath, deckName, buildPath, formats] = process.argv.slice(2);
  if (!sourcePath || !deckName || !buildPath || !formats) {
    console.error("Usage: html_render.mjs <source.html> <deck-name> <build-dir> <formats>");
    process.exit(2);
  }

  const enabledFormats = new Set(formats.split(",").map((value) => value.trim()).filter(Boolean));
  const reviewPath = path.join(buildPath, "review");
  await fs.mkdir(reviewPath, { recursive: true });
  await removePreviousReviewFiles(reviewPath, deckName);

  renderProgress("chromium_probe_start");
  try {
    const { execFileSync } = await import("node:child_process");
    const probeOutput = execFileSync(
      chromiumExecutablePath(),
      ["--headless=new", "--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage", "--dump-dom", "about:blank"],
      { timeout: 20000, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] },
    );
    renderProgress(`chromium_probe_ok ${probeOutput.length}`);
  } catch (probeError) {
    renderProgress(`chromium_probe_fail ${String(probeError.message || probeError).slice(0, 160)}`);
  }

  renderProgress("launch_start");
  const browser = await chromium.launch({
    executablePath: chromiumExecutablePath(),
    headless: true,
    timeout: 60000,
    args: [
      "--disable-background-networking",
      "--disable-background-timer-throttling",
      "--disable-breakpad",
      "--disable-client-side-phishing-detection",
      "--disable-component-update",
      "--disable-default-apps",
      "--disable-dev-shm-usage",
      "--disable-gpu",
      "--disable-hang-monitor",
      "--disable-ipc-flooding-protection",
      "--disable-popup-blocking",
      "--disable-prompt-on-repost",
      "--disable-renderer-backgrounding",
      "--disable-sync",
      "--metrics-recording-only",
      "--no-default-browser-check",
      "--no-first-run",
      "--no-sandbox",
      "--password-store=basic",
      "--use-mock-keychain",
    ],
  });

  renderProgress("launched");
  try {
    const page = await browser.newPage({ viewport: { width: slideWidth, height: slideHeight }, deviceScaleFactor: 1 });
    await page.route("**/*", (route) => {
      const requestURL = route.request().url();
      if (requestURL.startsWith("file:") || requestURL.startsWith("data:")) {
        route.continue();
      } else {
        route.abort();
      }
    });
    renderProgress("navigating");
    await page.goto(pathToFileURL(sourcePath).toString(), { waitUntil: "load", timeout: 30000 }).catch(() => {});
    await page.emulateMedia({ media: "print" });
    await waitForFonts(page);
    renderProgress("navigated");

    const slideCount = await page.locator("section").count();
    if (slideCount === 0) {
      throw new Error("slides.html must contain at least one <section> slide");
    }

    if (enabledFormats.has("pdf")) {
      await page.pdf({
        path: path.join(buildPath, `${deckName}.pdf`),
        printBackground: true,
        preferCSSPageSize: true,
        width: `${slideWidth}px`,
        height: `${slideHeight}px`,
        margin: { top: "0", right: "0", bottom: "0", left: "0" },
      });
    }

    if (enabledFormats.has("pptx") || enabledFormats.has("review")) {
      const slides = await page.locator("section").all();
      renderProgress(`screenshots ${slides.length}`);
      for (let index = 0; index < slides.length; index += 1) {
        const slide = slides[index];
        await slide.screenshot({
          path: path.join(reviewPath, `${deckName}.${String(index + 1).padStart(3, "0")}.png`),
          animations: "disabled",
        });
      }
    }
    renderProgress("render_done");
  } finally {
    await browser.close();
  }
}

async function waitForFonts(page) {
  await Promise.race([
    page.evaluate(async () => {
      if (document.fonts) {
        await document.fonts.ready;
      }
    }),
    new Promise((resolve) => setTimeout(resolve, 5000)),
  ]);
}

async function removePreviousReviewFiles(reviewPath, deckName) {
  const entries = await fs.readdir(reviewPath).catch(() => []);
  for (const entry of entries) {
    if (
      entry.startsWith(`${deckName}.`) ||
      entry.startsWith("contact-sheet-") ||
      entry.startsWith("fit-review") ||
      entry === "slide-review.json" ||
      entry === "slide-review.md"
    ) {
      await fs.rm(path.join(reviewPath, entry), { force: true });
    }
  }
}

function chromiumExecutablePath() {
  const candidates = [
    process.env.CHROME_PATH,
    process.env.PUPPETEER_EXECUTABLE_PATH,
    "/usr/bin/chromium",
    "/usr/bin/chromium-browser",
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
  ].filter(Boolean);
  for (const candidate of candidates) {
    if (existsSync(candidate)) {
      return candidate;
    }
  }
  throw new Error("Chromium executable not found");
}

main().catch((error) => {
  console.error(error.stack || String(error));
  process.exit(1);
});
