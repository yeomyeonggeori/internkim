# Interface design

internkim uses a restrained, neutral interface built from the shared shadcn-svelte components and the semantic colors in `web/src/app.css`.

## Foundations

- Use the existing background, foreground, muted, card, border, primary, destructive, success, warning, and info tokens in light and dark modes.
- Use the shared radius scale and compact desktop controls. Mobile buttons and toggles have a minimum 44px touch target. Avoid page-specific colors when a semantic token communicates the same meaning.
- Use the established `max-w-6xl` page width, responsive grid layouts, and spacing in multiples of four pixels.
- Use concise Korean and English copy. Controls describe their immediate action and errors explain how to recover.

## Structure

- Internal tools use the application rail and authenticated header.
- Standalone shared pages omit internal navigation and authentication chrome.
- Cards group settings, snapshots, and metrics. Headings establish page hierarchy; badges communicate state rather than acting as decoration.
- Forms use Field composition with explicit labels, descriptions, and inline errors.

## Mobile workspace

- Keep the five application tabs at the bottom edge, with space for the device safe area. Active tabs use foreground text and `aria-current`, without a large selection pill.
- Messenger uses its conversation header as the mobile page heading. Search, refresh, language, and theme controls remain available in More; desktop keeps the authenticated header.
- Start message composition with a single row in a capsule Input Group. The ghost + button opens attachments directly and keeps a 44px touch area. Text grows within a bounded area with a stable corner radius.
- Keep attachments, editing, keyboard shortcuts, and thread composition available. Mobile + opens the file chooser directly. Mobile uses the system keyboard for emoji; desktop keeps its emoji picker and message reactions remain available.
- Compose message input with shadcn Input Group and related actions with Button Group. The shared Tiptap editor displays formatting while typing and sends Markdown. On mobile, a nonempty editor selection opens contextual formatting above the capsule; formatting retains the editor selection and active marks. Preserve native selection, copy and paste. Native BIU availability is a device verification result, never inferred from OS names or contenteditable support.
- Mobile lists prioritize the record name and its current status. Keep secondary fields in labeled details and retain sorting, pagination, permissions, and editing. File locations and document categories use a labeled selector; mail actions move into More when space is limited.
- Mobile CRM starts with its records; its full metric overview is available through a disclosure above the tabs.
- Mobile notifications appear at the top, below the safe area and page controls, following the visual viewport. Keep the compact stack and accessible 44px dismiss/action controls. Desktop keeps bottom-center placement.
- Tab labels keep their full touch targets in a horizontally scrollable row. Dialogs and side panels follow the visual viewport during keyboard changes; respect safe areas and leave zoom gestures intact.
- Follow the visual viewport on narrow screens so the composer and application tabs remain above the software keyboard. Leave pinch zoom and desktop sizing to the browser.

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

## Navigation and loading

- Keep the application shell mounted during internal navigation. Render internal calendars directly; reserve frames for isolated content or external embedding.
- After verifying membership, show a scope-matched task snapshot while refreshing. Its versioned localStorage envelope expires after 24 hours and includes project, company, person and role. Distinguish saved data from a successful fresh read; week selection never renews its age.
- On a fresh visit, show a dimension-matched board skeleton, then real task cards before directory completion. Member-dependent edits wait for the complete state. Storage failure falls back to server loading.
- Invalidate memory and durable task snapshots on writes and logout. Generation checks prevent a pre-write read from repopulating the cache; permission denial removes the snapshot. Reset the workspace on a scope change.
- Week selection derives from loaded task state and must not trigger another full task read. A pending refresh still applies to the selected week.
- Load attendance administration views when selected or intended, warming only the next likely view during idle time. An unread approval inbox must not show a zero badge.
- Calendar keyboard actions yield to settings sheets, command dialogs and picker controls.
