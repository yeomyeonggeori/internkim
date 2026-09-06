# Agent learning design

The learning store, admin routes, company bridge mappings and web views are wired
for read-only review and explicit administrator controls. Learning remains
disabled by default and no production deployment is claimed here. It extends
[memory architecture](memory-design.md).

The central-plane `person.agent_learning.*` capabilities carry typed skill,
settings and soul reads and explicit skill actions. The persona signature binds
the complete request target; user reads and writes require the matching employee
identity, while initial seeding has a separate POST-only principal. Linux
permission verification and a complete company-plane run remain separate
deployment gates.

The product should improve at working with employees through retained context,
individual preferences, grounded working principles and reusable procedures.
Each has a separate owner and update path.

| Layer | Content | Writer |
| --- | --- | --- |
| `identity.json` | Company-selected name, role and presentation | Authorized human settings |
| `user.json` | One employee's stable preferences and collaboration profile | That employee and their conversation runtime |
| Memory | Scoped facts, decisions and context with sources | Authorized conversation tools and human controls |
| `soul.json` | Shared working principles learned across experiences | Internal reflection runtime |
| Learned skills | Reusable procedures supported by work evidence | Internal reflection runtime |

## Learning cycle

Completed tasks contribute immutable evidence references: the request, actual
tool effects, outcome checks, user corrections and skills loaded. A task marked
completed is insufficient proof that its result was correct. Unknown outcomes
remain unknown. Ordinary conversations can contribute observations without
producing a skill or changing the shared soul.

An internal scheduler batches unreviewed evidence. Proposed initial settings are
a minimum one-hour interval, five minutes without a foreground task, at most
three review batches per company per day, and at most twenty task records per
batch. No new evidence means no model call. A review has a fixed model-call and
token budget, a deadline, and a durable cursor. The administrator can disable
learning without disabling recall or existing skills. These are cost controls;
the model decides whether any experience merits learning.

The reviewer receives current principles, relevant existing skills, recorded
outcomes and source references as data. It produces a closed typed decision:
keep, create, revise, merge or retire, with affected exact IDs, expected revisions,
evidence IDs and rationale. Choosing keep is normal. A correction may improve a
procedure; a direct request to rewrite the shared personality does not itself
establish a durable working principle. Individual preferences stay individual.

The reviewer may learn from every employee. It operates within one authorized
audience per evidence batch. A private procedure stays private; combining sources
cannot broaden their audience. Shared principles contain no employee-specific
facts, confidential procedure details or verbatim conversation. Generalization
across audiences needs a separate reviewed promotion path; version one does not
automatically publish private learning to the company.

## Invocation and storage boundary

The server creates the reflection execution identity. No user-supplied task kind,
tool argument or forwarded requester claim can select it. Foreground tools can
read applicable principles and skills but cannot mutate either. The existing
general `persona_update` soul target must be removed from foreground execution,
including administrator conversations. Profile updates remain available there.

Prefer a private in-process capability when the reviewer and writer share a
process. If a separate service is required, use a scoped, expiring credential
bound to one internal run and audience. Neither credential nor writer endpoint
is exposed through the general tool catalog, user API or terminal environment.
Calling a tool an agent tool does not create this boundary.

The runtime service owns canonical soul and learned-skill storage. Foreground
POSIX identities receive immutable read-only projections. Parent directories,
symlink replacement, arbitrary file tools, terminal commands and accessible
administrative endpoints must all obey the same boundary. A same-UID unrestricted
terminal would invalidate the design even if the dedicated API checked a token.

The writer validates schemas, evidence scope, ownership, size, quota and expected
revision. It cannot send messages, execute a work procedure, change permissions
or change company settings. Learned procedures execute later under the current
requester's ordinary permissions and existing approval rules.

## A small learned-skill library

Propose twenty active self-authored skills per company initially, configurable
by an administrator. Private and shared learned skills count toward this total.
Bundled and human-installed skills occupy a separate protected collection and
cannot be modified or evicted by the reviewer. A protected learned skill still
counts toward the cap. If every slot is protected, creation is declined until
capacity is available; successful current work does not depend on saving a skill.

Prefer updating an applicable skill over creating a near-duplicate. At capacity,
the model receives eligible skills and observed use, outcome, age and dependency
facts, then chooses a justified merge or replacement. Similarity can retrieve
candidates but cannot decide equivalence or deletion. Low use alone is not proof
that a rare procedure is expendable. Active executions and scheduled references
protect the referenced version from removal.

Validate a replacement package before atomically switching the catalog revision
and retiring its predecessor. Concurrent writers must never exceed the cap or
overwrite a newer revision. A failed replacement leaves the old skill usable.
Retired versions leave discovery immediately and remain recoverable for thirty
days within a bounded archive budget. Restoring a version also respects capacity.

Each skill has a stable ID, audience, version, trigger description, required
inputs, procedure, outcome checks, recovery limits and linked evidence. Usage
and verification records belong in metadata rather than its instructions. Avoid
copying task-specific names, dates, secrets or results into a general procedure.

