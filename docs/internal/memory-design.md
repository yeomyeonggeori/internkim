# Memory architecture

Memory is a selective recall system. `user.json` remains the broad profile and
owns profile updates. Memory contains facts and decisions the assistant chose to
remember, with an audience, source episode, recorded time and validity state.
Task launch does not automatically retrieve or inject remembered facts.

## Runtime contract

The assistant may synchronously call `memory_remember` when a user explicitly
asks it to retain a fact or when the model judges a durable decision important.
`memory_search` is on demand: the model supplies a question or context and the
system returns selected matching facts. Retrieval is bounded; the web surface
reports incomplete results and unavailable stores.
`memory_update` and `memory_delete` address one exact fact and namespace. A
profile remains separate from these fact operations.

Graphiti stores searchable facts, embeddings and episode provenance through
Kuzu and capabilityd. Legacy Markdown storage remains available through its
existing API, but the agent's recall and remembrance tools use graph memory.
The previous-memory view shows graph facts with past or future validity.

## Security and provenance

Every graph query is scoped to the authenticated reader and filters namespaces
through the existing permission boundary. Fact update and delete requests from
the company web surface use a signed assertion with the existing central-plane
agent key. The assertion binds the HTTP method, exact path, reader identity,
expiry and request-body hash; missing keys fail closed and no unsigned fallback
is used.

Each fact exposes its source episode IDs when known. The UI joins those IDs to
recorded episodes and displays the source prompt, platform and time. Unknown
provenance is shown as unavailable. Validity compares timestamps with the
current time, including future start dates and expired or invalidated facts.

## Web inspection surface

The memory route has four tabs: memory list, memory search, relationships and
schedules. The list is a calm master/detail view of current facts, with an
optional previous-memory view. The detail panel shows the full statement,
audience, provenance, validity interval and exact fact edit/delete controls.
Search submits a real graph query and reuses the same fact/detail presentation;
loading, unavailable, incomplete and failed retrieval states remain visible.
The selected detail is available as an accessible mobile view without requiring
the user to scan the entire list first.

This shape follows the useful distinction between source material and extracted
memories described in [Supermemory’s architecture overview](https://supermemory.ai/docs/concepts/how-it-works).

## Verification

The Kuzu tests invoke production mutation methods, reopen the database, check
updated embeddings, reject another namespace and preserve source episodes after
fact deletion. Their embedder is synthetic. Admind and Blueclaw verify the same
fixed assertion fixture. Workbench browser tests cover search, provenance,
mutation, unavailable retrieval and mobile detail.

Use a live model with a stored synthetic fact and a separate empty-memory
control to assess recall. Start a fresh session with no conversation history,
then preserve the model request, retrieved fact IDs and answer. Repeat after
correction and deletion. A scripted Local Fleet scenario verifies Linux
execution and tool effects; the live experiment measures model judgment.
Profile tests inspect the persisted `user.json` after the model selects
`persona_update`.

Keep results with the tested revision in the local artifact directories and
the pull request. The [memory verification review](memory-verification-review.md)
records the pre-change baseline and the full experiment procedure.
