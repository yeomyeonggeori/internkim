# Harness split: blueclaw (host) / bluecollar (harness) / agentcontract

Companion to [`saas-design.md`](./saas-design.md). That document decides *where
things run*; this one decides *what plugs into what*, so the SaaS transition and
the open-sourcing of blueclaw do not have to be done twice.

Status: design agreed, implementation started. Task list at the end.

---

## 1. The three layers

| Layer | Role | Analogue |
|---|---|---|
| **blueclaw** | Host. Owns connectors, POSIX isolation, task store, approvals, capability registry. | openclaw |
| **bluecollar** | Harness. Owns the agent loop: route a turn, run it, answer, classify. | Claude Code / Codex |
| **agentcontract** | The contract both sides compile against: tool descriptors, model ports, task state, the harness port. | — |

blueclaw is the part we open-source. bluecollar goes to a private repo.
`agentcontract` must be public, because blueclaw cannot build without it.

## 2. The harness port is 6 verbs

Measured against the current tree, the host calls 25 methods on the harness.
Only 6 are the harness:

```
RunTurn · RouteTurn · RunAgentRequest
GenerateReply · GenerateReplyWithContext
ClassifyAddressing · ClassifyActiveTaskFollowUp · RefreshSkillIndex
```

The other 19 are not harness concerns:

- **10 are task-store passthrough** (`AppendTaskEvent`, `ListTaskEvent`,
  `FindTaskRun`, `ListTaskRunByPersonID`, `CompleteTask`, `CancelTask`,
  `CancelActiveTasks`, `IsTaskRunActuallyRunning`, `InterruptInactiveTaskRun`).
  Nine are one-line delegations to `taskRunService`; the host should call the
  store directly. `CompleteLaunchFailure` is the exception — it generates an LLM
  failure notice, so it is real harness behaviour and stays.
- **9 are construction config** (`Use*` setters). In a port these collapse into
  one options struct — the same shape as AI SDK's
  `HarnessAgent({harness, instructions, tools, skills, stopWhen})`.

So the swap surface is small. What makes the split *look* big is type ownership:
`AgentTurnRequest`, `AgentRequest`, `TurnDecision`, `VisibleContext`,
`InstructionBundle`, `MemoryFact` and their closure are referenced from 138
files. Go's implicit interfaces need type identity, so those definitions must
live in `agentcontract`. The repo already has a proven pattern for this
(`internal/llm/port.go`, `internal/task/port.go`): move the *definitions*, leave
alias re-exports behind, and the 138 consuming files never change.

## 3. Two ways to run a non-bluecollar agent — they are different layers

This is the reconciliation point with `saas-design.md` §Phase 3.

| | **AI SDK harness adapter** (this doc) | **ACP adapter** (saas-design Phase 3) |
|---|---|---|
| What is replaced | The loop *inside* our host | The whole agent, *outside* our host |
| Who owns identity/tools | blueclaw | The external agent |
| POSIX isolation | Preserved — tools still execute through blueclaw | Not ours to enforce |
| Transport | llmd sidecar, in-process port | Buzz / ACP, as a community participant |
| Use case | "Run this company's agent on Codex's loop" | "Let an employee's Claude Code join the chat" |

They are complementary. Do **not** collapse one into the other. The harness port
must not grow ACP concepts, and `acpd` must not try to drive blueclaw tools.

Implementation of the AI SDK side lives in **llmd**, which is already a
TypeScript sidecar on the AI SDK. Adapters available today:
`@ai-sdk/harness-claude-code`, `-codex`, `-opencode`, `-deepagents`, `-pi`.

```
blueclaw (Go host)
   └── Harness port ────┬── bluecollar        (Go, in-process, private repo)
                        └── llmd HarnessAgent (TS sidecar)
                              └── @ai-sdk/harness-{claude-code,codex,opencode}
```

**Non-negotiable:** whichever harness is selected, tool execution stays in
blueclaw behind the POSIX user/group boundary. The harness decides *what* to
call; blueclaw decides *who* it runs as. An adapter that executes tools in its
own process is not an acceptable integration.

## 4. Loop shape: stop diverging from the standard

Today bluecollar forces one structured **action document** per step:

```json
{"action":"continue","toolName":"...","toolInput":{...},
 "goalStatus":"...","completionEvidence":[...],"qualityCriteria":[...]}
```

Claude Code, Codex and opencode all use native `tool_calls` arrays with
multi-step loops. Consequences of our divergence:

