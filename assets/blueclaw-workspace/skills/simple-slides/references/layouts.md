# Layouts

Concrete slide layouts you can drop into `presentation.md` and fill in. All examples use CSS variables (`var(--accent)`, etc.) defined in the template frontmatter, so they adapt automatically to whatever design you chose in `DESIGN.md` — no per-deck color changes needed in these snippets.

If you need to deviate (e.g., tint a specific card), use the variables: `color: var(--accent)`, `background: var(--text-primary)`, etc. Don't hardcode hex values in slide content — the whole point is that `DESIGN.md` drives the look.

## Cover slide

```html
<!-- _class: lead -->
<!-- _paginate: false -->

<div class="topbar"><span>PRESENTER NAME</span><span>DATE</span></div>

# TOPIC TITLE

<p style="text-align:center;color:var(--text-body);font-size:20px;margin:12px 0">Subtitle or tagline</p>

<!-- Optional: logo -->
<p style="text-align:center;margin:24px 0 0 0"><img src="logo.png" style="height:120px"></p>

<!--
Opening line. Introduce the topic.
Key points: (1) Coverage (2) Why now
Duration: 30s
-->
```

## 3-column positioning slide

```html
# Slide Title

<p class="subtitle">Brief subtitle</p>

<div class="grid grid-3">
  <div class="card">
    <h2>Column 1</h2>
    <p>Body text.</p>
  </div>
  <div class="card">
    <h2>Column 2</h2>
    <p>Body text.</p>
  </div>
  <div class="card">
    <h2>Column 3</h2>
    <p>Body text.</p>
  </div>
</div>
```

## 4-card product / feature grid

```html
# Hero Products

<div class="grid grid-4">
  <div class="card" style="padding:14px">
    <p><span class="pill" style="font-size:10px">BEST</span></p>
    <img src="product1.jpg" style="width:100%;aspect-ratio:1;object-fit:contain;border-radius:6px;margin:4px 0;background:#f8f8f8">
    <p style="font-size:12px;font-weight:700;margin:2px 0">Product Name · <span style="color:var(--accent)">$24</span></p>
    <p style="font-size:11px;color:var(--text-body);margin:2px 0"><strong>USP:</strong> Short benefit</p>
    <p style="font-size:10px;color:var(--text-muted);margin:1px 0">Key ingredient · Focus area</p>
    <p style="font-size:10px;font-weight:600;margin:2px 0">Headline stat</p>
  </div>
  <!-- 3 more -->
</div>
```

Use `aspect-ratio:1` + `object-fit:contain` + a light gray image background so non-square source images look uniform.

## Horizontal timeline

```html
# Strategy Timeline

<p class="subtitle" style="margin:8px 0 16px 0">Evolution over time</p>

<div style="position:relative;padding:20px 0 0 0">
  <div style="position:absolute;top:36px;left:20px;right:20px;height:2px;background:var(--text-primary)"></div>
  <div style="display:flex;justify-content:space-between;position:relative">
    <div style="flex:1;text-align:center">
      <div style="width:14px;height:14px;border-radius:50%;background:var(--accent);border:2px solid var(--text-primary);margin:28px auto 10px"></div>
      <p class="label">PHASE 1</p>
      <p style="font-size:13px;font-weight:700;color:var(--text-primary);margin:2px 0">Launch</p>
      <p style="font-size:11px;margin:2px 0">3.5M+ units sold</p>
      <p style="font-size:10px;color:var(--text-muted);margin-top:4px">Built initial trust</p>
    </div>
    <!-- 3 more phases -->
  </div>
</div>
```

4 phases fits comfortably. More than 5 → split into 2 slides or go vertical.

## 3-column "how it works"

```html
# How It Works

<p class="subtitle">Three moving parts</p>

<div class="grid grid-3">
  <div class="card">
    <h2>Part One</h2>
    <p><strong>Headline takeaway</strong></p>
    <p>Supporting detail</p>
    <hr class="divider">
    <p><strong>Why</strong></p>
    <p>Reasoning</p>
  </div>
  <!-- 2 more cards -->
</div>
```

Use `<hr class="divider">` to break up sections inside a card. 3 sections per card max — beyond that it gets crowded.

## Tables (creators, products, comparisons)

```markdown
| Handle | Platform | Niche | Content |
|:---|:---|:---|:---|
| [**@username**](https://tiktok.com/@username) | TikTok | Skincare edu | Product review |
```

Auto-styled from the frontmatter CSS. Make handles clickable with Markdown links — works in HTML output, and in PDF the links survive.

**Highlighting a row / cell** — Markdown tables don't support styling, so
swap for HTML when you need color cues:

