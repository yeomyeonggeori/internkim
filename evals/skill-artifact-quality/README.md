# Skill Artifact Quality Evals

These evals measure output quality for the bundled `document`, `pdf`, `spreadsheet`, `website`, and `presentation` skills.

Each evaluation is self-contained. `evals.json` stores the fixed instruction, and `fixtures/` stores the fixed source data. The runner combines them into the exact prompt saved in each run directory.

Do not edit an existing instruction or fixture to improve a score. Add a new evaluation ID when the task or source data needs to change. If the shared runner rules or scoring criteria change, bump `version` in `evals.json` so new scores are not confused with older baselines.

Run all fixed evals:

```bash
tools/run-skill-artifact-evals run
```

The generator must use `./internkim test`, which sends each prompt through the real Local Fleet Mattermost DM path. This path evaluates whether KimIntern can select and use the skill correctly. Do not replace generation with direct skill script execution, direct tool calls, or a virtual-session shortcut.

KimIntern is not the judge. It only generates the artifact. Scoring happens after generation through a separate judge step that reads the generated artifact, the fixed source data, and audit metadata.

Runs default to fixed generation options: seed `41` and temperature `0`. Override only for a deliberately separate comparison run:

```bash
tools/run-skill-artifact-evals run --seed 41 --temperature 0
```

Preview commands without running the local fleet:

```bash
tools/run-skill-artifact-evals run --dry-run
```

Run one evaluation:

```bash
tools/run-skill-artifact-evals run --evaluation docx-business-review
```

Reuse an already running shared Local Fleet while still using Mattermost DM:

```bash
tools/run-skill-artifact-evals run --reuse
```

Each run creates a timestamped directory under `.artifacts/skill-artifact-evals/`. Result artifacts are stored separately from diagnostics:

- `results/<evaluation-id>/files/` contains the produced attachment, such as `.docx`, `.pdf`, or `.xlsx`.
- `results/<evaluation-id>/screenshots/` contains render screenshots such as `render-001.png` when local rendering is available.
- `results/<evaluation-id>/summary.json` contains the compact file, screenshot, and audit summary used by the score ledger.
- `diagnostics/<evaluation-id>/` contains prompt, command, stdout, stderr, and raw command metadata.
- `scorecard.json` is the manual scoring template for the run.

Audit artifacts in an existing run:

```bash
tools/run-skill-artifact-evals audit .artifacts/skill-artifact-evals/<run-id>
```

DOCX, PDF, and XLSX audits render a local thumbnail with `qlmanage` when available and record image dimensions with `sips`. Render PNGs are saved under each evaluation's `screenshots/` directory. XLSX audits also inspect workbook XML for sheet names, formulas, frozen panes, filters, and non-empty cells. Website runs require the Mattermost verification path to publish a URL and pass remote Chromium desktop/mobile screenshot checks.

The generated prompt also includes artifact-specific quality rules for requested-language labels, consistent date/number notation, readable document tables, and responsive website controls. These rules are deliberately generic and must not contain answer keys from a fixture.

Score a generated run with the local deterministic judge:

```bash
tools/run-skill-artifact-evals judge .artifacts/skill-artifact-evals/<run-id>
```

The heuristic judge is separate from KimIntern and is useful for repeatable source-fidelity, format, and render checks. For a separate LLM judge, provide an OpenRouter model and key:

```bash
OPENROUTER_API_KEY=... tools/run-skill-artifact-evals judge .artifacts/skill-artifact-evals/<run-id> --mode openrouter --model <judge-model>
```

You can also fill `scorecard.json` manually after reviewing the artifacts and screenshots. Then record the score:

```bash
tools/run-skill-artifact-evals record .artifacts/skill-artifact-evals/<run-id>
```

Scores are written to `scores.json` and are intended to stay in git history. The score ledger records the judge metadata. Raw generated artifacts stay ignored.

Scoring should heavily penalize unsupported invention. If the artifact adds facts not present in the fixture, score `sourceFidelity` low even when the artifact looks polished.
