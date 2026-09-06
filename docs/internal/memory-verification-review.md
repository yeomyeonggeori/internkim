# Memory verification review

This document is the pre-change baseline. The current architecture and
verification status are in [memory-design.md](memory-design.md).

Reviewed against main `e4ccb40e5` and Blueclaw `fd6b129f` on 2026-09-05. This is a source review;
it does not establish the deployed agent's recall quality.

## Findings

The current engine is Graphiti with a Markdown memory fallback. Supermemory's
documented distinction between source documents and extracted memories, and its
update/extend/derive relationships, is a useful comparison:
https://supermemory.ai/docs/concepts/how-it-works.

| Priority | Finding | Evidence |
| --- | --- | --- |
| High | The main explicit-memory scenario uses scripted responses in one virtual session. It cannot establish recall after a restart with conversation history absent. | `.dependency/blueclaw/internal/e2e/scenarios.go`, `MemoryExplicitToolAcceptanceScenario`; `internal/e2e/virtual_session_test.go` |
| High | Graph fact listing writes the fact/node identifier into `sourceEpisodeID` and emits a zero `validAt`. The UI tries to join this identifier to episode IDs. Source navigation and recency therefore lack reliable data. | `.dependency/blueclaw/tools/graphiti_memoryd/main.py`, `memory_fact`; `web/src/routes/memory/memory-fact-list-model.ts`, `episodeForFact` |
| High | The list projection carries no invalidation or replacement metadata. The UI cannot distinguish current facts from superseded facts using its response contract. | `main.py`, `list_namespace_facts`; `web/src/routes/memory/memory-graph-api.ts`, `MemoryGraphFact` |
| High | Fact/node listing catches database exceptions and returns empty collections. Namespace collection also skips search failures. An empty GUI can conceal a retrieval failure. | `main.py`, `list_namespace_facts`; `.dependency/blueclaw/internal/adminapi/memory_graph_handler.go`, `collectNamespaceFacts` |
| Medium | The fact list fetches at most 120 items and filters that collection locally. Its search is not a complete memory search. | `web/src/routes/memory/memory-fact-list.svelte`, `loadMemoryGraph`; `memory-graph-api.ts`, `fetchMemoryGraph` |
| Medium | Private memory can be durably written to Markdown while Graphiti enrichment is unavailable or queued. Storage and search readiness are distinct states. | `.dependency/blueclaw/internal/agentruntime/memory_tools.go`, `persistMemoryUpdateTool` |
| High | Markdown merging appends distinct lines until the size limit triggers model compression. A corrected preference can coexist with its old value in pinned context before compression. | `.dependency/blueclaw/internal/memory/markdown_store.go`, `mergeMarkdownMemory` |

Existing unit coverage includes namespace access, requester-scoped fallback,
Markdown persistence and deletion, and explicit memory tool calls. Those checks
are useful but do not measure model judgment or semantic correction.

## Acceptance experiment

Run only in a disposable Local Fleet with dedicated identities. Record the model,
revision, timings, tool results and memory IDs. Expected answers belong exclusively
to the scenario; runtime prompts and tools must contain no answer-specific hints.

1. In session A, supply a synthetic fact whose answer cannot come from general
   knowledge. Verify `memory_remember` and its durability result. For a graph
   test, wait for recorded ingestion completion before retrieval.
2. End session A and restart the agent while retaining the memory store. Use a
   fresh conversation B without previous turns, summaries or message-history
   access. Inspect the actual model request to confirm the fact is absent.
3. Ask a paraphrased question. Record retrieved fact IDs and the final answer.
   Separate graph retrieval from Markdown automatically injected at task launch.
4. Repeat with a matched isolated control that has no stored fact. It should
   acknowledge missing information. This comparison establishes that memory
   contributed to the answer.
5. Correct the fact in another session. Verify both the current answer and an
   explicit historical question. The old value must not appear as current truth.
6. Delete the fact and verify that graph results, pinned context and subsequent
   answers no longer contain it. Separately verify that another person and an
   unauthorized circle cannot retrieve it.
7. Add unrelated memories and ask again. Measure retrieval recall, answer
   correctness, stale-answer rate, unauthorized disclosure and latency separately.

Use live model calls for these outcomes. Scripted sessions remain appropriate for
queue, cancellation, permission and persistence invariants. A successful tool
invocation alone does not pass the answer-quality check.

## Inspection UI

The default view should list what the agent currently believes. Each item needs
its owner/audience, source, recorded time, validity interval and current/replaced
status. Distinguish explicitly supplied facts from model inferences. Put the
relationship graph behind a secondary view.

Provide a retrieval inspector that accepts a question as the signed-in member
and shows the actual retrieved facts and source links. Display indexing failures
and incomplete results explicitly. Keep edit/delete results tied to their
recorded effects, including removal from every recall source.

Implement provenance and current-state correctness first, then the isolated
recall experiment, then the inspection interface. Expanding graph visualization
before those steps would leave the central verification problem unresolved.