- no parallel tool calls — five file reads cost five turns
- a ~20-field envelope rides on every request, on top of the tool schemas
- portability depends on structured-output quality rather than tool-call support
- an AI SDK harness adapter cannot express it, so the port would need a
  lossy translation layer

Decision: **move to native multi-step tool calling** unless a concrete advantage
justifies the divergence. The completion-evidence and approval gates that
currently ride inside the action document are genuinely valuable and stay — as
separate concerns evaluated around the loop, not as the loop's wire format.

## 4a. The provider axis — audited, mostly already true

`harness-reference-survey.md` and a code audit settled this. Recording it so it is
not re-litigated:

- **"Provider-agnostic via the AI SDK" already holds.** bluecollar reaches models
  through a Go port (`model.LanguageModelProvider`); the production implementation
  is an llmd client, and llmd *is* the AI SDK. `internal/llm/openrouter_client.go`
  is not in the production path — `providerByName` accepts only `capabilityLLM`
  and `llmd`, and admind rewrites the deployed `defaultProvider` to `llmd`
  (`internal/admind/blueclaw_updates.go:837`). The `capabilityLLM` default left in
  `config/runtime.example.json` is stale and misleads readers; fix the example.
- **"Swappable mid-runtime" did not hold, and now does.** The model was bound once
  per turn from `intakeDecision.TaskLevel`; `escalateBudgetTier` raised the task
  level mid-turn but never re-consulted the ladder, so an escalated turn got more
  iterations on the same model. `AgentTurnRunner` now holds a tier→provider
  resolver and re-resolves on escalation, re-applying the `llm.call` observer so
  the ledger survives the swap. Guarded by
  `TestAgentTurnRunnerSwapsLanguageModelWhenBudgetTierEscalates`.
- **Do not upgrade `ai` v6 → v7 for this.** llmd runs no agent loop — zero
  `stopWhen` / `maxSteps` / `prepareStep` in `llmd/src/provider.ts`. It is a
  single-shot completion service; the loop is Go's. `prepareStep` is per-step model
  selection *for a loop the AI SDK owns*, which is not our architecture. Our
  equivalent is sending a different `model` string per call, which the wire already
  accepts. The upgrade is only needed if we adopt `HarnessAgent`, which the survey
  argues against — all four reference harnesses hand-roll their own loop, and
  opencode does so while sitting on the AI SDK.
- **`HarnessAgent` cannot swap models mid-session** (bound at adapter
  construction). It therefore cannot be bluecollar's engine; it stays an optional
  *alternative* harness selectable by a blueclaw host.
- **Open: the Coding tier is structurally unreachable.** It is a model tier
  occupying a task-level slot — there is no `TaskLevelCoding`, so no selector can
  pick it, and its vision fallback is dead. `CodingImageVisionFallbackScenario`
  appears to exercise it and cannot. Decide whether Coding becomes a real
  selection axis or is removed; both touch config and scenarios.

## 5. Terminal entry points

From `saas-design.md` §Phase 2: "starting with `host` mode headless (CLI/package
install) for a spare Linux box". That is the same deliverable as making these
usable from a terminal, so build it once:

- **bluecollar** — a Claude Code–style interactive CLI (session, streaming
  output, tool and approval rendering).
- **blueclaw** — an openclaw-style host CLI: bring the host up, select a
  harness, join a community, show task/approval state.

**TUI stack: Bubble Tea, not termcn.** termcn is a copy-in React component
registry running on Ink (Node) or OpenTUI (Bun) — no Go binding, no tagged
release, and the npm name is a squatted placeholder. Both our binaries are Go, so
termcn would force a TypeScript CLI talking to a Go daemon: two binaries and a
protocol to keep in sync, for a component library. Use
`charm.land/bubbletea/v2` + lipgloss + bubbles, and treat termcn's MIT component
source as a design reference. Precedent: Google rewrote Gemini CLI's Ink/React TUI
into Go + Bubble Tea v2 for Antigravity CLI; opencode went the other way, and it
is a TypeScript product.

Note the ordering dependency: `blueclaw host` headless CLI is on the SaaS
critical path (Phase 2 gate), while the bluecollar interactive CLI is not. If
Phase 2 is the priority, build the blueclaw host CLI first.

## 6. What was applied to saas-design.md

Applied — `saas-design.md` now carries all four:

