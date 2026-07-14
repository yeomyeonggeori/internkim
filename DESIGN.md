# Interface design

InternKim uses a restrained, neutral interface built from the shared shadcn-svelte components and the semantic colors in `web/src/app.css`.

## Foundations

- Use the existing background, foreground, muted, card, border, primary, destructive, success, warning, and info tokens in light and dark modes.
- Use the shared radius scale and compact controls. Avoid page-specific colors when a semantic token communicates the same meaning.
- Use the established `max-w-6xl` page width, responsive grid layouts, and spacing in multiples of four pixels.
- Use concise Korean and English copy. Controls describe their immediate action and errors explain how to recover.

## Structure

- Internal tools use the application rail and authenticated header.
- Standalone shared pages omit internal navigation and authentication chrome.
- Cards group settings, snapshots, and metrics. Headings establish page hierarchy; badges communicate state rather than acting as decoration.
- Forms use Field composition with explicit labels, descriptions, and inline errors.

## Shared company page

- `/company/` is a password-protected company page for any intended recipient, without an audience-specific label.
- The locked state is a single focused card. The unlocked page uses a company thesis, key metrics, trends, facts, milestones, contact action, and publication date.
- Monetary metrics default to USD. A local-currency toggle appears only when the published data includes one non-USD currency with a stored USD equivalent.
- Only an administrator-published snapshot appears. Empty sections remain quiet or are omitted so the page never resembles an internal dashboard.
- Team activity uses only administrator-approved aggregates. Reuse the existing avatar component with published surname and job title, and derive pulse, activity-grid, and work-distribution shapes from actual snapshot counts.
- Present recorded metrics, selected milestones, and evidence documents before the team signal. Team activity confirms that the organization is operating but never leads the evaluation hierarchy.
- Do not assume a universal company metric or that an increase is favorable. Administrators may feature one metric or use a balanced layout, localize its label and explanation, and declare increase, decrease, or neutral direction.
- Render each published metric from its available history: a current value for one period and a time series with explicit period change for repeated observations. Keep company facts as a secondary reference near the end.
- Metric notes, record attributes, and document summaries enter a snapshot only through explicit administrator approval. Original record details, document paths, counterparties, and requester identities remain private.
- Activity motion may emphasize changing data but never invent it. Reduced-motion mode presents the same data as a static visualization.
