# Harness feature correspondence

This document meets issue #1789's "Correspondence table for all current
features and the finalized default tool contract" completion criterion. It
maps every feature class named in that issue to the tool or skill that
currently answers it, derived by reading the canonical catalog, the plugin,
Bluecollar's kernel tool set, and the test suites as they stand today.

Revisions this table was derived from: this worktree at
`b3b1406e3656b444e934429b75dedb0810ce49ef`, `.dependency/blueclaw` at
`f41ae448518575d722f1da79355a94e64d6841e7`, `.dependency/internkim-plugin` at
`c1e156ba9ee4cb698f8b32cda6eca1bbbb09c473`, and the tool catalog's
`protocolVersion` `0.4.0` (`pkg/capabilityprotocol/generated/capability-tools.json`,
matching `internkim-plugin`'s `plugin.json` version).

## Default tool contract

Three layers compose what a model actually sees.

**Bluecollar's kernel tools**, fixed by the harness itself and independent of
InternKim (`.dependency/blueclaw/.dependency/bluecollar/toolcontract/kernel_tools.go`,
`KernelToolNames()`): `shell`, `read`, `file_read`, `file_write`, `file_edit`,
`file_delete`, `file_preview`, `file_deliver`, `skill_search`, `image_read`,
`conversation_history`, `plan_update`, `request_tools`: 13 tools. Bluecollar
also defines `ask_input`, `ask_confirm`, and `ask_choice` as tool-name
constants, but only `ask_input` is registered as a callable tool
(`.dependency/blueclaw/internal/agentruntime/ask_tools.go`); `ask_confirm` is
reused as the interaction *kind* label the approval gate stamps on a pause
(`.dependency/blueclaw/internal/approvalgate/gate.go:172`) rather than
registered as a second tool, and `ask_choice` is folded into `ask_input`'s
`options` field.

**Blueclaw's default allowlist**
(`.dependency/blueclaw/internal/agentruntime/tool_catalog.go:272`,
`DefaultAllowedToolNames()`) is exactly `KernelToolNames()` plus `ask_input`:
14 tools, the fallback ceiling when no agent-profile override applies
(`internal/app/tool_catalog.go:95`, `deriveAllowedToolNames`, which only ever
admits kernel tool names, so this ceiling gates the native tool set, not the
InternKim catalog).

Beyond that ceiling, Blueclaw registers seven more native tools when their
dependency is configured: `memory_remember`, `memory_search`, `memory_forget`
(only when a memory store is wired in, `tool_catalog.go:293`), `persona_read`,
`persona_update`, `skill_add`, `skill_remove`. The generated
`.dependency/blueclaw/docs/tool-catalog.md` lists all 20 of these
Blueclaw-native tools (plus one placeholder row for "named by the device
catalog") with their description byte cost; regenerate it with
`BLUECLAW_WRITE_CATALOGS=1 go test ./internal/catalog/`.

**The InternKim capability catalog** is spliced in separately
(`registerRecordCatalogTools` / `registerCapabilityTools` in
`tool_catalog.go:234`) and is not gated by the kernel allowlist. It holds 108
tools: 73 `answeredBy: record`, 26 `company`, 9 `local`
(`pkg/capabilityprotocol/generated/capability-tools.json`). Of the 99
non-local tools, 97 carry `modelVisibility: visible`; `document_read` and
`image_read` are `hidden` and are fetched by the runtime for attachment
ingestion instead of offered to the model as a tool choice.

**Visibility and permission rule**
(`web/src/lib/server/public-api/catalog.ts`): a tool is model-visible iff
`modelVisibility === "visible"`. Its public-API permission is derived from
`sideEffectClass`, not stated separately: `read`/`computation` → `read`
(39 tools currently), `destructive` → `delete` (12 tools), everything else
(`connect`, `external_send`, `external_write`, `site_publish`,
`workspace_write`) → `write` (57 tools). `requiresApproval: true` marks 24
tools whose effect the approval gate must pause on before executing
(`task_delete`, `person_invite`, `team_delete`, `event_delete`,
`leave_update`, `leave_delete`, `leave_decide`, `leave_grant_set`,
`attendance_update`, `attendance_delete`, `message_send`, `message_delete`,
`site_unserve`, `company_record_delete`, `crm_organization_archive`,
`crm_contact_archive`, `crm_opportunity_move`, `crm_opportunity_archive`,
`company_settings_update`, `company_holiday_delete`,
`attendance_work_policy_set`, `attendance_leave_policy_set`,
`mail_connection_start`, `mail_message_send`); the 9 `local` (browser and
desktop) tools instead carry `requiresUserPresence: true` and an
`approvalScope` of `browser` or `desktop`, since they hand off to a person at
a keyboard rather than pausing for a policy decision.

So a fully-provisioned company session's default surface is at most
14 always-on native tools, up to 20 native tools where memory/persona/skill
management are wired in, 97 model-visible catalog tools filtered further by
the company's held public-API permission, and up to 9 `local` companion
tools when a companion browser or desktop is connected.

## Correspondence table

"Answered by" follows the catalog's own vocabulary: `record` (InternKim's own
database), `company` (a service on the company's machine, e.g. mail, web
search, schedules), `local` (the companion beside the requester),
`harness-native` (Bluecollar's own kernel/host tool, no InternKim catalog
entry). "harness-native only" marks a feature an external harness (Claude
Code, Codex, Pi) does not get merely by installing `internkim-plugin`,
because nothing in the plugin's skills names it.

| Feature | Tools / skills | Answered by | Permission and approval | Regression scenario |
|---|---|---|---|---|
| File discovery | `shell` (e.g. `rg`, `find`) | harness-native | `workspace_write`, no approval | `.dependency/blueclaw/internal/agentruntime/shell_tools_test.go` |
| File reading | `file_read`, `read`, `image_read` | harness-native | `read`, no approval | `.dependency/blueclaw/internal/agentruntime/kernel_tool_provider_test.go` (`TestKernelToolProviderUsesCanonicalDescriptors`) |
| File writing / editing / deleting | `file_write`, `file_edit`, `file_delete`, `file_deliver` | harness-native | `workspace_write`/`external_write`, no approval | `.dependency/blueclaw/internal/e2e/virtual_session_test.go` (`TestFileWriteAcceptance`, `TestFileWriteAcceptanceRejectsWrongPersistedContent`) |
| Terminal execution | `shell` | harness-native | `workspace_write`, runs as the requester's POSIX identity, no approval | `.dependency/blueclaw/internal/agentruntime/shell_tools_test.go` (`TestTerminalRun*`), `.dependency/blueclaw/internal/security/posix_identity.go` |
| Long-running sessions | none; `shell` is one stateless command per call (`TimeoutSecond` field only) | harness-native | same as terminal execution | none found |
| Dependency installation | none; runs through `shell` like any other command | harness-native | same as terminal execution | none found |
| Web search | `web_search` | company | `read`, no approval | `internkim-plugin` skill `web-search`; no dedicated scenario found |
| Web page reading | `web_fetch` | company | `read`, no approval | `internkim-plugin` skill `web-search`; no dedicated scenario found |
| Browser manipulation | `browser_open`, `browser_click`, `browser_fill`, `browser_press`, `browser_select`, `browser_wait` | local | `connect`/`external_write`, `requiresUserPresence: true`, `approvalScope: browser` | `internkim-plugin` skill `website`; no dedicated scenario found |
| Screenshots | `browser_screenshot`, `browser_snapshot` | local | `read`, `requiresUserPresence: true` | `internkim-plugin` skill `website` |
| User handoff (desktop) | `computer_task` | local | `external_write`, `requiresUserPresence: true`, `approvalScope: desktop` | none found |
| Task (work items) | `task_add`, `task_list`, `task_update`, `task_delete`, `task_vocabulary_set` | record | `read`/`write`/`delete`; `task_delete` requires approval | `internkim-plugin` skill `internkim-task`; `web/tests/integration/record-tools.test.ts`; `web/tests/plane/the-agent-discovers-the-record-tools.test.ts` |
| Calendar (events) | `event_add`, `event_list`, `event_update`, `event_delete` | record | `read`/`write`/`delete`; `event_delete` requires approval | `internkim-plugin` skill `calendar`; `web/tests/integration/record-tools.test.ts`, `web/tests/integration/calendar-feed.test.ts`; `.dependency/blueclaw/internal/e2e/virtual_session_test.go` (`TestVirtualCalendarMutationUsesExactEventHint`) |
| Scheduled work (reminders, recurring/finite tasks) | `schedule_create`, `schedule_list`, `schedule_update`, `schedule_cancel` | company | `read`/`write`, no approval | `internkim-plugin` skill `scheduled-task`; `internkim-task` skill also refs `schedule_create`; localfleet scenarios `schedule-through-the-catalog`, `firing-schedules-nothing` (`internal/localfleet/service.go`, run via `./internkim dev fleet run --scenario <name>`) |
| Attendance | `attendance_add`, `attendance_list`, `attendance_update`, `attendance_delete`, `attendance_work_policy_get/set`, `attendance_leave_policy_get/set` | record | `read`/`write`/`delete`; update/delete/policy-set require approval | harness-native only, no plugin skill references any `attendance_*` tool; `web/tests/integration/attendance-tools.test.ts`; ~60 unit tests under `web/tests/unit/attendance/` |
| Leave / HR entitlement | `leave_balance`, `leave_list`, `leave_request`, `leave_update`, `leave_delete`, `leave_decide`, `leave_grant_set`, `leave_return_early` | record | `read`/`write`/`delete`; update/delete/decide/grant-set require approval | harness-native only, no plugin skill references any `leave_*` tool; `web/tests/integration/leave-tools.test.ts`, `leave-year-share-conformance.test.ts` |
| Organization directory / HR profile | `person_list`, `person_update`, `person_invite`, `team_add`, `team_list`, `team_update`, `team_delete` | record | `read`/`write`/`delete`; `person_invite` and `team_delete` require approval | `person_list` only via `internkim-task` skill; `person_update`/`person_invite`/`team_*` are harness-native only, no skill references them; `web/tests/integration/people-and-team-tools.test.ts` |
| CRM (contacts, organizations, opportunities, activities) | `crm_contact_*`, `crm_organization_*`, `crm_opportunity_*`, `crm_activity_*`, `crm_vocabulary_*` | record | `read`/`write`/`delete`; the three `*_archive` and `crm_opportunity_move` require approval | harness-native only, no plugin skill references any `crm_*` tool; `web/tests/integration/crm-tools.test.ts`; ~21 unit tests under `web/tests/unit/crm/` |
| Company profile, metrics, records, holidays, settings | `company_info_*`, `company_metric_*`, `company_record_*`, `company_holiday_*`, `company_settings_*` | record | `read`/`write`/`delete`; `company_record_delete`, `company_holiday_delete`, `company_settings_update` require approval | `company_info_*`, `company_metric_*`, `company_record_*` via `internkim-plugin` skill `company-data`; `company_holiday_*` and `company_settings_*` are harness-native only; `web/tests/integration/company-ledger-tools.test.ts`, `company-settings-tools.test.ts` |
| Company documents (data room) | `company_document_list/search/download/register/update/upload` | record | `read`/`write`, no approval | `internkim-plugin` skills `dataroom`, `paperwork`; `web/tests/integration/company-ledger-tools.test.ts` |
| Mail (connected email) | `mail_connection_start/status`, `mail_message_list/search/read/send/mark/move` | company | `read`/`write`/`external_send`; `mail_connection_start` and `mail_message_send` require approval | `internkim-plugin` skill `mail`; ~17 unit tests under `web/tests/unit/mail/`; no integration or scenario test found |
| Messaging and recipient identification | `message_context`, `message_search`, `message_send`, `message_update`, `message_delete` | company | `read`/`write`/`external_send`/`delete`; `message_send` and `message_delete` require approval; recipients resolve server-side from a hint field, never a model-supplied ID | `internkim-plugin` skills `messages`, `direct-message`; `web/tests/plane/dm-to-a-colleague.test.ts`, `a-dm-through-the-acp-session.test.ts`, `a-channel-post-through-the-acp-session.test.ts`, `somebody-just-invited.test.ts`; `.dependency/blueclaw/internal/e2e/virtual_session_test.go` (`TestVirtualMessageToolsUseGeneratedCanonicalContracts`) |
| Documents (.docx/.pdf) | local skill scripts (no catalog tool); attach through `file_deliver` | harness-native | `external_write`, no approval | `internkim-plugin` skills `document`, `pdf`; `tests/expensive/09-document-lifecycle.json` (retained spec, not run, see Gaps) |
| Spreadsheets (.xlsx) | local skill scripts; attach through `file_deliver` | harness-native | `external_write`, no approval | `internkim-plugin` skill `spreadsheet`; no scenario found |
| Presentations (slides/.pptx) | local skill scripts; attach through `file_deliver` | harness-native | `external_write`, no approval | `internkim-plugin` skill `presentation`; `tests/expensive/03-presentation-lifecycle.json` (retained spec, not run); `.dependency/blueclaw/internal/e2e/presentation_assets_test.go` |
| Images | `image_generate` (creation), `image_read` (hidden, attachment ingestion) | company / harness-native | `external_write` for `image_generate`, no approval | no plugin skill references `image_generate`; no scenario found |
| Sites (websites/prototypes) | `site_list`, `site_serve`, `site_unserve`, `artifact_review` | company | `read`/`write`/`delete`; `site_unserve` requires approval | `internkim-plugin` skill `website`; `tests/expensive/04-website-lifecycle.json` (retained spec, not run); `.dependency/blueclaw/internal/e2e/virtual_session_test.go` (`TestVirtualSiteServeRequiresValidSourceBundle`, `TestVirtualSiteToolsUseCanonicalThreeToolContracts`) |
| Skill discovery | `skill_search`, `skill_add`, `skill_remove`, `request_tools` | harness-native | `read`/`workspace_write`, no approval | `.dependency/blueclaw/internal/agentruntime/skill_search_tool_test.go`, `skill_management_test.go`; `web/tests/plane/the-agent-can-see-its-skills.test.ts`, `the-agent-can-see-the-tools-it-was-given.test.ts` |
| Memory | `memory_remember`, `memory_search`, `memory_forget` | harness-native | `workspace_write`/`read`, no approval; conditional on a configured memory store | harness-native only, no InternKim plugin path exists for memory; `.dependency/blueclaw/internal/agentruntime/memory_store_tools_test.go`; localfleet scenario `memory-store`, `lab/scripts/scenario-memory-store.py` |
| Conversation history | `conversation_history` | harness-native | `read`, no approval; conditional on a configured history provider | harness-native only; none found |
| Scheduled execution (the schedule firing itself, as distinct from the CRUD tools above) | host cron/poller, no model-facing tool | n/a | n/a | localfleet scenario `firing-schedules-nothing`, `lab/scripts/scenario-firing-schedules-nothing.py` |
| User questions | `ask_input` | harness-native | `read`-like interaction, `requiresUserPresence` implicit in pausing the turn | `.dependency/blueclaw/internal/agentruntime/ask_tools_test.go` |
| Approvals | approval gate wraps any `requiresApproval: true` tool call, presented as an `ask_confirm`-kind interaction; resumed by `/admin/api/run/approve` or a messenger button | harness-native | pauses the task in `waiting_approval` until a recorded, authenticated decision | `.dependency/blueclaw/internal/approvalgate/{gate,turn_gate,continuation,executed_call,permission_asker,target,wording}_test.go` |
| Composite work across multiple tools | no dedicated tool; the loop's own planning (`plan_update`) and tool sequencing | harness-native | n/a | localfleet scenario `morning-briefing`, `lab/scripts/scenario-morning-briefing.py` |
| Follow-up instructions / long conversations | `conversation_history`, memory-guided follow-up handling in the loop | harness-native | n/a | `.dependency/blueclaw/internal/e2e/virtual_session_test.go` (`TestMemoryGuidedFollowup`, `TestRequestRevisionBurstProducesOneReply`) |
| Cancellation | task-run cancel transition (no model tool; triggered by the requester or an admin action) | harness-native | task moves to a terminal cancelled state, remaining work stops | `.dependency/blueclaw/.dependency/bluecollar/taskstate/task_run_service_test.go` (`TestTaskRunCancelCallsRegisteredCancelFunction`, `TestCancelledTaskRunCannotComplete`, `TestCancelTaskRunTransitionAllowsActiveTaskRuns`, `TestCancelTaskRunTransitionRejectsIllegalSourceStates`) |
| Recovery after failure / restart | auto-resume of interrupted task runs (no model tool) | harness-native | resumes at most once per interruption, expires if nobody resumes it | `.dependency/blueclaw/.dependency/bluecollar/taskstate/task_run_service_test.go` (`TestAdvanceTaskRunAllowsBlockedResume`, `TestClaimInterruptedTaskRunAutoResumeAllowsOnlyOneAttempt`, `TestAnInterruptNobodyResumesEventuallyExpires`); localfleet scenario `restart-policy-survival`, `task-history-retry`, `lab/scripts/scenario-task-history-retry.py` |

## Gaps

- Attendance, leave, CRM, `person_update`/`person_invite`, `team_*`,
  `company_holiday_*`, `company_settings_*`, and `image_generate` are
  model-visible catalog tools with no `internkim-plugin` skill naming them
  (`web/scripts/plugin-tool-provenance.ts` output: 43 of the 97 model-visible
  catalog tools are referenced by a skill; the other 54 include all of the
  above). An external harness that only installs the plugin has no written
  instruction for these; today a caller depends on the tool's own JSON
  Schema description and, for Bluecollar, on prompt material outside the
  plugin. A search of `.dependency/blueclaw` for any of these tool names
  outside generated code and tests found none, so no such instruction
  currently exists there either.
- Mail has a plugin skill but no integration or scenario-level regression
  test, only unit tests under `web/tests/unit/mail/`.
- Long-running terminal sessions and dependency installation have no
  dedicated mechanism or test: `shell` is one stateless command per call
  (`.dependency/blueclaw/internal/agentruntime/shell_tools.go`), so a
  background process or a package install is only as reliable as one `shell`
  call's timeout.
- `tests/expensive/*.json` (task, calendar, presentation, website, message,
  addressing, two person-capture, document, and file lifecycle) are retained
  as a specification only. Per `tests/expensive/README.md`, nothing runs
  them: their Mattermost driver was retired, four of the ten still assert
  Mattermost-specific fields, and none of them has a decided meaning for "the
  user approved" against Buzz. The live acceptance gate today is
  `./internkim dev fleet run --scenario <name>` against the names in
  `internal/localfleet/service.go`'s `scenarioPlanBuilders()`.
- No regression scenario, of any kind, was found for: file discovery,
  long-running sessions, dependency installation, web search, web page
  reading, browser manipulation, screenshots, desktop handoff (`computer_task`),
  spreadsheets, images, and conversation history.
