# Harness identity and mechanics map

Companion to [`harness-reference-survey.md`](./harness-reference-survey.md) and
[`harness-split-design.md`](./harness-split-design.md). This one answers a
narrower question: of everything the loop does, which parts are the product and
which parts are plumbing we should be taking from a reference implementation.

## 1. Purpose and pins

The design principle this document records: preserve Bluecollar's identity, and
default to Pi's approach for generic mechanics. Identity is how the loop
understands a work request, tracks follow-ups, judges outcomes against evidence,
chooses a recovery direction, carries company and requester context, and routes
between models on purpose. Mechanics are the read/write/edit/shell interfaces
and their argument conventions, tool registration, result packaging, loop
plumbing, streaming, cancellation transport, and context representation.

Every generic component gets one of three decisions: **reuse** (take Pi's shape),
**adapt** (take Pi's shape with a named change), **retain** (keep ours). Anything
that is not reuse needs a reason in one of four classes: required product
behavior, host security boundary, protocol compatibility, or a measured
difference. A divergence with no reason in those classes is listed in §4 rather
than defended here.

Verified 2026-09-19. Bluecollar is `yeomyeonggeori/bluecollar` at
`9934105c82c5f33babc97fd0161576bd92661a85` (2026-09-18), reached through
`.dependency/blueclaw` at `f41ae448518575d722f1da79355a94e64d6841e7`. Pi is
`@earendil-works/pi-coding-agent` `0.83.0` (upstream `earendil-works/pi`), read
from the local install at
`/opt/homebrew/lib/node_modules/@earendil-works/pi-coding-agent`.

Paths below are relative to those two roots. Bluecollar owns no tools, so the
read/write/edit/shell rows describe the tools blueclaw registers in
`internal/agentruntime/` against the names in
`.dependency/bluecollar/toolcontract/kernel_tools.go`.

Two claims carried in from the 2026-08 measurement session were re-checked
against this revision and no longer hold as written. Bluecollar's README still
lists "native multi-step tool calling" under *What is not here yet*; the code
sends `ParallelToolCalls: true` and collects extra calls through
`batchedNativeAgentActions` (`loop/agent_turn_state.go`). And the loop no longer
forces one structured action per step: the first sample of every step leaves
`tool_choice` unset.

## 2. Identity: what stays ours

| Behavior | Code location | Evidence |
|---|---|---|
| An outcome contract is agreed before work starts and expected results are declared | `loop/outcome_contract.go`, `loop/expected_result.go` | `TestCompletionGateRejectsEvidenceThatMissesDeclaredResultCondition`; `expectedTaskStatus` is a required expectation in `tests/expensive/*.json` |
| A deterministic gate checks recorded facts before `finish` is allowed | `loop/completion_gate.go`, `loop/required_evidence_validation.go` | `TestCompletionGateRejectsSatisfiedFinishWithUnresolvedFailureDebt`, `TestCompletionGateAcceptsZeroRemainingWork` |
| A second model call reads the ledger and can reject the finish with reasons | `loop/completion_judge.go` | `TestCompletionJudgeLedgerIncludesSuccessfulReadsAndWrites`, `TestCompletionJudgeLedgerBudgetsBytesAndNamesDroppedEntries` |
| A failed run reports the failure to the person who asked | `loop/failure_reply.go`, `loop/failure_report_build.go` | `bench/terminalbench/README.md`, "The one bluecollar wins": 91 of 188 grader-failed runs reported as a problem against Pi's 0 of 30 |
| Failure opens typed debt spent from named recovery allowances | `loop/failure_debt.go`, `loop/recovery_planner.go`, `loop/recovery_guidance.go` | `TestAnAlternateRouteThatSucceedsStillClearsTheDebt`, `TestIndependentWorkThatSucceedsDoesNotClearTheDebtOfTheFailedCall` |
| Progress is counted in novel results, not in tool calls | `loop/action_loop_progress.go`, `loop/turn_progress.go` | `TestARepeatedOutputDoesNotCountAsLoopProgress`, `TestActionProgressTrackerStopsAfterThreeActionsWithoutProgress` |
| The time budget is derived from measured per-model iteration cost | `loop/execution_budget.go`, `loop/task_level_profile.go` | `TestExecutionBudgetStartsAfterCompletedRouting`, `TestExecutionBudgetPreservesCallerDeadline` |
| The model is chosen from the difficulty of the work and can escalate mid-turn | `loop/agent_kernel.go` (`UseTaskTierLanguageModels`), internkim `internal/modelladder/model_ladder.go` | `internal/modelladder/model_ladder_test.go`; scenarios pin a tier ceiling (`--maximum-model-tier low`) and never a model |
| The turn carries requester identity, calling name, and company context | `loop/llm_context_builder.go` (`LLMContextInput`) | `TestCompanyContextEmptyStateSaysSoAndAsksRatherThanInventing`, `TestCompanyContextNamesNoToolThatDoesNotExist` |
| The reply is written in the requester's language | `loop/contract.go`, `loop/failure_report_build.go` (`ResponseLanguage`) | `loop/response_language_test.go` |
| A side-effecting call can sit at an approval gate for days and resume | blueclaw `internal/approvalgate/gate.go`, `continuation.go` | `approval.pending_call` / `approval.executed` are required expectations in `tests/expensive/*.json` |
| What an inbound message means is decided before a turn runs | `intake/turn_router.go`, `intake/decision_planner.go` | blueclaw `internal/e2e/intake_decision_llmeval_test.go`; `tests/expensive/06-addressing-response.json` |

None of these rows is a candidate for replacement by a reference
implementation, because none of the reference loops has the behavior. Pi in
particular has no outcome contract, no completion gate, no judge, and no failure
report, which is what the 0 of 30 column measures.

## 3. Mechanics: component by component

| Component | Pi 0.83.0 | Bluecollar today | Decision | Reason class | Evidence |
|---|---|---|---|---|---|
| read | `read {path, offset?, limit?}`; head truncation at 2000 lines or 50 KB; continuation notice names the next `offset`; images returned as inline parts | `read` and `file_read` `{path, startLine?, lineCount?, startByte?, maxOutputBytes?}`; typed result with `nextByte`, `isEndOfFile`, `totalLinesKnown`; images through a separate `image_read` | adapt | product behavior | `internal/agentruntime/file_tools.go:120`; byte-range continuation is what makes minified and single-line files readable, which Pi's line-only offset cannot express |
| write | `write {path, content}` | `file_write {path, content}` | reuse | n/a | `internal/agentruntime/file_tools.go:102`; argument shape is identical to `dist/core/tools/write.js` |
| edit | `edit {path, edits[{oldText, newText}]}`, each edit matched against the original file, uniqueness required | `file_edit {edits[{path, oldText, newText}]}`, same matching and uniqueness rule, path moved into the element so one call spans files | adapt | product behavior | `internal/agentruntime/file_tools.go:174` against `dist/core/tools/edit.js:11`; the only change is batching across files |
| shell / terminal | `bash {command, timeout?}`, executed by the process owner through `cross-spawn` | `shell {command, workingDirectoryPath?, timeoutSecond?, approvalRequired?, approvalReason?}`, executed through `blueclaw-posix-helper` as the requester's UID/GID | retain | security boundary | `internal/agentruntime/requester_shell.go`; the POSIX projection is the boundary the host exists to provide, and the approval fields are how a wide effect reaches a human |
| file tools execution path | Node `fs` calls in the agent process | every file tool runs as the requester through `runRequesterShell` | retain | security boundary | `internal/agentruntime/file_tools.go` lines 239, 282, 388, 734, 1382, 1495, 1577 all route through the shell; no Go-side path resolver remains |
| tool registration and schema shape | `ToolDefinition` objects with typebox schemas, registered per session | `toolcontract.ToolDefinition` with an `InputSchema`, plus a descriptor carrying namespace, visibility, side-effect class, idempotency, completion mode and a result contract | retain | product behavior | `internal/agentruntime/kernel_tool_provider.go`; the gate and judge read the descriptor, and behavior branches on the descriptor rather than on the tool name |
| schema portability | typebox, provider-specific conversion downstream | string enums only, no `$ref`, no `const` in model-facing input schemas; numeric ranges described in prose | retain | protocol compatibility | Gemini drops properties with numeric enums and then rejects the orphaned `required`; `loop/action_schema.go` and `AGENTS.md` carry the rule |
| result packaging | `{content: [{type: "text" | "image", ...}], details?}`, `details` untyped | typed JSON result validated against the tool's declared `OutputSchema`, with `ResultContract.Effects` and `EvidenceCondition` naming which field proves the effect | retain | product behavior | `internal/agentruntime/kernel_tool_contracts.go`; the completion gate matches recorded effects to contract fields, which an untyped `details` cannot support |
| retry | session-layer 3 attempts, 2000 ms base, `2^(n-1)` backoff (2/4/8 s), plus a provider-layer retry | provider-layer only: 3 attempts, 1 s base, exponential, 30 s ceiling, honors `Retry-After` | adapt | measured | `model/openaicompatible/provider.go:253-267`; adopted from Pi after the 08 comparison, `TestRetryDelayHonoursTheEndpointsRequestedWaitWithinTheCeiling` |
| output truncation | fixed 2000 lines / 50 KB, tail for bash and head for read, full output written to an OS temp file whose path is printed in the body | limit derived from the remaining context window (`toolResultLimit`), middle elided, full body kept as a task artifact and spilled through an optional `ToolResultSpillStore` whose locator is printed in the body | adapt | measured | `loop/turn_runner.go:271`, `loop/tool_result_spill.go`; the spilled-path-in-the-body convention is Pi's and was copied, the context-derived limit is ours because a tier can move the model mid-turn |
| context / transcript representation | native message list: assistant turns with `tool_calls`, `tool` role results | same: `toolCallTranscript` emits an assistant message carrying the `tool_calls` array and a `tool` role message per result, keyed by observation ID | reuse | n/a | `loop/agent_turn_state.go:965`; the comment on that function records why a loose-text result was abandoned |
| compaction | threshold auto-compaction plus one compact-and-retry on a context-overflow error | threshold summarization of older observations into a pinned summary at 96k tokens, keeping the 10 most recent, requiring 6 new observations and 20k new characters before it fires again | retain | product behavior | `loop/task_context_summary.go:13-16`; the summary must stay a ledger read the judge can expand, so a rewritten transcript is not available to us |
| cancellation | `AbortSignal` threaded into every tool `execute` | `context.Context` threaded the same way, plus `RegisterTaskRunCancel` on the task store so a cancel can arrive out of band while the turn is running | adapt | product behavior | `loop/turn_runner.go:395-405`; an unattended task must be cancellable by someone who is not holding the request |
| streaming | token streaming from the provider, `onUpdate` callbacks from tools, throttled at the TUI | non-streaming chat completions; `turnstream` publishes ledger events (reply, tool, approval) to a host as they are appended | retain | product behavior | `turnstream/stream.go`; the unit a host renders is a ledger event, and the same events feed the bench, `--trace`, and ACP `session/update` |
| reasoning handling | replayed in the field it arrived in | same replay (`reasoningFieldName` picks `reasoning` or `reasoning_content`), plus a required `reasoning` string property injected into every action tool schema | adapt | measured | `model/openaicompatible/provider.go:144`, `loop/agent_turn_state.go:1083`; the injected slot is the think-then-act change the bench README credits with moving the row from 20/157 to 9/24 |
| tool_choice | never set (provider default `auto`) | unset on the first sample of every step; a correction retry restricts to one tool with `"required"` and `ParallelToolCalls: false` | adapt | measured | `loop/agent_turn_state.go:841`; `TestBuildAgentActionChatRequestExposesDirectToolsAndTerminalControls` asserts the first ask names no `tool_choice`, because naming it even as the default excludes providers that serve tools and reject the parameter, and `TestDecideAgentActionNativeChatRetryRequiresSinglePendingContractTool` asserts the restricted retry |
| malformed tool arguments | `parseStreamingJson` returns `{}` when repair fails, silently | `unreadableModelActionError` is returned and the step is re-asked with a correction message, capped at 2 corrections | retain | product behavior | `loop/agent_turn_state.go:1208`; a silently empty argument object is indistinguishable from a call the model meant to make with no arguments |
| turn / time / token ceilings | none | step, wall-clock and deadline ceilings, one free tier escalation from whichever arrives first | retain | product behavior | `loop/execution_budget.go`; deliberately not copied from Pi |

## 4. Open divergences

These differ from Pi without a reason in the four classes. They are candidates
to revisit, not decisions.

- **Read argument names.** `startLine` / `lineCount` against Pi's `offset` /
  `limit`. The byte-range fields have a reason; the renaming of the line fields
  does not, and it costs description bytes on every step for no behavior.
- **Two read tools.** `read` is model-visible and `file_read` is internal
  (`internal/agentruntime/kernel_tool_provider.go`), with near-identical result
  schemas. Pi has one. No record says what the second one buys.
- **Elision shape.** Pi truncates head (read) or tail (bash) and never removes
  the middle. Bluecollar elides the middle of every oversized result
  (`withMiddleElided`). Nothing measured says the middle is the least useful
  part of a command's output.
- **Retry base delay.** 1 s with a 30 s ceiling against Pi's 2 s. The ceiling and
  the `Retry-After` handling are ours on purpose; the base delay difference
  looks incidental.
- **No search tools.** Pi ships `grep`, `find` and `ls` as first-class tools with
  their own truncation rules; blueclaw expects the model to reach them through
  `shell`. This was never argued either way, and the bench README's finding that
  help text is 79% of bluecollar's tool output bytes is circumstantial evidence
  it matters.
- **Non-streaming provider.** Everything downstream consumes ledger events, so
  the turn does not need token streaming, but no record says whether that was
  decided or simply never built.
- **The Pi reference is not pinned.** `bench/terminalbench/pi_agent.py` defaults
  its `version` to `latest`, so `pi-setup.sh.j2` installs whatever npm serves on
  the day. Every published comparison row therefore names a Pi that cannot be
  reconstructed. This document pins 0.83.0; the bench should pin the same value
  and record it in the row.
- **Correction cap.** Two corrections per step
  (`maximumAgentActionCorrectionCount`) against Pi's per-call argument repair
  with no cap. The number is not derived from anything.