Version one authors `SKILL.md` plus references and templates using existing
tools. Executable helper generation needs a separate sandbox verification path
before activation. The main document targets less than 8 KB and obeys the
repository's hard 15 KB / 300-line limit. Supporting files have a total package
budget so the active-count limit cannot hide unlimited prompt or disk growth.

New procedures undergo an independent model assessment against their source
effects before activation. The UI labels that state as evidence reviewed.
Schema validation establishes loadability. Neither check establishes successful
execution on a new task. Behavioral verification is a separate state, supported
by an isolated execution with changed inputs. No production connector action is
performed merely to validate a generated procedure. Activated skills retain the
exact evidence and version used in subsequent work.

Learning starts disabled in the implementation. An authenticated administrator
can change its persisted setting when the lifecycle is integrated and verified.
The review model uses the existing configured low-tier provider. The planned
initial integration retains procedures within the originating employee's
audience; shared procedure promotion is outside that first integration.

Discovery returns only an authorized compact catalog or retrieved candidates.
The model loads the selected body, and then supporting files as needed. Creation,
revision and retirement update the existing skill index. Running tasks keep
their selected version; the next task sees the new catalog.

## Product surface

Use three clear areas within the agent workspace: remembered information,
learned procedures, and working principles. Keep personal preferences in the
employee profile and company identity in company settings.

The learned-procedure view uses the memory workbench's list and detail pattern.
Show a quiet capacity label such as “12 / 20”, procedure purpose, audience,
last use and verification state. Avoid scores presented as intelligence or
growth. The detail view explains when it applies, the procedure, why it was
learned, observed outcomes and version changes. Evidence visibility follows its
original permissions even when a procedure is visible more broadly.

Authorized humans can disable, protect, retire and restore learned procedures.
Working principles show current text, changes and their reasons. Learning can be
paused through its persisted administrator setting. Soul history is read-only;
there is no human soul restore control in this surface.

## Acceptance evidence

- A foreground request cannot invoke a soul or skill writer through tools,
  fabricated execution kinds, direct HTTP, file replacement or terminal access.
- Own-profile updates still persist and load on a new task.
- Repeated synthetic work produces a useful candidate; ordinary chatter and
  unsupported success claims do not force creation.
- A held-out task with changed inputs succeeds using the learned procedure.
  Compare against the same task without it: result quality, tool calls, latency
  and tokens. Preserve live model requests and execution evidence.
- Overlapping routines are revised or merged when appropriate. Distinct routines
  retain their distinctions. These judgments require live-model evaluation.
- Concurrent creation respects twenty slots. Failed replacement, stale revision,
  protected skill and in-flight references preserve the usable version.
- Private evidence cannot become a discoverable company skill. Disabled learning
  produces no review calls or mutations; existing skill use continues.
- Restart preserves the evidence cursor, catalog and evolved soul. The UI shows
  actual persisted state, errors and unverified outcomes.

Runtime acceptance follows the repository's simulation and Local Fleet Linux
gates. Model judgment requires the approved isolated live path. No production
conversation is an acceptance fixture.

## Integration with the current runtime

`internal/agentruntime/skill_management.go` already provides foreground
`skill_add` and `skill_remove` for requester-managed workspaces, validates
packages and protects bundled names. Its production service-owned `/workspace`
guard prevents using this path as the proposed company learning service. Keep
explicit requester-owned authoring distinct from internal company learning;
neither path may write the other's collection or bypass the company quota.

`internal/app/agent_instructions.go` discovers `.agents/skills` and bundled
roots. `internal/app/agent_kernel.go` supplies `newSkillIndexRefresher`, wired
through `internal/app/tool_catalog.go` after existing skill mutations. Extend
this loader and refresh contract with audience-filtered immutable learned
versions rather than introducing a second independent skill discovery system.

`runTurnLaunchStep.Run` in `internal/agentruntime/task_launcher.go` observes
the model result, before connector reply delivery. It can enqueue evidence
references, but must not label delivery successful there. Later effect and
delivery events complete the evidence. A tracked application lifecycle worker
owns review scheduling; ordinary completion should not wait for a review.
There is currently no autonomous reflection worker. Existing connector tests
reject automatic memory ingestion; preserve that distinction by referencing
task evidence without indiscriminately copying messages into semantic memory.

## Reference and deliberate choices

Hermes provides on-demand skill loading and create, patch and delete operations
through its [skills system](https://hermes-agent.nousresearch.com/docs/user-guide/features/skills/).
Its [curator](https://hermes-agent.nousresearch.com/docs/user-guide/features/curator)
tracks use, archives recoverably and supports consolidation. Its
[skill manager source](https://github.com/NousResearch/hermes-agent/blob/main/tools/skill_manager_tool.py)
also implements frontmatter validation and guarded file writes.

This proposal adds a company-wide active cap, audience-aware evidence, internal
authoring authority, atomic replacement and explicit behavioral verification.
It does not adopt prose keyword scanning as a security or semantic decision
mechanism, automatic age-based eviction, or unrestricted foreground authorship.