```html
<table class="compare">
  <thead><tr><th>Metric</th><th>Our product</th><th>Competitor</th></tr></thead>
  <tbody>
    <tr><td>Latency</td><td><strong>12ms</strong></td><td>48ms</td></tr>
    <tr><td>Price</td><td><strong>$9/mo</strong></td><td>$15/mo</td></tr>
    <tr><td>Korean support</td><td><span style="color:var(--accent);font-weight:700">✓</span></td><td>—</td></tr>
  </tbody>
</table>

<style>
.compare{width:100%;border-collapse:collapse;font-size:16px}
.compare th,.compare td{padding:10px 14px;border-bottom:1px solid var(--divider);text-align:left}
.compare th{font-weight:700;color:var(--text-primary)}
.compare td strong{color:var(--accent)}
</style>
```

Keep tables at ≤6 columns and ≤8 rows on a slide — past that it's
eye-test territory and should become a chart, a card grid, or two slides.

## Charts

Marp / Markdown has no native chart engine. Use one of these, in
preference order:

### 1. CSS bar chart (static, always works)

Best for 3–8 simple comparisons. Pure HTML/CSS — renders identically
in HTML, PDF, and PPTX. No JS needed.

```html
# Monthly active users

<div class="bars">
  <div class="bar-row"><span class="bar-label">Jan</span><div class="bar" style="width:35%"></div><span class="bar-value">1.2M</span></div>
  <div class="bar-row"><span class="bar-label">Feb</span><div class="bar" style="width:48%"></div><span class="bar-value">1.7M</span></div>
  <div class="bar-row"><span class="bar-label">Mar</span><div class="bar" style="width:62%"></div><span class="bar-value">2.1M</span></div>
  <div class="bar-row"><span class="bar-label">Apr</span><div class="bar" style="width:78%"></div><span class="bar-value">2.8M</span></div>
  <div class="bar-row"><span class="bar-label">May</span><div class="bar" style="width:100%" data-emph="true"></div><span class="bar-value">3.6M</span></div>
</div>

<style>
.bars{display:flex;flex-direction:column;gap:14px;margin-top:20px}
.bar-row{display:grid;grid-template-columns:60px 1fr 80px;gap:12px;align-items:center}
.bar-label{font-size:14px;color:var(--text-muted);text-align:right}
.bar{height:18px;background:color-mix(in srgb,var(--accent) 35%,var(--bg));border-radius:6px}
.bar[data-emph="true"]{background:var(--accent)}
.bar-value{font-size:14px;font-weight:700;color:var(--text-primary)}
</style>
```

Width % = (value / max value) × 100. Highlight the hero row with
`data-emph="true"` so the accent color draws the eye.

### 2. Inline SVG (line, donut, sparkline — full control)

For anything beyond bars, hand-author an SVG. Works everywhere
including PPTX.

```html
# Growth trend

<svg viewBox="0 0 480 200" style="width:100%;max-width:640px">
  <!-- grid -->
  <line x1="0" y1="160" x2="480" y2="160" stroke="var(--divider)" stroke-width="1"/>
  <line x1="0" y1="100" x2="480" y2="100" stroke="var(--divider)" stroke-width="1" stroke-dasharray="3,3"/>
  <line x1="0" y1="40"  x2="480" y2="40"  stroke="var(--divider)" stroke-width="1" stroke-dasharray="3,3"/>
  <!-- line -->
  <polyline points="20,140 100,120 180,90 260,70 340,50 420,25"
            fill="none" stroke="var(--accent)" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/>
  <!-- dots -->
  <circle cx="420" cy="25" r="5" fill="var(--accent)"/>
  <!-- x-axis labels -->
  <g font-size="11" fill="var(--text-muted)" text-anchor="middle">
    <text x="20"  y="180">Jan</text>
    <text x="100" y="180">Feb</text>
    <text x="180" y="180">Mar</text>
    <text x="260" y="180">Apr</text>
    <text x="340" y="180">May</text>
    <text x="420" y="180">Jun</text>
  </g>
</svg>
```

Pick y-coordinates by: `y = plot_top + (1 − value/max) × plot_height`.
Keep the plot simple — no axis titles unless essential, one accent
color, light gray grid.

**Donut chart** (single percentage):

```html
<svg viewBox="0 0 120 120" width="120" height="120">
  <circle cx="60" cy="60" r="50" fill="none" stroke="var(--divider)" stroke-width="12"/>
  <circle cx="60" cy="60" r="50" fill="none" stroke="var(--accent)" stroke-width="12"
          stroke-dasharray="251.3" stroke-dashoffset="75.4"
          transform="rotate(-90 60 60)" stroke-linecap="round"/>
  <text x="60" y="66" text-anchor="middle" font-size="22" font-weight="700" fill="var(--text-primary)">70%</text>
</svg>
```

Math: circumference = `2 × π × r` (here `2π × 50 ≈ 314.16`, rounded to
251.3 to keep ~20% invisible for a gap). `stroke-dashoffset = circumference × (1 − percentage)`.

