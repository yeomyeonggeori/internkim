# Artifact Quality Tester

Use a bounded quality loop for rendered artifacts:

1. Capture intent, audience, artifact kind, and success criteria.
2. Build or render the artifact.
3. Run deterministic checks for blank output, stale builds, clipped content, missing source files, default fonts, starter markers, and format-specific risks.
4. Create visual evidence with screenshots, contact sheets, exported page images, or slide images.
5. Call `artifact_review` with the evidence, intent, rubric, expected visible text when available, and previous issues.
6. Revise source when any blocking issue remains. Repeat at most three times.
7. Accept warnings only with an explicit rationale in the review decision.
8. Publish, promote, or attach only after the review decision has no blocking issues.

Severity:

- `blocking`: clipped or missing content, blank render, broken responsive layout, unreadable text, starter/template leakage, or wrong main experience.
- `warning`: polish, density, hierarchy, alignment, or fidelity issues that should be fixed or explicitly accepted.
- `info`: useful notes that do not affect delivery.

Keep the runtime implementation shared. Keep each skill's workflow self-contained enough that the model can execute it without reading this reference first.