- **§8 (open source)** — "all source public" carries the exception: bluecollar is
  private, `agentcontract` is public, and blueclaw's self-host path stays green
  with an AI SDK harness instead of bluecollar, so the open-source story has no
  hole where the agent loop should be.
- **§Phase 2** — the client app's harness is selectable at this point, not later;
  building it single-harness first means rewiring in Phase 3.
- **§Phase 3** — retitled to the *external participant* path (ACP over Buzz), and
  it points at §3 above for the distinction from harness replacement.
- **§11 open questions** — added: which harness is the default for a self-hoster
  who does not have bluecollar access?

## 6a. How the boundary is decided

The split is judged by ownership, not by what compiles. A tree that builds
cleanly because loop internals were pushed into the shared package has not
divided anything — it moved the mess and renamed it.

Three rules, in force for every step:

- **`agentcontract` is the vocabulary the two sides speak, never a junk drawer
  both sides import.** Do not add a symbol to it to fix a compile error. If a
  symbol would exist there only to satisfy one host call site, that call site is
  the defect. Test each symbol: must the host *say or hear* this (contract), or
  does the loop *decide* it (loop-owned)? Budget tables, action schemas, plan
  compilation, checkpoint wording, and skill scoring are decisions the loop owns;
  a host reaching for them is reaching wrong.
- **`internal/bluecollar/contract.go` is scaffolding with an expiry date.** It
  exists so ~700 call sites did not have to change at once, and it must end up
  deleted. Between two designs, prefer the one that shrinks it; a design that
  grows it is arguing against itself. Its line count is the honest measure of
  remaining debt.
- **A named unresolved boundary beats a tidy fiction.** If a piece cannot be
  divided without changing behaviour, say so and leave it whole. Do not reach for
  a shape that compiles.

Acceptance, in order:

```
git grep -q 'Dawn-kim-official/bluecollar' -- '*.go'   # empty in blueclaw
test ! -e internal/bluecollar/contract.go              # scaffolding gone
```

The same rule applies to tests. A stub that satisfies `Harness` while the test
it serves no longer asserts on anything real is the same cheat in test clothing:
replace a loop only where the assertion survives the replacement intact.

## 7. Task list

| # | Task | Notes |
|---|---|---|
| 1 | ~~Remove host store-passthrough through the harness kernel~~ **Done** | Landed as blueclaw `ab091d7`: connectors and `TaskLauncher` hold `*taskstate.TaskRunService` directly, the 9 passthrough methods are gone from `AgentKernel`, `CompleteLaunchFailure` stayed. |
| 2 | ~~Extract `agentcontract` (types + harness port)~~ **Done** | 9-method `Harness` port plus the signature-type closure. `toolcontract`, `taskstate`, and `model` also left `internal/` so a separate module can name them. Host `bluecollar.X` references: ~700 → 58. |
| 3 | Split bluecollar into a private repo consuming `agentcontract` | Repo created (private, MIT). Its non-test closure is exactly `agentcontract`, `toolcontract`, `taskstate`, `model`, `jsonschema-go`. |
| 4 | AI SDK harness adapter as a second `Harness` implementation | Blocked on a `HarnessV1SandboxProvider` backed by blueclaw's POSIX helper-exec — without it the adapters' built-in `read/write/edit/bash` run in a sandbox outside our boundary, and `sandbox` is a required field. `pi` is the only host-process adapter and the only one usable as-is. |
| 5 | Terminal entry points, Bubble Tea TUI | See §5. Not started. |
| 6 | ~~Prepare blueclaw for open source (README + docs)~~ **Done** | `README.md` and `docs/architecture.md` written against the code, with an honest gap list. Note they surfaced `internal/access/access.go:22`, a surviving Go-side ACL pre-check that contradicts the POSIX-boundary claim — remove before publishing. |
| 7 | Move the turn loop to native multi-step tool calling | See §4. Scoped in 5 stages. Key finding: the action document is **already** delivered as a forced native tool call, and llmd already returns full multi-call arrays — we drop them with `keepFirstToolCall`. This is plumbing, not a new transport. The `continue` envelope fields are already dead on the native path while still costing schema tokens. |
| 9 | Runtime model swap | ~~Not in the original list~~ **Done** — see §4a. |
| 8 | ~~Keep this document and `saas-design.md` reconciled~~ **Done** | §6 edits applied to `saas-design.md`; keep both in sync on further changes. |

Suggested order: 1 → 2 → 3 unblocks the split; 7 before 4 (an adapter written
against the action-document loop would be thrown away); 5 and 6 last.