### 3. QuickChart.io (Chart.js as a static image)

For quick data-dense charts without hand-authoring SVG — QuickChart
renders Chart.js config server-side and returns a PNG. Works in PDF and
PPTX because it's just an image by the time the slide is built.

```markdown
![Revenue by quarter](https://quickchart.io/chart?c={type:'bar',data:{labels:['Q1','Q2','Q3','Q4'],datasets:[{label:'2025',data:[12,18,26,34],backgroundColor:'%231E3A8A'}]}}&w=600&h=360&bkg=white)
```

URL-encode the `{...}` config. Keep width/height ≤ slide-safe area.
Use `&bkg=white` if your slide background is white; transparent PNG has
fringing with some Marp themes.

### 4. Mermaid (flow diagrams / sequence / gantt)

Marp does NOT ship a Mermaid renderer. Options:

- Render mermaid to SVG locally first (`npx @mermaid-js/mermaid-cli`),
  then embed the SVG inline.
- Or use QuickChart mermaid endpoint:

```markdown
![flow](https://quickchart.io/graphviz?graph=digraph{A->B;B->C;C->A})
```

Charts in decks — rules of thumb:

- One chart per slide. Don't cram.
- Label the big number, not every data point. Use an annotation if the
  audience needs to know the hero value.
- Accent color = the one thing you want them to notice. Everything else
  is neutral.
- No 3D. No gradients on bars. No pie charts beyond 3 slices.

## Video reference slides (TikTok / Instagram)

Instagram reel container — scaled iframe to balance Instagram's UI chrome vs the video area:

```html
<div style="width:270px;height:540px;overflow:hidden;border-radius:12px;border:1px solid var(--divider)">
  <iframe src="https://www.instagram.com/reel/{ID}/embed/captioned/" width="432" height="960" frameborder="0" scrolling="no" style="transform:scale(0.625);transform-origin:top left;border:none"></iframe>
</div>
```

TikTok grid (6 videos with captions):

```html
<div style="display:flex;gap:8px;justify-content:center;flex-wrap:wrap">
  <div style="width:170px">
    <div style="width:170px;height:400px;overflow:hidden;border-radius:10px;border:1px solid var(--divider)">
      <iframe src="https://www.tiktok.com/embed/v2/{VIDEO_ID}" width="340" height="800" frameborder="0" allow="autoplay" scrolling="no" style="transform:scale(0.5);transform-origin:top left;border:none"></iframe>
    </div>
    <p style="font-size:10px;color:var(--text-body);margin:6px 0 0 0;line-height:1.3">Caption</p>
  </div>
  <!-- more -->
</div>
```

Iframes show blank in PDF — this is a Chromium `printToPDF` limitation. If PDF delivery matters, swap iframes for clickable thumbnail images.

## Consumer response / testimonials

```html
# What Users Say

<div class="grid grid-3" style="gap:14px">
  <div class="card" style="padding:16px">
    <h2 style="font-size:15px">Platform Name</h2>
    <p style="font-size:13px;font-weight:700;margin:2px 0">Product — 4.8/5 (317 reviews)</p>
    <p style="font-size:11px;color:var(--text-body);margin:2px 0">⭐ "Actual quote" — Reviewer</p>
  </div>
  <!-- more platforms -->
</div>
```

## KPI scoreboard (4 big numbers)

```html
# Scoreboard

<div class="grid grid-4" style="gap:14px;margin-top:12px">
  <div class="card center" style="padding:12px">
    <p style="font-size:12px;font-weight:700;margin:0">Metric Name</p>
    <p class="big" style="font-size:24px">500%+</p>
    <p style="font-size:10px;color:var(--text-muted);margin:0">Context</p>
  </div>
  <!-- 3 more -->
</div>
```

Pair every big number with a short context line — a number alone is vague.

## Key takeaways (2x2 + ideas callout)

```html
# Key Takeaways

<div class="grid grid-2">
  <div class="card">
    <h2>What They Do Well</h2>
    <ul>
      <li><strong>Point</strong> — explanation</li>
      <li><strong>Point</strong> — explanation</li>
    </ul>
  </div>
  <div class="card">
    <h2>What We Can Improve</h2>
    <ul>
      <li><strong>Point</strong> — explanation</li>
    </ul>
  </div>
</div>

<div class="card-accent" style="margin-top:20px">
  <h2>Ideas for Us</h2>
  <ul>
    <li><strong>Idea</strong> — brief explanation</li>
  </ul>
</div>
```

`card-accent` uses the accent-color outline — reserve for the "action" section so it reads differently from the analytical cards above it.

## Thank you / closing

```html
<!-- _class: lead -->

# Thank you

<p style="text-align:center;color:var(--text-muted);font-size:16px;margin-top:24px">Deck Title<br>Presenter Name | Month Year</p>
```

Keep it simple. Contact info belongs in chat, not on the slide.
