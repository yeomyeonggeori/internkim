# Harness reference survey

Research companion to [`harness-split-design.md`](./harness-split-design.md) and
[`saas-design.md`](./saas-design.md). Verified 2026-08-03 against npm registry
metadata, package `.d.ts` files unpacked from the published tarballs, and the
official docs. Anything not confirmed from a primary source is marked
**UNVERIFIED**.

---

## 0. What this changes about our plan

| # | Claim in `harness-split-design.md` | Status | What is actually true |
|---|---|---|---|
| 1 | "Adapters available today: `@ai-sdk/harness-claude-code`, `-codex`, `-opencode`, `-deepagents`, `-pi`" | **Correct** | All five exist and are published. Versions in §1. Also announced but not yet published: `-amp`, `-goose`, `-mastra`. |
| 2 | "`HarnessAgent({harness, instructions, tools, skills, stopWhen})`" | **Correct but incomplete** | All five fields exist. The doc omits the one field that decides feasibility: **`sandbox` is required, not optional.** |
| 3 | "**Non-negotiable:** tool execution stays in blueclaw behind the POSIX boundary. An adapter that executes tools in its own process is not an acceptable integration." | **Half satisfied** | *Custom* tools we supply execute on the host — the contract is explicit: "The adapter does not execute the tool … the adapter emits a `tool-call` event and waits for `submitToolResult`." But each adapter also ships **built-in** `read/write/edit/bash/grep/glob/webSearch` that execute **inside the sandbox**, outside our POSIX boundary. They can be denied via `builtinToolFiltering`, but then the harness is a loop with no file or shell access of its own. |
| 4 | Implied: this is a drop-in for a self-hosted blueclaw | **Wrong today** | Four of five adapters (`claude-code`, `codex`, `opencode`, `deepagents`) are *bridge* adapters: they install a Node bridge **inside a sandbox** and talk to it over a WebSocket on a sandbox-proxied port. The only supported port-exposing sandbox provider today is `@ai-sdk/sandbox-vercel` (Vercel Sandbox, a hosted cloud service). The local provider `@ai-sdk/sandbox-just-bash` explicitly **has no ports**, so it cannot back them. A customer-run, outbound-only host box therefore cannot run these four out of the box. |
| 5 | Implied: `pi` is one option among equals | **Sharpen** | **`pi` is the only adapter that runs in the host process** ("Pi runs in the host Node.js process and uses the sandbox as a remote filesystem + shell — no bridge process is installed inside the sandbox"), and the only one that works with a non-port sandbox. It is the single adapter compatible with our constraint today without extra work. |
| 6 | §3 table: ACP means "POSIX isolation: **not ours to enforce**" | **Wrong** | ACP puts filesystem and terminal execution on the **client** side, not the agent side: `fs/read_text_file`, `fs/write_text_file`, `terminal/create`, `terminal/output`, `terminal/wait_for_exit`, `terminal/kill`, `terminal/release` are all Agent→Client calls the client implements. If blueclaw is the ACP *client*, our POSIX boundary holds exactly as it does with our own harness. This makes ACP a real candidate for the harness port, not only for the "external participant" path. |
| 7 | §2 "The harness port is **6** verbs", then lists 8 names, then says `CompleteLaunchFailure` stays | **Cosmetic error** | `agentcontract/harness.go` at HEAD has **9** methods. Fix the number in the doc. |
| 8 | Not mentioned | **New constraint** | `@ai-sdk/harness@1.0.54` depends on `ai@7.0.48`. `llmd` pins `ai@6.0.224`. Adopting HarnessAgent means an `ai` v6 → v7 upgrade in llmd, which also renames `stepCountIs` → `isStepCount` and promotes `experimental_prepareStep` → `prepareStep`. |
| 9 | §4 "move to native multi-step tool calling" | **Reinforced, with a concrete mechanism** | `prepareStep` in `ai` v7 can return `{ model }` per step. That is the direct replacement for our tier-escalation ladder, and it only exists on the `generateText`/`streamText`/`ToolLoopAgent` path — **not** on `HarnessAgent`. |
| 10 | §5 "TUI via `shadcn-labs/termcn`" | **Not implementable as written** | termcn is a copy-in **React** component registry on Ink (Node) or OpenTUI (Bun). No Go binding, no Go port, no versioned release, no git tags. Both our CLIs are Go. Recommendation in §4: build on `charm.land/bubbletea/v2` and use termcn's MIT component source as design reference only. |

| 11 | §4 "Claude Code, Codex and opencode all use native `tool_calls` arrays with multi-step loops" | **Correct, and stronger than stated** | All four reference harnesses use native tool calls **and hand-roll their own loop**. opencode is built on the AI SDK and still writes its own `runLoop` — zero uses of `stopWhen`/`stepCountIs`. So "adopt native tool calling" and "adopt someone's loop" are separate decisions; do the first, not the second. |
| 12 | §5 CLI framework choice | **Two live data points, opposite directions** | opencode **deleted** its Go/Bubbletea TUI (101 Go files → 0) in favour of Solid.js on OpenTUI. Google **rewrote** Gemini CLI's Ink/React TUI into Go + Bubble Tea v2 for Antigravity CLI, which is the newest of the four. `charmbracelet/crush` (Go, ~27k stars, active) is the maintained descendant of the old Go opencode and is the better Go TUI reference to read. |

**The one-line consequence.** The AI SDK harness abstraction is real and the
custom-tool execution model matches our non-negotiable. What does not match is
its *deployment* assumption: it is built for a cloud host renting a Vercel
Sandbox, not for a customer's spare Linux box that already has a POSIX
isolation boundary. If we want it, the work is not "write an adapter" — it is
**write a `HarnessV1SandboxProvider` backed by blueclaw's POSIX helper-exec**,
so that even the adapters' built-in `bash`/`write` land inside our boundary
instead of outside it. That provider is ~one interface with two methods
(`createSession`, `resumeSession`) and is the highest-leverage piece of work in
this whole area.

