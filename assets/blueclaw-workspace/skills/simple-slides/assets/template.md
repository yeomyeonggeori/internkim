---
marp: true
theme: default
paginate: false
size: 16:9
html: true
title: Deck Title
style: |
  section {
    --background: #F8FAFC;
    --surface: #FFFFFF;
    --ink: #111827;
    --muted: #64748B;
    --teal: #0F766E;
    --amber: #F97316;
    --line: #CBD5E1;
    font-family: Freesentation, Paperlogy, "Pretendard Variable", Pretendard, "Noto Sans KR", "Apple SD Gothic Neo", sans-serif;
    background: var(--background);
    color: var(--ink);
    padding: 58px 68px;
    letter-spacing: 0;
  }
  h1, h2, h3 {
    font-family: Paperlogy, Freesentation, "Pretendard Variable", Pretendard, "Noto Sans KR", sans-serif;
    letter-spacing: 0;
    margin: 0;
  }
  h1 { font-size: 70px; line-height: 0.98; font-weight: 850; }
  h2 { font-size: 44px; line-height: 1.08; font-weight: 800; }
  h3 { font-size: 23px; line-height: 1.18; font-weight: 780; }
  p, li { font-size: 22px; line-height: 1.45; color: var(--ink); }
  .eyebrow { color: var(--teal); font-weight: 800; font-size: 15px; text-transform: uppercase; margin-bottom: 18px; }
  .subtitle { color: var(--muted); font-size: 25px; line-height: 1.42; max-width: 900px; margin-top: 24px; }
  .accent { color: var(--teal); }
  .grid { display: grid; gap: 18px; margin-top: 30px; }
  .grid-2 { grid-template-columns: 1fr 1fr; }
  .grid-3 { grid-template-columns: repeat(3, 1fr); }
  .card { background: var(--surface); border: 1px solid var(--line); border-radius: 8px; padding: 24px; min-height: 145px; }
  .card p { color: var(--muted); font-size: 18px; margin: 10px 0 0; }
  .number { color: var(--amber); font-size: 18px; font-weight: 850; margin-bottom: 10px; }
  .band { background: var(--ink); color: white; border-radius: 8px; padding: 26px 30px; margin-top: 28px; }
  .band p { color: #E5E7EB; margin: 0; }
  .pill-row { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 24px; }
  .pill { background: white; border: 1px solid var(--line); border-radius: 999px; padding: 9px 15px; font-size: 17px; color: var(--ink); }
  .footer { position: absolute; left: 68px; right: 68px; bottom: 34px; display: flex; justify-content: space-between; color: var(--muted); font-size: 14px; }
---

<!-- design-source: DESIGN.md -->

<div class="eyebrow">Deck section</div>

# Strong title with<br><span class="accent">one clear emphasis</span>

<p class="subtitle">A concise subtitle that says what the audience can do after reading the deck.</p>

<div class="pill-row">
<span class="pill">Context</span>
<span class="pill">Evidence</span>
<span class="pill">Next action</span>
</div>

<!-- Opening note. Explain why this deck exists and what decision it supports. -->

---

## Three-part story

<div class="grid grid-3">
<div class="card"><div class="number">01</div><h3>Situation</h3><p>What is happening now.</p></div>
<div class="card"><div class="number">02</div><h3>Meaning</h3><p>Why it matters to the audience.</p></div>
<div class="card"><div class="number">03</div><h3>Action</h3><p>What should happen next.</p></div>
</div>

<div class="band"><p>Use one strong takeaway per slide. Avoid raw notes and placeholder prose.</p></div>

<!-- Explain the three cards and connect them to the next slide. -->
