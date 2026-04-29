---
# ===================================================================
# SCAFFOLD — intentionally minimal black & white.
#
# Before building, open DESIGN.md and fill it in, then propagate the
# values into this file:
#   1. Replace the default font import with DESIGN.md's primaryImportURL
#   2. Fill --display-font and --body-font from DESIGN.md
#   3. Fill your color palette into --accent, --text-*, etc.
#      (defaults below are pure black/white so nothing looks styled yet)
#   4. Adjust --card-* and --pill-radius if your design calls for it
#
# The goal of this neutrality: if you skip DESIGN.md, the deck looks
# obviously unfinished and reminds you to come back to it.
# ===================================================================
marp: true
theme: uncover
paginate: false
backgroundColor: "#FFFFFF"
color: "#000000"
html: true
style: |
  /* -------- Fonts --------
     Korean-first default. Replace with DESIGN.md's primaryImportURL if changed. */
  @import url('https://cdn.jsdelivr.net/gh/fonts-archive/Paperlogy/Paperlogy.css');

  :root {
    /* Font families — replace with choices from DESIGN.md */
    --display-font: Paperlogy, A2Z, Freesentation, "Pretendard Variable", Pretendard, "Noto Sans KR", "Apple SD Gothic Neo", "Malgun Gothic", sans-serif;
    --alternate-display-font: A2Z, Paperlogy, Freesentation, "Pretendard Variable", Pretendard, "Noto Sans KR", "Apple SD Gothic Neo", "Malgun Gothic", sans-serif;
    --body-font: Freesentation, Paperlogy, "Pretendard Variable", Pretendard, "Noto Sans KR", "Apple SD Gothic Neo", "Malgun Gothic", sans-serif;

    /* Color palette — replace with values from DESIGN.md.
       These defaults are intentionally B&W so an unfinished deck looks unfinished. */
    --bg: #FFFFFF;
    --text-primary: #000000;
    --text-body: #000000;
    --text-muted: #666666;
    --accent: #000000;  /* set to a real accent once DESIGN.md is locked */
    --divider: #CCCCCC;

    /* Component tokens */
    --card-radius: 12px;
    --card-border: 1.5px solid var(--text-primary);
    --card-fill: #FFFFFF;
    --pill-radius: 20px;
  }

  /* -------- Base -------- */
  section {
    font-family: var(--body-font);
    letter-spacing: 0;
    padding: 48px 64px;
    justify-content: flex-start;
    background: var(--bg);
    color: var(--text-primary);
  }
  h1 {
    font-family: var(--display-font);
    color: var(--accent);
    font-size: 80px;
    font-weight: 900;
    text-align: center;
    line-height: 0.95;
    margin: 0;
    letter-spacing: 0;
  }
  h2 {
    font-family: var(--display-font);
    color: var(--text-primary);
    font-size: 20px;
    font-weight: 700;
    margin: 0 0 8px 0;
  }
  p, li { font-size: 15px; color: var(--text-body); line-height: 1.55; margin: 4px 0; letter-spacing: 0; }
  ul { margin: 4px 0; padding-left: 20px; }
  strong { color: var(--text-primary); }
  em { color: var(--accent); font-style: normal; font-weight: 700; }

  /* -------- Tables -------- */
  table { font-size: 13px; width: 100%; border-collapse: collapse; margin-top: 12px; }
  th { background: var(--text-primary); color: var(--bg); font-weight: 600; padding: 8px 12px; text-align: left; }
  td { padding: 8px 12px; border-bottom: 1px solid var(--divider); }
  tr:nth-child(even) td { background: #FAFAFA; }

  /* -------- Lead slide (cover / section / thank-you) -------- */
  section.lead { justify-content: center; }
  section.lead h1 { font-size: 100px; }

  /* -------- Helpers / components -------- */
  footer { display: none; }
  .topbar { display: flex; justify-content: space-between; font-size: 13px; color: var(--text-muted); margin-bottom: 16px; }
  .subtitle { text-align: center; font-size: 18px; color: var(--text-body); margin: 12px 0 24px 0; }

  .grid { display: grid; gap: 20px; margin-top: 16px; }
  .grid-2 { grid-template-columns: 1fr 1fr; }
  .grid-3 { grid-template-columns: 1fr 1fr 1fr; }
  .grid-4 { grid-template-columns: 1fr 1fr 1fr 1fr; }

  .card { border: var(--card-border); border-radius: var(--card-radius); padding: 24px; background: var(--card-fill); }
  .card h2 { font-size: 17px; margin: 0 0 8px 0; }
  .card p { font-size: 14px; margin: 4px 0; }
  .card ul { font-size: 14px; margin: 4px 0; }
  .card-accent { border: 1.5px solid var(--accent); border-radius: var(--card-radius); padding: 24px; background: var(--card-fill); }
  .card-dark { background: var(--text-primary); border-radius: var(--card-radius); padding: 24px; color: var(--bg); }
  .card-dark h2 { color: var(--bg); }
  .card-dark p { color: rgba(255,255,255,0.7); }

  .label { font-family: var(--display-font); font-size: 11px; font-weight: 700; color: var(--accent); letter-spacing: 0; text-transform: uppercase; margin: 0 0 4px 0; }
  .big { font-family: var(--display-font); font-size: 32px; font-weight: 800; color: var(--text-primary); margin: 0; line-height: 1.1; }
  .big-accent { font-family: var(--display-font); font-size: 32px; font-weight: 800; color: var(--accent); margin: 0; line-height: 1.1; }
  .center { text-align: center; }

  .pill { display: inline-block; background: var(--text-primary); color: var(--bg); padding: 4px 14px; border-radius: var(--pill-radius); font-size: 12px; font-weight: 600; margin: 2px 4px; }
  .pill-accent { display: inline-block; background: var(--accent); color: var(--bg); padding: 4px 14px; border-radius: var(--pill-radius); font-size: 12px; font-weight: 700; margin: 2px 4px; }
  .pill-outline { display: inline-block; border: 1.5px solid var(--text-primary); color: var(--text-primary); padding: 3px 12px; border-radius: var(--pill-radius); font-size: 12px; font-weight: 500; margin: 2px 4px; }

  .ref { font-size: 11px; color: var(--text-muted); margin-top: 12px; }
  .divider { border: none; border-top: 1px solid var(--divider); margin: 16px 0; }
  .bar { background: var(--accent); height: 3px; width: 100%; border-radius: 2px; margin: 10px 0; }
---

<!-- _class: lead -->
<!-- _paginate: false -->

<div class="topbar"><span>PRESENTER</span><span>DATE</span></div>

# YOUR TOPIC

<p style="text-align:center;color:var(--text-body);font-size:20px;margin:12px 0">Subtitle or tagline</p>

<!--
Opening line. Introduce the topic and why it matters.

Key points: (1) What we're covering (2) Why now
Duration: 30 seconds
-->

---

# Section One

<p class="subtitle">Brief subtitle</p>

<div class="grid grid-3">
<div class="card">
<h2>First Point</h2>
<p>Description of the first key point.</p>
</div>
<div class="card">
<h2>Second Point</h2>
<p>Description of the second key point.</p>
</div>
<div class="card">
<h2>Third Point</h2>
<p>Description of the third key point.</p>
</div>
</div>

<!--
[Transition] Bridge from previous slide.

Main talking points about this section. [Pause] for emphasis where needed.

Key points: (1) First (2) Second (3) Third
Duration: 2 minutes
-->

---

<!-- _class: lead -->

# Thank you

<p style="text-align:center;color:var(--text-muted);font-size:16px;margin-top:24px">Deck Title<br>Presenter Name | Month Year</p>

<!--
[Transition] Closing remarks.

Thank the audience. Invite questions or next steps.

Duration: Flexible
-->