---

## 1. AI SDK harness adapters

### 1.1 Do they exist

Registry data from `registry.npmjs.org`, 2026-08-03.

| Package | latest | beta/canary | Runtime location | Sandbox needs ports? | Backed by |
|---|---|---|---|---|---|
| [`@ai-sdk/harness`](https://www.npmjs.com/package/@ai-sdk/harness) | 1.0.54 | 1.0.0-beta.27 / canary.13 | — (core) | — | — |
| [`@ai-sdk/harness-claude-code`](https://www.npmjs.com/package/@ai-sdk/harness-claude-code) | 1.0.55 | beta.24 / canary.9 | **sandbox bridge** | **yes** | `@anthropic-ai/claude-agent-sdk` → `claude` CLI |
| [`@ai-sdk/harness-codex`](https://www.npmjs.com/package/@ai-sdk/harness-codex) | 1.0.56 | beta.24 / canary.9 | **sandbox bridge** | **yes** | `@openai/codex-sdk` → `codex` CLI |
| [`@ai-sdk/harness-opencode`](https://www.npmjs.com/package/@ai-sdk/harness-opencode) | 1.0.55 | beta.1 | **sandbox bridge** | **yes** | OpenCode |
| [`@ai-sdk/harness-deepagents`](https://www.npmjs.com/package/@ai-sdk/harness-deepagents) | 1.0.53 | — | **sandbox bridge** (`node bridge.mjs`) | **yes** | LangChain `deepagents` (LangGraph) |
| [`@ai-sdk/harness-pi`](https://www.npmjs.com/package/@ai-sdk/harness-pi) | 1.0.54 | beta.25 / canary.9 | **host process** | **no** | `@earendil-works/pi-coding-agent` |
| `@ai-sdk/sandbox-vercel` | 1.0.54 | — | — | provides ports | `@vercel/sandbox@^2` |
| `@ai-sdk/sandbox-just-bash` | 1.0.54 | — | — | **no ports** | `just-bash@^2.14` (in-process JS bash + virtual FS) |

Announced, not yet on npm: `@ai-sdk/harness-amp`, `-goose`, `-mastra`
([harness adapters doc](https://ai-sdk.dev/docs/ai-sdk-harnesses/harness-adapters)).

**Stability: experimental.** Every package README carries "_This package is
**experimental**._"; Vercel's own changelog says "Expect breaking changes
between releases as this early API gets further refined"
([changelog](https://vercel.com/changelog/program-agent-harnesses-with-ai-sdk)).
`harness-deepagents` is weaker still — its README says turn continuation,
suspend/detach, cross-process resume and built-in tool approvals all throw
`HarnessCapabilityUnsupportedError`.

License: Apache-2.0, repo `vercel/ai`, directory `packages/harness*`.

### 1.2 The real `Harness` interface

From `@ai-sdk/harness@1.0.54`, `dist/index.d.ts`. This is the adapter-facing
spec (what you implement if you write your own), verbatim shape:

```ts
type HarnessV1<TBuiltinTools extends ToolSet = ToolSet> = {
  readonly specificationVersion: 'harness-v1';
  readonly harnessId: string;
  readonly builtinTools: TBuiltinTools;
  readonly supportsBuiltinToolApprovals?: boolean;
  readonly supportsBuiltinToolFiltering?: boolean;
  readonly lifecycleStateSchema?: FlexibleSchema<unknown>;
  readonly getBootstrap?: (options?: { abortSignal?: AbortSignal })
    => PromiseLike<HarnessV1Bootstrap>;
  doStart(options: HarnessV1StartOptions): PromiseLike<HarnessV1Session>;
};

type HarnessV1Session = {
  readonly sessionId: string;
  readonly isResume: boolean;
  readonly modelId?: string;
  doPromptTurn(options: HarnessV1PromptTurnOptions): PromiseLike<HarnessV1PromptControl>;
  doCompact(customInstructions?: string): PromiseLike<void>;
  doContinueTurn(options: HarnessV1ContinueTurnOptions): PromiseLike<HarnessV1PromptControl>;
  doSuspendTurn(): PromiseLike<HarnessV1ContinueTurnState>;
  doDetach(): PromiseLike<HarnessV1ResumeSessionState>;
  doStop(): PromiseLike<HarnessV1ResumeSessionState>;
  doDestroy(): PromiseLike<void>;
};
```

`doStart` receives, among others, `sessionId`, `skills`, `resumeFrom`,
`continueFrom`, `permissionMode`, `builtinToolFiltering`, `observability`,
and — always present, framework-created — `sandboxSession:
HarnessV1NetworkSandboxSession` plus `sessionWorkDir: string`.

`doPromptTurn` receives `prompt: string | UserModelMessage`, `tools:
ReadonlyArray<HarnessV1ToolSpec>`, `instructions?: string`, `abortSignal`, and
an `emit(event: HarnessV1StreamPart)` callback. Note the comment on `prompt`:
"The harness session owns its own conversation history, so prior turns are
never replayed across the contract." That is a structural mismatch with our
current design, where blueclaw owns the transcript.

The consumer-facing settings object:

```ts
type HarnessAgentSettings = {
  readonly harness: THarness;
  readonly id?: string;
  readonly tools?: TUserTools;
  readonly skills?: ReadonlyArray<HarnessAgentSkill>;
  readonly instructions?: string;
  readonly stopWhen?: Arrayable<StopCondition<…>>;
  readonly permissionMode?: 'allow-reads' | 'allow-edits' | 'allow-all';
  readonly toolApproval?: HarnessAgentToolApprovalConfiguration;
  readonly sandbox: HarnessV1SandboxProvider;   // ← required
  readonly sandboxConfig?: HarnessAgentSandboxConfig;
  readonly telemetry?: TelemetryOptions;
  // …harnessOptions, activeTools/inactiveTools, diagnostics
};
```

Usage lifecycle (from the `harness-claude-code` README, verbatim):

```ts
const session = await agent.createSession();
try {
  const result = await agent.generate({ session, prompt: '…' });
} finally {
  await session.destroy();
}
```

### 1.3 Where do tools execute — the decisive question

Two disjoint categories.

| | Custom tools (`HarnessAgentSettings.tools`) | Built-in tools (`harness.builtinTools`) |
|---|---|---|
| Who executes | **Our host process.** `.d.ts`: "The adapter does not execute the tool; when the runtime calls it, the adapter emits a `tool-call` event and waits for `submitToolResult` from the caller." And: "the agent executes `tool.execute()` on the host and submits the result back to the harness." | **Inside the sandbox** (or, for Pi, against the sandbox's remote FS/shell). |
| What crosses the wire | name, description, JSON Schema only — `harnessV1BridgeToolWireSchema` comment: "`execute` stays on the host" | nothing; the runtime calls them natively |
| Vocabulary | ours | `HARNESS_V1_BUILTIN_TOOLS` = `read`, `write`, `edit`, `bash`, `grep`, `glob`, `webSearch` |
| Can we turn it off | n/a | `builtinToolFiltering: {mode:'allow'\|'deny', toolNames}` — plus `activeTools`/`inactiveTools` on the agent. Adapters lacking native filtering route inactive built-ins through the approval path and auto-deny. |
| Approval | `toolApproval` on `HarnessAgentSettings`: `not-applicable` / `approved` / `user-approval` (pauses the turn) / `denied` | `permissionMode`: `allow-reads` / `allow-edits` / `allow-all`; adapter must set `supportsBuiltinToolApprovals` |

**Verdict against our non-negotiable:**

| Adapter | Usable with tools behind blueclaw's POSIX boundary? |
|---|---|
| `pi` | **Yes, today.** Host-process runtime; sandbox needs no ports so `sandbox-just-bash` works; our tools execute in our process. Its file/shell access still goes through the sandbox session, so pointing that at a blueclaw-backed provider closes the loop. |
| `claude-code`, `codex`, `opencode`, `deepagents` | **Only with work.** Custom tools are fine, but (a) built-ins execute in the sandbox and (b) the bridge needs a port-exposing sandbox, and `@ai-sdk/sandbox-vercel` is the only supported one. Denying all built-ins removes (a) but leaves the harness with no native file/shell tools, which is most of why you would pick Claude Code in the first place. |
| Any of them, after we write a blueclaw sandbox provider | **Yes.** `HarnessV1SandboxProvider` is a two-method interface (`createSession`, optional `resumeSession`) returning a session with a `restricted()` view (filesystem/exec/spawn) plus `getPortUrl`/`ports`/`setNetworkPolicy`. A provider whose `restricted()` shells out through blueclaw's POSIX helper-exec as the task actor puts *every* tool, built-in included, behind our boundary. Loopback ports are trivially available on a real Linux host. |

The design doc's sentence "An adapter that executes tools in its own process is
not an acceptable integration" is therefore not a filter that rejects these
adapters — it is a requirement that we own the sandbox provider.

### 1.4 Skills and instructions

```ts
skills: [
  {
    name: 'careful-refactors',
    description: 'Make small, low-risk code changes.',
    content: 'Prefer minimal diffs. Preserve public APIs...',
    files: [{ path: 'references/checklist.md', content: '# Refactor checklist…' }],
  },
]
```

Skill files use skill-relative POSIX paths (`reference.md`,
`references/codes.md`, `templates/config.json`) — a direct match for our
`SKILL.md` + `references/` + `assets/` bundle layout, so task 4 in the design
doc ("`SKILL.md` bundles → harness skills") maps cleanly.

Two caveats:
- Skills can be passed on `HarnessAgentSettings.skills` **or** on the adapter
  factory (`createClaudeCode({ skills })`); adapters decide how to surface them.
- Codex has no skills directory, so `harness-codex` "injects every skill inline
  into the user prompt on each turn. Use fewer, larger skills rather than many
  tiny ones." Our per-skill budget rules (15 KB / 300 lines) get *more*
  important, not less, under that adapter.

`instructions` is a separate scalar string and is applied **once**, prepended to
the first user message of a fresh session, and deliberately **not** re-applied
on resume. Our current design injects fresh context every turn (requester
identity, timestamps, recorded effects); that pattern does not survive as
`instructions` and would have to ride in the per-turn `prompt`.

### 1.5 Stop conditions and the loop model

- `stopWhen?: Arrayable<StopCondition<…>>` on `HarnessAgentSettings`.
  Docstring: "Conditions that stop the current result after a completed harness
  tool step that can continue into another model step. The underlying turn
  remains unfinished and can be suspended and continued. A terminal text-only
  step finishes naturally and is not stopped early. When omitted, the harness
  runs until the turn naturally finishes or pauses."
- So `stopWhen` here is a **slicing** primitive, not a hard budget: hitting it
  suspends (`doSuspendTurn` → `HarnessV1ContinueTurnState`) rather than kills.
  `doContinueTurn` resumes — losslessly for bridge adapters that can re-attach
  and replay, lossily for host-resident Pi whose in-flight tail is recomputed.
- Compaction is the **runtime's**, not the harness's: `doCompact()` is only a
  trigger, and adapters that cannot honour it (Codex over `codex exec`, which
  auto-compacts on its own) throw `HarnessCapabilityUnsupportedError`.
- On the plain AI SDK loop (`ToolLoopAgent`/`generateText`), the stop model is
  `stopWhen: isStepCount(n)` with a **default of 20 steps**
  ([loop control](https://ai-sdk.dev/docs/agents/loop-control)).

### 1.6 The event vocabulary — worth stealing even if we skip the adapters

`HarnessV1StreamPart` is a normalized, cross-harness event union. It is the
cleanest existing specification of what an agent CLI has to render, and it is a
good target shape for our own `agentcontract` turn events:

```
stream-start (+ modelId)
text-start / text-delta / text-end
reasoning-start / reasoning-delta / reasoning-end
tool-call (+ nativeName) | tool-approval-request | tool-result
finish-step (finishReason, usage) | finish (finishReason, totalUsage)
file-change  { event: 'create'|'modify'|'delete', path }
compaction   { trigger: 'manual'|'auto', summary, tokensBefore?, tokensAfter? }
error | raw
```

Note `file-change` and `compaction` are first-class stream parts — the CLI
renders a live diff feed and a visible compaction notice without inventing a
side channel. Our current design has neither.

Sources: [harnesses overview](https://ai-sdk.dev/docs/ai-sdk-harnesses/overview),
[tools](https://ai-sdk.dev/docs/ai-sdk-harnesses/tools),
[skills](https://ai-sdk.dev/docs/ai-sdk-harnesses/skills),
[adapters](https://ai-sdk.dev/docs/ai-sdk-harnesses/harness-adapters),
[claude-code](https://ai-sdk.dev/providers/ai-sdk-harnesses/claude-code),
[pi](https://ai-sdk.dev/providers/ai-sdk-harnesses/pi),
[opencode](https://ai-sdk.dev/providers/ai-sdk-harnesses/opencode),
plus the published `.d.ts` and READMEs of the packages above.

---

## 2. Runtime model swapping with the AI SDK

**Short answer: on the plain loop it is a per-step field, so mid-task tier
escalation is a supported first-class feature. On `HarnessAgent` it is not.**

| Path | Where the model is bound | Mid-session swap |
|---|---|---|
| `generateText({ model, … })` / `streamText({ model, … })` | per call | Trivial — pass a different `LanguageModel` next call. |
| `ToolLoopAgent({ model, … })` | at construction | Overridable per step via `prepareStep`. |
| `prepareStep` | per step | **Yes — this is the mechanism.** |
| `HarnessAgent` + adapter | at adapter construction (`createClaudeCode({model})`, `createCodex()`, `createPi({model})`, `createOpenCode({model, provider})`) | **No API.** `HarnessV1Session.modelId` is read-only telemetry; there is no `model` field on `HarnessAgentSettings`, `doPromptTurn`, or `doContinueTurn`. A swap means building a new adapter instance and a new session. Whether a session's `resumeFrom` state can be handed to a *different-model* adapter instance is **UNVERIFIED**. |

The concrete shape, from `ai@7.0.48` `dist/index.d.ts`:

```ts
prepareStep?: PrepareStepFunction<TOOLS, RUNTIME_CONTEXT>;

type PrepareStepFunction = (options: {
  steps: Array<StepResult<…>>;
  stepNumber: number;
  model: LanguageModel;            // the model in use for this step
  instructions: Instructions | undefined;
  messages: Array<ModelMessage>;
  runtimeContext: RUNTIME_CONTEXT;
  …
}) => PromiseLike<PrepareStepResult> | PrepareStepResult;

type PrepareStepResult = {
  model?: LanguageModel;           // "Optionally override which LanguageModel instance is used for this step."
  toolChoice?: ToolChoice<TOOLS>;
  activeTools?: ActiveTools<TOOLS>;
  toolOrder?: ToolOrder<TOOLS>;
  instructions?: Instructions;     // override carries forward to later steps
  messages?: Array<ModelMessage>;  // override carries forward to later steps
  toolsContext?: …;
  runtimeContext?: RUNTIME_CONTEXT;
  providerOptions?: ProviderOptions;
} & LanguageModelCallOptions | undefined;
```

Documented example ([loop control](https://ai-sdk.dev/docs/agents/loop-control)):

```js
const agent = new ToolLoopAgent({
  model: 'openai/gpt-4o-mini',
  tools: { /* … */ },
  prepareStep: async ({ stepNumber, messages }) => {
    if (stepNumber > 2 && messages.length > 10) {
      return { model: "xai/grok-4.5" };
    }
    return {};
  },
});
```

**Implications for us**

1. Our tier-escalation ladder maps onto `prepareStep` almost exactly: it sees
   `stepNumber`, all prior `steps` (with usage), and `runtimeContext`, and can
   return a stronger `model` *and* a narrowed `activeTools` set for that step.
   It can also rewrite `messages` — which is where a compaction pass would sit.
2. This mechanism lives on the **plain loop only**. Choosing `HarnessAgent` as
   bluecollar's substrate means giving up runtime tier escalation inside a turn.
   Given the stated requirement "the model swappable at runtime, mid-session",
   `HarnessAgent` fails it and `ToolLoopAgent`/`streamText` + `prepareStep`
   satisfies it. That is an argument for treating the AI SDK harness adapters
   as an *optional alternative harness* (design doc §3), never as bluecollar's
   own engine.
3. Version note: in `ai@6` (what llmd pins) the field is
   `experimental_prepareStep` and the helper is `stepCountIs`; in `ai@7` they
   are `prepareStep` and `isStepCount`. `@ai-sdk/harness` requires `ai@7.0.48`.
4. `LanguageModel` in a `prepareStep` return is an instance, not a string, so
   swapping providers (llama.cpp ↔ OpenRouter) mid-loop is the same call — this
   is llmd's existing `ProviderRoute.languageModel` value, already constructed
   per route in `llmd/src/provider.ts`.

---

## 3. Reference harnesses — how their loops and CLIs are actually built

### 3.0 Repo identity corrections

| Name as we used it | Reality |
|---|---|
| `@earendil-works/pi-coding-agent` | Repo is **github.com/earendil-works/pi** (author Mario Zechner). Monorepo packages: `agent`, `ai`, `client`, `coding-agent`, `evals`, `protocol`, `server`, `storage`, `tui` — the same layering we are doing. |
| `github.com/sst/opencode` | **301-redirects to `github.com/anomalyco/opencode`** (same repo id — a rename, not a fork). |
| "the Go opencode" | A *different, archived* project (`opencode-ai/opencode`), continued as **`charmbracelet/crush`** (Go, ~27k stars, active). If you want a Go TUI reference for an agent CLI, read crush, not opencode. |
| "antigravity cli" | Real, and **written in Go**. Binary is `agy`. **Closed source** — `google-antigravity/antigravity-cli` is an issue tracker + release host only (verified: `language: null`, no license, tree contains only `.github`, `CHANGELOG.md`, `README.md`, a demo gif, and `examples`; release v1.1.9 ships prebuilt `agy_cli_{linux,mac,windows}_{x64,arm64}` archives). It replaces Gemini CLI, which stays open (TypeScript, Apache-2.0). |

### 3.1 At a glance

| | **pi** | **codex** | **opencode** | **antigravity (`agy`)** |
|---|---|---|---|---|
| Repo | earendil-works/pi | openai/codex | anomalyco/opencode | google-antigravity/antigravity-cli |
| Source | Open | Open | Open | **Closed** (binaries only) |
| Language | TypeScript | **Rust** (`codex-rs/`) | TypeScript / Bun | **Go** |
| License | MIT | Apache-2.0 | MIT | none published |
| Stars | 82.3k | 103.3k | 192.3k | 1.8k (tracker only) |
| Latest | v0.83.0 (2026-07-29) | rust-v0.146.0 (2026-07-29) | v1.18.11 (2026-08-01) | v1.1.9 (2026-07-31) |
| Cadence | ~weekly | multiple alphas/day | ~daily | ~weekly |

Open Google reference where `agy` is unreadable: **gemini-cli** (TypeScript,
Apache-2.0, ~106k stars, still maintained). The Antigravity Python SDK also
ships `google/antigravity/proto/localharness.proto` (672 lines), which is the
only readable primary source for the Antigravity harness contract.

### 3.2 Loop shape

| | pi | codex | opencode | antigravity |
|---|---|---|---|---|
| Tool calling | native | native (Responses API) | native | native (Gemini function calling) |
| Loop owner | its own | its own | **its own** — zero uses of the AI SDK's `stopWhen`/`stepCountIs` despite being on the AI SDK | its own (Go) |
| Stop condition | turn produced no tool calls | text-only turn, `end_turn`, empty queue | **recorded parts contain no tool calls** — deliberately distrusts `finish_reason` | **an explicit `finish` tool** whose JSON schema the caller supplies |
| Step cap | **none** | **none** (soft token-budget prompt) | `agent.steps` unbounded; the cap injects a *prompt*, not an abort | `max_tool_calls` = 512 |
| Parallel tool calls | **yes, default** | yes | yes | yes |
| Compaction | auto when `tokens > window − reserve` (reserve 16384) + `/compact` | auto (mid-turn and pre-turn) + `/compact`; local and server-side | auto, checked every iteration, **queued as a normal task**; `<leader>c` | `compaction_threshold`, with a visible boundary marker |
| Approvals | **none built in** — an extension `beforeToolCall` hook | `UnlessTrusted` / `OnRequest` / `Granular` / `Never` + Seatbelt / Landlock / seccomp | `ask` / `allow` / `deny` wildcard rules, last match wins | `ALLOW` / `DENY` / `ASK_USER`, 9-level precedence |
| Sandbox | **explicitly none, by design** | 4 OS sandboxes | **none** | real OS sandbox + a `proceed-in-sandbox` mode |

Key file paths:

- **pi** — `packages/agent/src/agent-loop.ts` (~792 lines: outer follow-up loop, inner tool loop, `executeToolCallsParallel`, `shouldTerminateToolBatch`); `packages/agent/src/types.ts:144` `AgentLoopConfig`; `packages/coding-agent/src/core/compaction/compaction.ts:235` `shouldCompact()`.
- **codex** — `codex-rs/core/src/session/turn.rs:149` `run_turn` (unbounded `loop {}` at :268); `core/src/tools/parallel.rs`; `core/src/tools/orchestrator.rs`; `core/src/safety.rs`; `sandboxing/src/manager.rs`; `core/src/session/context_window.rs`.
- **opencode** — `packages/opencode/src/session/prompt.ts` `runLoop` (~L1081–1400); `session/llm.ts:280`; `permission/index.ts` and `permission/arity.ts`; `tool/shell.ts:258-290`; `session/compaction.ts`.
- **antigravity** — `google/antigravity/proto/localharness.proto`.
- **gemini-cli** — `packages/core/src/core/client.ts:79` `MAX_TURNS=100`; `scheduler/scheduler.ts` `_isParallelizable()`; `policy/types.ts` `ApprovalMode`; `context/chatCompressionService.ts` (0.5 compress threshold / 0.3 preserve).

**The single most important observation:** none of the four delegates its loop
to an SDK's step counter. opencode is on the AI SDK and still hand-rolls
`runLoop`. That is direct evidence for our design doc §4 decision — adopt
*native multi-step tool calling as the wire format*, but keep owning the loop.
It is also an argument against `HarnessAgent` as bluecollar's engine (§2).

### 3.3 CLI / TUI

| | Framework | Notable mechanics |
|---|---|---|
| **pi** | **custom raw-ANSI** library `@earendil-works/pi-tui` (differential rendering, CSI 2026 synchronized output, Kitty/iTerm2 inline images) | `Component { render(width) -> string[] }`; word-level intra-line diff in `components/diff.ts` |
| **codex** | **Ratatui + crossterm**, with a forked `Terminal` | Finalized cells are written into the **real terminal scrollback** as escape sequences (`insert_history.rs`); markdown is committed only at newline boundaries (`markdown_stream.rs`); approvals are a bottom-pane overlay backed by a **request queue** (`approval_overlay.rs`, ~2445 lines); frame snapshot tests via `insta` |
| **opencode** | **Solid.js on OpenTUI** (TypeScript). Its Go/Bubbletea TUI was **deleted** between v1.0.0 and v1.2.0 (101 Go files → 0) | daemon + **HTTP/SSE** client-server; persisted message *parts* are the source of truth and the UI is a projection; approval is a Portal overlay with a 3-stage `permission` / `always` / `reject` and typed correction fed back to the model |
| **antigravity** | **Go + Bubble Tea v2 + glamour** | Google rewrote Ink/React → Go/Bubbletea while keeping the same UX vocabulary; Artifacts viewer supports line-level inline comments |

Two data points cut in opposite directions on the termcn question (§4):
opencode moved **away** from Go/Bubbletea toward a TS TUI; Google moved
**toward** Go/Bubbletea away from Ink/React, and Antigravity is the newest of
the four.

### 3.4 Worth copying into a Go harness

Consensus items (present in two or more of them):

1. **Own the loop.** All four hand-roll it. In Go we have no alternative anyway.
2. **`ALLOW | DENY | ASK_USER` with an explicit precedence lattice**, first match wins, fail-closed when a predicate errors. opencode and both Google products arrived here independently.
3. **Approval as a protocol event, not a UI callback** — a deferred/channel map keyed by request ID plus an event bus. That makes the TUI, the chat connector, and our virtual-session tests equal clients of the same mechanism.
4. **Stop on recorded state, not `finish_reason`.** opencode carries an in-source note that providers return `"stop"` alongside tool calls. That is a production scar worth inheriting for free.
5. **Step cap as an injected prompt, not a hard abort** (opencode `MAX_STEPS_PROMPT`, codex `budget_limit.md`). This matches our existing time-limit finalize/escalate behaviour, and is the reason **not** to copy gemini-cli's hard `MAX_TURNS=100`.
6. **Parallelism gate via a single `RwLock`** (codex): read lock = parallel-safe tool, write lock = must run alone. Two lines instead of a scheduler. Default off, opt-in per descriptor.
7. **Dispatch tool calls as they stream in**, collect into an ordered queue, drain after the stream closes (codex `FuturesOrdered`).
8. **Behaviour lives on the descriptor, never on the name** — codex carries `supports_parallel`, `exposure`, `escalate_on_failure`, `waits_for_cancellation` on the tool descriptor. This directly ratifies our existing no-tool-name-branching rule.
9. **Anchored incremental compaction**: merge into a `<previous-summary>`, protect a token-budgeted tail, prune tool output separately with a protected-tool list, never split a tool-call/result pair, and run compaction as a **queued task inside the normal loop** rather than a special-case interrupt.
10. **Malformed tool call → synthetic `invalid` tool result** the model can react to (opencode), with the repair tool registered but hidden from the model's tool list. llmd already does the equivalent via `experimental_repairToolCall`.

Single-source but high value:

- **tree-sitter-parsed bash before permission matching**, plus an arity table for prefix reduction (opencode `permission/arity.ts`). This is what makes a rule like `"git *": allow` actually hold for `a && b | c`. It is the strongest transferable idea in the whole survey and is a sanctioned alternative to regex-over-prose: it is a grammar parse, not a string filter.
- **`doom_loop` guard**: N identical `(tool, JSON(input))` calls in a row escalate to a permission ask rather than a watchdog kill. Deterministic, narrow blast radius — directly relevant to our livelock history and strictly better than the no-progress hard kill we already replaced once.
- **Approval vocabulary richer than yes/no** (codex): `ApprovedForSession`, `ApprovedExecpolicyAmendment{prefix}`, `NetworkPolicyAmendment{host}`, `TimedOut`, `Abort`. Approvals that *teach the policy* are what stop prompt fatigue.
- **Approval reviewer indirection** — `Hook | Guardian(LLM) | User`, with the deciding source recorded on the event (codex).
- **`external_directory` as a first-class permission** instead of a path-prefix filter in a resolver (opencode). Clean fit for our "no path-string access filters" rule.
- **Headless soft-deny** (antigravity): in non-interactive mode a tool needing approval is denied with a stderr notice **naming the exact allow-rule that would have permitted it**; the run continues and exits 0. Better than hanging and better than auto-approving.
- **Tool exposure tiers + a `tool_search` tool** (`Direct` / `Deferred` / `Hidden`) — a principled answer to tool-list bloat that is not a hand-kept allowlist. Directly relevant to our 88-tool catalog and the `request_tools` work.
- **Persisted parts as the source of truth, UI as an SSE projection** (opencode) — restart-safe by construction, and framework-independent even though their TUI is not.

### 3.5 Do not copy

- **codex** — Responses-API plumbing (`end_turn`, `encrypted_function_args`, remote compaction); the four OS sandboxes (~100 files for Windows alone) when our boundary is already POSIX; ChatGPT auth / quota / cloud-tasks / voice / plugin marketplace; the `execpolicy` Starlark DSL; the two coexisting multi-agent stacks (`multi_agents` and `multi_agents_v2`, with a "remove before stabilizing" TODO — actively churning).
- **opencode** — Effect-TS throughout (no Go analogue worth emulating; do not build an Effect emulator); `packages/llm`'s ten hand-written provider wire protocols; the per-provider prompt zoo (12 model-specific prompt files, maintenance we have no eval budget for); the permissive default posture (`bash: allow`), which is wrong for a server-side agent acting on other people's behalf; `always` grants held only in memory.
- **pi** — it deliberately ships **no sandbox and no permission system**. Its security doc argues that a partial in-process sandbox is worse than none, because it reads as a boundary while still depending on the host shell, filesystem and credentials. Worth reading as an argument; not a model for a multi-tenant service.
- **antigravity / gemini** — the Artifacts + inline-comment review GUI (take the *idea* — review at milestones rather than per tool call — not the viewer); the Kitty-graphics / LaTeX / altscreen renderer; the `~/.gemini/` config namespace and Google auth; the hard `MAX_TURNS=100`; and the Ink/React TUI, which Google itself abandoned for Go/Bubbletea.

### 3.6 Caveats

- codex `main` moves multiple alpha tags per day; the line numbers above will drift quickly.
- The commit that deleted opencode's Go TUI is bisected only to "after v1.0.0, at or before v1.2.0". The `sst` → `anomalyco` rename is verified; its reason is **UNVERIFIED**.
- Antigravity's prompts and loop text are inside a closed binary. Whether `agy` and `localharness` are the same engine build is claimed in the README but not provable from the artifacts. Its default `compaction_threshold` is **UNVERIFIED**.
- The exact set of codex tools whose descriptor returns `supports_parallel_tool_calls() == true` came from code search, not from opening each handler.

---

## 4. termcn

### 4.1 What it actually is

**A shadcn-style copy-in component registry for terminal UIs written in
React.** Not a framework, not a renderer, not its own CLI.

| Question | Answer |
|---|---|
| Category | Component registry. You run the **existing `shadcn` CLI**, it copies `.tsx` files into `components/ui/`, and you own the source. Zero runtime dependency on termcn. |
| Sits on top of | Two renderers, you pick one as the "base": **Ink** (React reconciler → ANSI, Node) or **OpenTUI** (Zig native core + React/Solid reconcilers, Bun). Repo tagline: "Beautiful terminal UI components, built on Ink and OpenTUI." |
| Registry URL | `https://termcn.dev/r/{name}.json`, namespaces `@termcn/ink/*` and `@termcn/opentui/*` |
| Own CLI | None yet — [issue #13 "feat: add termcn CLI package"](https://github.com/shadcn-labs/termcn/issues/13) is open |
| Runtime | Ink base → **Node ≥20.9**. OpenTUI base → **Bun, explicitly** (repo skill notes: "Use Bun, Not Node.js — OpenTUI is built for Bun") |
| **Usable from Go** | **No.** It is React JSX. There is no Go binding and no Go port. |

Only nuance: **OpenTUI's core** (not termcn) exposes a C ABI and once had Go
bindings ([pkg.go.dev `github.com/sst/opentui/packages/go`](https://pkg.go.dev/github.com/sst/opentui/packages/go),
MIT, dated 2026-01-07). The current upstream tree (`anomalyco/opentui`) contains
no Go code — the binding appears removed. Even if revived it would give a raw
cell buffer, not termcn's components.

### 4.2 Maturity

| | termcn | bubbletea / lipgloss / bubbles |
|---|---|---|
| Version | **None.** No releases, no git tags, unversioned rolling registry. npm `termcn` is a squatted 45-byte placeholder from an unrelated publisher. | bubbletea **v2.0.8** (2026-07-03), lipgloss **v2.0.5**, bubbles **v2.1.1**. v2 GA 2026-02-24, patch-only since. |
| Age | Created 2026-04-03 (~4 months) | ~6 years |
| Stars | 1,021 | 44.1k / 11.6k / 8.7k |
| License | MIT | MIT |
| Components | **329 registry items**, ~120 UI components per base | **14** primitives you compose yourself |
| API stability | **Experimental.** No semver, open install bugs ([#8](https://github.com/shadcn-labs/termcn/issues/8)), open issues proposing component reshapes (#17, #18). Copy-in mitigates churn: once copied it is your code. | Settled. Module path moved to `charm.land/bubbletea/v2` (Go 1.25+); `View()` now returns `tea.View`; `tea.KeyMsg` split into `KeyPressMsg`/`KeyReleaseMsg`. |
| Registry / copy-in model | Yes, that is the whole product | **No.** Charm has nothing analogous; bubbles is a normal `go get charm.land/bubbles/v2` import. Closest is a one-shot GitHub template. |

### 4.3 API shape

```bash
npx shadcn@latest add @termcn/ink/spinner
npx shadcn@latest add @termcn/opentui/spinner
```

```tsx
import { Spinner } from "@/components/ui/spinner";
import { Text } from "ink";

export function App() {
  return (
    <>
      <Text>Loading...</Text>
      <Spinner />
    </>
  );
}
```

Keybindings live inside each copied component via a copied `useInput` hook —
from real source, [`registry/bases/ink/ui/confirm.tsx`](https://github.com/shadcn-labs/termcn/blob/main/apps/web/registry/bases/ink/ui/confirm.tsx):

```tsx
export const Confirm = ({ message, onConfirm, onCancel, ... }: ConfirmProps) => {
  const theme = useTheme();
  const [selected, setSelected] = useState<boolean>(defaultValue);
  useInput((input, key) => {
    if (key.leftArrow || key.rightArrow) { /* … */ }
  });
};
```

Adding one component also installs `components/ui/types.ts`,
`lib/terminal-themes/default.ts`, `components/ui/theme-provider.tsx`, and
`hooks/use-input.ts`.

### 4.4 The relevant components

There is a real agent/AI cluster, which is why the design doc picked it:
`chat-thread`, `streaming-text`, `thinking-block`, **`tool-call`**,
**`tool-approval`**, `token-usage`, `model-selector`, **`diff-view`**,
`file-change`, `git-status`, `usage-monitor`, `command-palette`,
`embedded-terminal`, `directory-tree`, `file-picker`, `log`, `wizard`. Those are
close to a one-to-one list of what a bluecollar CLI has to render.

Streaming behaviour depends on the base, not on termcn:
- **Ink base — inline, scrollback preserved.** Permanent output via Ink's
  `<Static>`, live progress below it. This is the Claude Code model.
- **OpenTUI base — full-screen alternate buffer.**
- `StreamingText` takes `stream?: AsyncIterable<string>` — real incremental
  streaming at the component level.
- `ScrollView` is an in-app viewport with a drawn scrollbar, not terminal
  scrollback.

The Go stack has the same trade-off, differently spelled: bubbletea v2 renders
inline by default and alt-screen is a per-frame field (`v.AltScreen = true`);
`tea.Println` streams above the UI but is a no-op while alt-screen is active.

### 4.5 Recommendation — pick bubbletea, not termcn

**termcn is TypeScript-only. It cannot be used from Go, full stop.** blueclaw is
Go and bluecollar is Go, so adopting termcn means the CLI layer becomes a
Node (Ink) or Bun (OpenTUI) process.

| Option | Cost | Verdict |
|---|---|---|
| **TS TUI + Go daemon** — CLI is an Ink app talking to blueclaw over its local REST/socket | A second runtime and a second protocol on every install, purely for the terminal. For a product whose install story is "one binary on a spare Linux box", shipping Node/Bun alongside it to draw a spinner is a regression. Also puts the TUI behind a wire boundary the loop does not need. | **No** |
| **Rewrite the CLI layer in TS** | Splits bluecollar across two languages; the loop stays Go, the UI is TS, and every stream part crosses a boundary. Worst of both. | **No** |
| **Go TUI: bubbletea + lipgloss + bubbles** | 14 primitives instead of 120 components, so the agent-specific widgets (`tool-call`, `tool-approval`, `diff-view`, `streaming-text`, `token-usage`) get built by hand. That is real work — but it is work in the same language and binary as the loop, on a semver-stable MIT stack. | **Yes** |

**Do this:** build both CLIs on `charm.land/bubbletea/v2` + `lipgloss/v2` +
`bubbles/v2`, and use termcn's component *source* as design reference — it is
MIT, it is copy-in by design, and its agent cluster is a good specification of
what to render. Port the patterns; do not import the runtime.

Update `harness-split-design.md` §5 accordingly: "TUI via `shadcn-labs/termcn`"
is not implementable from Go.

Sources: [shadcn-labs/termcn](https://github.com/shadcn-labs/termcn),
[installation](https://github.com/shadcn-labs/termcn/blob/main/apps/web/content/docs/(root)/installation.mdx),
[registry.json](https://github.com/shadcn-labs/termcn/blob/main/apps/web/registry.json),
[vadimdemedes/ink](https://github.com/vadimdemedes/ink),
[anomalyco/opentui](https://github.com/anomalyco/opentui),
[charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea),
[charmbracelet/bubbles](https://github.com/charmbracelet/bubbles).

**UNVERIFIED:** whether termcn's registry components are validated against Ink 7
(its docs app pins Ink 6.8.0 while Ink is at 7.1.1).

---

## 5. Harness-agnostic wiring

blueclaw must not name bluecollar. Four mechanisms, ordered by machinery.

| Option | Selection mechanism | Where tools execute | Language fit | Machinery |
|---|---|---|---|---|
| **A. Go interface + registry** (what we have) | `agentcontract.Harness` (9 methods); a `map[string]func(Options) Harness` registry keyed by a config string; bluecollar registers itself in its own `main`, blueclaw only imports `agentcontract`. | Host, by construction | Native Go both sides | Lowest. Already 90% built — `agentcontract/harness.go` exists at HEAD. |
| **B. ACP over stdio** | Config names an agent binary; blueclaw spawns it and speaks JSON-RPC 2.0 as the **client**. | **Host.** `fs/read_text_file`, `fs/write_text_file`, and `terminal/create` / `terminal/output` / `terminal/wait_for_exit` / `terminal/kill` / `terminal/release` are Agent→Client calls the client implements; the client advertises `terminal` as a capability and the agent "MUST NOT" call terminal methods if it is absent. | Go SDK exists (`github.com/coder/acp-go-sdk`), plus official Rust and TypeScript (`@zed-industries/agent-client-protocol`). | Medium. One subprocess + JSON-RPC. |
| **C. MCP server** | blueclaw exposes its tool registry as an MCP server; any MCP-capable agent connects. | **Host** (tools run in the MCP server = blueclaw). | Go MCP libs exist. | Medium. But MCP carries tools only, not the loop/turn/approval lifecycle — you still need something to drive turns. |
| **D. AI SDK `HarnessAgent` in llmd** | Config names an adapter; llmd constructs it. | Custom tools host-side; **built-ins in the sandbox** unless we write our own sandbox provider (§1.3). | TS sidecar, already exists. | Highest: `ai` v6→v7 upgrade, sandbox provider, `stopWhen`/suspend semantics, no mid-session model swap. |

**Recommendation.**

1. **Keep A as the port.** It is already the thing that exists, it is the only
   option with zero serialization cost on the hot path, and Go's implicit
   interfaces mean blueclaw genuinely never names bluecollar — it only imports
   `agentcontract`. Selection at runtime is a registry keyed by a config
   string; blueclaw's binary links whichever implementations its build includes.
2. **Add B (ACP client) as the second implementation, not D.** It is a smaller
   step than the AI SDK harness path, it is the only external-agent protocol
   that already puts filesystem and terminal on the *client* side (i.e. inside
   our POSIX boundary), it has a Go SDK, and Codex and Gemini CLI already speak
   it as agents — so it buys "run this company's agent on Codex's loop" without
   the Vercel Sandbox dependency. Correct §3 of the design doc: ACP is not only
   the "external participant" path; as *client* it is a legitimate harness
   adapter, and as *agent* it is the external-participant path. The distinction
   the doc wants to preserve is **which side of ACP blueclaw sits on**, not ACP
   vs. non-ACP.
3. **Treat D as an experiment, gated on our own `HarnessV1SandboxProvider`.**
   Do not put it on the SaaS Phase-2 critical path while the packages are
   experimental and the only supported port-exposing sandbox is a Vercel
   product.
4. **Do not use C alone.** MCP is the right shape for exposing our tools to a
   third-party agent, but it is not a harness port — it has no turn, approval,
   or session lifecycle.

Sources: [ACP introduction](https://agentclientprotocol.com/overview/introduction),
[ACP file system](https://agentclientprotocol.com/protocol/file-system),
[ACP terminals](https://agentclientprotocol.com/protocol/terminals),
[coder/acp-go-sdk](https://github.com/coder/acp-go-sdk),
[zed.dev/acp](https://zed.dev/acp).
