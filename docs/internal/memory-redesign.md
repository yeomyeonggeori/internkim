# Agent memory: a clean-slate design

Status: Implemented; the store lives in [bluememo](https://github.com/yeomyeonggeori/bluememo) · Last updated: 2026-09-03

This replaces blueclaw's memory subsystem (`internal/memory`, the Graphiti
sidecar, the Kuzu graph, the per-person `MEMORY.md`) with one store, one write
path, and one read path. It borrows the memory model that supermemory documents
and keeps our scope model, which is finer than theirs. Nothing here moves memory
off the host; [`saas-design.md`](./saas-design.md) §7.1 decided that agent memory
is host-local, and this design lives inside the host's own Postgres.

Layering follows [`harness-split-design.md`](./harness-split-design.md), one
step further: the store is its own module, `bluememo`, vendored into blueclaw
at `.dependency/bluememo` the way the harness is. It depends on nothing but
`pgx`; blueclaw adapts its task, model and identity types at the edge in
`internal/memory`. The harness sees memory only as a rendered context string
and a `MemoryFact` list through `agentcontract`, exactly as today.

---

## 1. What is wrong today, in one table

| Symptom | Cause |
|---|---|
| The model sees 4% of a person's `MEMORY.md` | The whole file is one `MemoryFact`, clamped to 240 runes at render |
| Circle and workspace memories vanish on restart | Writes go through a 128-slot in-process channel |
| Nothing is remembered unless the model calls a tool | No extraction from finished work |
| Semantic ranking is undone | Sidecar results are re-sorted by substring score in Go |
| Two of five scopes are reachable | Search asks for user and active circle only |
| Contradictions coexist forever | Dedup is exact string match; there is no supersede relation |
| One user's ingest blocks every user's search | Global lock in the Python sidecar |
| Three stores disagree silently | Kuzu, markdown, and Postgres metadata, with no reconciliation |

Every row above is a design fault, not a tuning problem, which is why this is a
rewrite.

---

## 2. Principles

1. **One store.** Postgres with `vector` and `pg_trgm`, in the host database the
   agent already runs. No sidecar, no second process, no files as a database.
2. **Episodes are immutable, facts are derived.** An episode is something that
   happened (a finished task, an explicit "remember this"). A fact is an atomic,
   entity-centric sentence extracted from an episode. Facts point at their
   episode, so every memory is traceable to evidence.
3. **The model judges, the runtime knows.** The model decides what a fact says,
   what kind it is, which older fact it replaces, and when a temporary fact
   expires. The runtime supplies identity, scope, security label, time, and the
   candidate list of facts that could be replaced, and it rejects any reference
   outside that list.
4. **Scope is access.** A fact carries the same scope and security label
   vocabulary that `internal/access` already resolves. The reader's clearance is
   a `WHERE` clause; there is no second gate in Go.
5. **Durable or loud.** A write is a committed row or a tool failure the model
   has to explain. Nothing returns "accepted" for work that may not happen.
6. **Profile first, search second.** What should colour every answer (name,
   role, preferences, what someone is working on this week) is rendered from a
   per-person profile on every task. Search is for the specific.
7. **A budget, in characters, enforced at render.** Memory has a fixed share of
   the system prompt and the ledger records what it spent.

---

## 3. Data model

```sql
create extension if not exists vector;
create extension if not exists pg_trgm;

create table memory_episode (
  episode_id          text primary key,
  source_kind         text not null check (source_kind in ('task_run', 'explicit', 'import')),
  source_id           text not null,
  requester_person_id text not null references person (person_id),
  conversation_id     text not null default '',
  content             text not null,
  occurred_at         timestamptz not null,
  created_at          timestamptz not null default now(),
  unique (source_kind, source_id)
);

create table memory_fact (
  fact_id             text primary key,
  episode_id          text not null references memory_episode (episode_id),
  owner_person_id     text not null check (owner_person_id <> ''),
  subject_person_id   text not null default '',
  kind                text not null check (kind in ('identity', 'preference', 'fact', 'episode', 'temporary')),
  content             text not null check (char_length(content) <= 240),
  embedding_model     text not null default '',
  security_level_rank smallint not null default 0,
  required_classes    text[] not null default '{}',
  valid_from          timestamptz not null,
  valid_until         timestamptz,
  superseded_by       text references memory_fact (fact_id),
  reinforcement_count integer not null default 1,
  last_recalled_at    timestamptz,
  forgotten_at        timestamptz,
  forget_reason       text,
  created_at          timestamptz not null default now()
);

create table memory_fact_circle (
  fact_id   text not null references memory_fact (fact_id) on delete cascade,
  circle_id text not null,
  primary key (fact_id, circle_id)
);

create index memory_fact_content_idx   on memory_fact using gin (content gin_trgm_ops);
create index memory_fact_owner_idx     on memory_fact (owner_person_id)
  where superseded_by is null and forgotten_at is null;

-- only where the vector extension is installed
create table memory_fact_embedding (
  fact_id   text primary key references memory_fact (fact_id) on delete cascade,
  embedding vector(1024) not null
);
create index memory_fact_embedding_hnsw_idx on memory_fact_embedding
  using hnsw (embedding vector_cosine_ops);

create table memory_profile (
  person_id             text primary key references person (person_id),
  identity_lines        text[] not null default '{}',
  current_lines         text[] not null default '{}',
  built_from_fact_count integer not null,
  built_at              timestamptz not null
);

create table memory_job (
  job_id       text primary key,
  kind         text not null check (kind in ('extract', 'profile', 'reembed', 'import')),
  subject_id   text not null,
  attempts     integer not null default 0,
  run_after    timestamptz not null default now(),
  locked_until timestamptz,
  last_error   text,
  created_at   timestamptz not null default now(),
  finished_at  timestamptz
);

create unique index memory_job_pending_idx on memory_job (kind, subject_id)
  where finished_at is null;
```

Vectors live in a side table that the migration creates only when
`pg_available_extensions` lists `vector`. Two databases blueclaw runs against
have no pgvector: the Firecracker guest is Debian bookworm, whose archive
carries no pgvector package (trixie is the first release that does), and the
standalone `cmd/blueclaw` boots the embedded-postgres binaries, which ship
without it. Both still apply the migration and run lexical search; the vector
side arrives with the extension and a `reembed` job. `memory_search` reports
which mode answered. Giving the guest image pgvector is an internkim change to
`tools/prepare-blueclaw-runtime`, either the PGDG apt repository or a rootfs
move to trixie, and it is scheduled with the rollout, not before it.

### Scopes

Every fact has an owner, the person whose task or request produced it, and
zero or more circles in `memory_fact_circle`:

| circles | Readable by |
|---|---|
| none | the owner, and nobody else |
| one or more | the owner, plus members of any named circle (or of a circle that contains one) whose rank and classes pass the label |

There is no company-wide scope. Sharing with everyone is sharing with the
`member` circle everyone belongs to.

Circles nest. `memberCircles` on a circle in `policy.json` lists the circles
that belong to it; the projection turns that into a containment map, and a
reader's readable set is their own circles plus everything those contain,
transitively, with cycles tolerated. A fact shared with `platform` is readable
from `engineering` when `engineering` contains `platform`; the reverse is not
true. Writing to a circle needs direct membership, so the model can name
several circles for one fact but never one the requester is not in; a fact
whose named circles all fail that test narrows to `private`.

The `conversation` scope is gone. A conversation's own memory is its task ledger
and the recent-turn context the loop already injects; extracting from it into a
fourth scope duplicated that with weaker access rules.

The security label (`security_level_rank`, `required_classes`) is inherited from
the episode's origin conversation through `identity.ResolveConversationPolicy`,
never chosen by the model. The reader filter is the existing triple gate, now in
SQL:

```sql
where (owner_person_id = $person_id
    or (exists (
          select 1 from memory_fact_circle c
          where c.fact_id = f.fact_id and c.circle_id = any($readable_circle_ids))
        and security_level_rank <= $reader_rank
        and required_classes <@ $granted_classes))
  and superseded_by is null
  and forgotten_at is null
  and (valid_until is null or valid_until > now())
```

### Kinds

| Kind | Example | Lifecycle |
|---|---|---|
| `identity` | "이샘플 goes by 샘, works in the platform team" | Lives in the profile; replaced only by supersede |
| `preference` | "박예시 wants summaries as bullet points" | `reinforcement_count` grows when re-extracted; ranks higher with repetition |
| `fact` | "The Q3 review is owned by 최견본" | Valid until superseded |
| `episode` | "Deployed admind 2026-08-30 after the config postmortem" | Ranking decays with age; never deleted by decay |
| `temporary` | "Out of office until 2026-09-05" | `valid_until` set by the model; invisible after it |

Superseding is a pointer, not a delete. `superseded_by` keeps the history and
the live index excludes it. Forgetting is a timestamp and a reason.

---

## 4. Write path

Everything that writes memory goes through one pipeline, ingest. Its input is
an episode: the transcript of a finished task, or the one sentence a person
asked the assistant to remember. Its output is the set of facts the memory
should hold afterwards, each related to what was already there.

1. Embed the episode and fetch up to 24 live facts the requester can read that
   are nearest to it. These are the **candidates**, each with its ID.
2. Call the low-tier model with the schema in §4.3, the episode, the
   candidates, the requester's name, the active circle, and today's date.
3. Validate: `relatedFactID` must be a candidate ID when the relation is
   `supersedes` or `reinforces` and empty when it is `new`; `circleIDs`
   keeps the named circles the requester is a member of, and a fact left
   with none is the owner's alone;
   `validUntil` is required for `temporary` and forbidden otherwise; a
   private fact carries no security label because its scope is already one
   person. A violation fails the ingest with a ledger event, never a silent
   repair.
4. Embed the new facts, then insert the episode and its facts in one
   transaction, mark superseded candidates, increment `reinforcement_count` on
   reinforced ones, and enqueue a `profile` job for every subject touched.

The model decides what a fact says, what kind it is, and whether it replaces
or repeats an existing one. The runtime decides which facts exist, who may
read them, and what the label is. Neither side does the other's job, and no
similarity threshold ever merges two facts on its own.

Memory holds little. The conversation history and the task ledger already
keep what was said and what was done, so the instruction tells the model that
an empty list is the normal answer and that a fact survives only when it is
about a person or their work, would change how the assistant acts next time
without being repeated, and is stated by the source. What the assistant did
in a task is never a fact; a file rename leaves nothing behind, and the live
check asserts that.

### 4.1 Extraction from finished work

`TaskRunService.RegisterTaskRunTransitionObserver` fires on every transition.
The observer enqueues `memory_job(kind='extract', subject_id=task_run_id)` when
the status is completed, failed, or cancelled. Enqueue is one insert; the
observer never touches a model. At launch the runtime had already written a
`memory.extraction_context` event with the requester's name, the active
circle, the conversation's security label, and the platform, so the worker
reads the ledger and never guesses.

A single worker goroutine polls `memory_job` with `for update skip locked`,
renders the transcript (prompt, each step's instruction and output, the final
reply, the outcome) and ingests it. Failures retry with exponential backoff up
to five attempts, then stay in the table with `last_error` and emit
`memory.extraction_failed` on the task run's ledger. A restart loses nothing
because the job is a row. `memory.extractionDisabled` turns the observer off.

### 4.2 Explicit memory

`memory_remember` takes one sentence and nothing else. It ingests it
synchronously as an explicit episode, so the tool result says which facts were
created, which were superseded, and which were reinforced. The agent never
looks a fact up before writing it and never chooses a kind or a scope: the
merge model does, with the same candidates and the same rules as extraction.
If the merge model or the embedding gateway is down, the tool fails and the
model tells the user.

### 4.3 Ingest schema

Provider-portable by the runtime policy: string enums only, `required` kept,
`""` in place of null.

```json
{
  "type": "object",
  "additionalProperties": false,
  "required": ["facts"],
  "properties": {
    "facts": {
      "type": "array",
      "maxItems": 12,
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["content", "kind", "circleIDs", "subjectPersonHint", "relation", "relatedFactID", "validUntil"],
        "properties": {
          "content":           { "type": "string", "maxLength": 240 },
          "kind":              { "type": "string", "enum": ["identity", "preference", "fact", "episode", "temporary"] },
          "circleIDs":         { "type": "array", "items": { "type": "string" } },
          "subjectPersonHint": { "type": "string" },
          "relation":          { "type": "string", "enum": ["new", "supersedes", "reinforces"] },
          "relatedFactID":     { "type": "string" },
          "validUntil":        { "type": "string" }
        }
      }
    }
  }
}
```

`subjectPersonHint` follows the `personHint` resolver pattern: the exact name
the model saw, resolved to a person ID by exact unique match on display name,
with an empty string for "nobody in particular". The instruction pairs every
must-check with a must-not-invent: extract only what the source states, do not
restate the task itself as a fact, do not repeat an existing fact as new.

## 5. Read path

### 5.1 Profile

`memory_profile` holds two short lists per person, rebuilt by a `profile` job
whenever a fact about them changes (the pending-job unique index debounces a
burst into one rebuild):

- `identity_lines`: from `identity` and reinforced `preference` facts.
- `current_lines`: from `fact`, `episode`, and `temporary` facts in the last
  30 days, most recent first.

The rebuild is a low-tier structured call that condenses the source facts; it
sees only those facts and returns at most 8 and 6 lines respectively. The
requester's profile is loaded on every task launch with one primary-key read.

### 5.2 Search

One query, all scopes the reader can see, hybrid ranked:

```sql
with vector_hits as (
  select fact_id, row_number() over (order by embedding <=> $query_embedding) as rank
  from memory_fact where <reader filter> order by embedding <=> $query_embedding limit 40
), lexical_hits as (
  select fact_id, row_number() over (order by similarity(content, $query) desc) as rank
  from memory_fact where <reader filter> and content % $query
  order by similarity(content, $query) desc limit 40
)
select fact_id,
       coalesce(1.0 / (60 + v.rank), 0) + coalesce(1.0 / (60 + l.rank), 0) as fused
from vector_hits v full outer join lexical_hits l using (fact_id)
```

The fused score is then adjusted deterministically:

| Signal | Adjustment |
|---|---|
| `kind = 'episode'` | × `exp(-age_days / 90)` |
| tie | higher `reinforcement_count` first, then newer `valid_from` |

Reinforcement is a tie-breaker and never a multiplier: adjacent reciprocal
ranks differ by under two percent, so any mild multiplier would put a repeated
preference ahead of a fact that matched the query better. The live test caught
exactly that.

Results update `last_recalled_at`. No substring re-ranking, no per-namespace
fan-out, no cross-encoder in v1.

`pg_trgm` is chosen over a `tsvector` index because Postgres has no Korean
tokenizer and trigram overlap on Hangul syllables is a usable lexical signal.

### 5.3 Rendering and budget

Memory has a fixed share of the stable system message:

| Section | Ceiling |
|---|---|
| Profile | 1,200 characters |
| Recalled facts | 2,400 characters |

Facts are capped at 240 characters at ingest, the clamp the loop's renderer
applies, so rendering never truncates a fact; it stops adding facts when the
ceiling would be crossed. The launch-time
query is the user prompt; the loop's `LLMContextInput.MemoryContext` seam takes
the rendered string. `loop/testdata/prompt-budget.json` is regenerated with the
new section and the reason stated in the pull request.

```
Memory:
About 이샘플: goes by 샘 · platform team · prefers bullet summaries
Currently: migrating admind config to the central plane (since 2026-08-20)
Recalled:
- [fact · workspace · 2026-08-12] The Q3 review is owned by 최견본
- [temporary · until 2026-09-05] 박예시 is out of office
```

The ledger records `memory.recall_injected` with the fact count and the
characters spent in each section, so a prompt that grew can be explained.

---

## 6. Tools

| Tool | Input | Notes |
|---|---|---|
| `memory_search` | `{ query }` | Returns `facts[{ factID, kind, scopeType, content, validAt }]`; the IDs surfaced in this run are remembered for `memory_forget` |
| `memory_remember` | `{ content }` | One sentence; the merge model picks kind, scope, and what it replaces. Returns the fact IDs created, superseded, and reinforced |
| `memory_forget` | `{ factIDs[], reason }` | IDs must have been surfaced by `memory_search` in this run; unknown IDs fail closed with the known set |

The tool surface is shaped for a model: one verb each to search, write, and
forget, no IDs on the write path, and IDs on the forget path only because
the handle rule is what makes forgetting safe: the model can only forget what
it was just shown, inside the scopes it could read.

The descriptor specs in `local_tool_provider.go` carry the policy identity
(`tool:memory_search`, `tool:memory_remember`, `tool:memory_forget`) and the
effect contracts (`memory_fact` created, `memory_fact` forgotten).

---

## 7. What is removed

| Item | Replacement |
|---|---|
| `tools/graphiti_memoryd/`, `tools/graphiti-memoryd` | none |
| `internal/memory` as the store | `.dependency/bluememo`; `internal/memory` keeps the host adapters |
| Kuzu at `/workspace/.blueclaw/graphiti/kuzu`, its backup root entry | `memory_*` tables in the host Postgres |
| `graphiti_namespace`, `graphiti_episode` (migrations 012, 028, 029) | dropped in migration 031 |
| `memory.graphitiEndpoint`, `memory.graphitiKuzuPath` | none |
| `MEMORY.md` per person, `markdown_store.go`, `markdown_compressor.go` | `memory_profile` |
| `update_queue.go` (in-process channel) | `memory_job` |
| `-graphiti-url` on `blueclaw-guest-healthd` | none |
| Go-side `relevanceScore` substring ranking | SQL ranking |

Existing `MEMORY.md` files are imported once: each becomes an episode
(`source_kind='import'`, `source_id` = path) and goes through extraction. The
files are then left unread and removed in the following release.

---

## 8. Embedding

The capability socket at `/v1/embedding/create` stays the only embedding path;
blueclaw never names a provider. Two small additions to the Go client:
`inputType` (`query` versus `document`) and a batch form, both already spoken by
the sidecar it replaces.

A host picks its embedding model once, at setup, from how it executes. A
device that runs the agent as a Firecracker guest embeds with `baai/bge-m3`,
the model its own llama.cpp serves on CPU beside generation, and reaches
the same model on OpenRouter when the local server is down. A host that runs
the agent directly without local models embeds with
`perplexity/pplx-embed-v1-4b` on OpenRouter. The two are not fallbacks for
each other: vectors from two models do not compare, so a store ranks vectors
only from the model it names, and a host that changes its model enqueues a
`reembed` job that moves every live fact across while the rest answer
lexically. Memory embeds little: one query per launch, one per
`memory_search`, and one transcript plus at most twelve facts per finished
task. At OpenRouter's listed price that is well under a cent per thousand
tasks, and a 200 ms round trip sits beside an LLM call that already takes
seconds. The text sent is the same class of company text the agent's own LLM
calls already send to the same gateway, so it opens no new exposure.

Three consequences are handled, not hoped away:

- **Dimensions.** The model emits 2,560 values and pgvector's HNSW index stops
  at 2,000. capabilityd already truncates and renormalises through
  `OutputDimensions`; memory asks for 1,024, which the model's Matryoshka
  training supports, so the column stays `vector(1024)`.
- **Instruction.** Qwen3 embeddings expect an instruction prefix on queries and
  none on documents. The `inputType` field carries that difference; blueclaw
  never composes the prefix itself.
- **One model per company, recorded on every fact.** Vectors from two models
  cannot be compared, so `embedding_model` is stored per row and a change is a
  `reembed` job. capabilityd's existing rule that local and remote embedding
  models must match stays; a local-only company runs `baai/bge-m3` on the
  device at the same 1,024 dimensions and never mixes. The current capabilityd
  default names `baai/bge-m3` as the remote model too, and OpenRouter's
  embedding catalog does not serve it, so that default changes with this work.

When the gateway is unreachable, `memory_search` returns `searchStatus:
degraded` with the profile still rendered, and extraction jobs wait in
`memory_job` until it is back.

The model is a configuration value and a `reembed` job away from being
replaced. A first measurement on 2026-09-02, thirty Korean facts and twenty
hand-marked Korean questions at 1,024 dimensions
(`tests/integration/memory_embedding_benchmark_test.go`, on demand):

| Model | R@1 | R@3 | MRR | 50 calls |
|---|---|---|---|---|
| `qwen/qwen3-embedding-8b` | 1.000 | 1.000 | 1.000 | 117 s |
| `qwen/qwen3-embedding-4b` | 1.000 | 1.000 | 1.000 | 79 s |
| `perplexity/pplx-embed-v1-4b` | 1.000 | 1.000 | 1.000 | 15 s |
| `perplexity/pplx-embed-v1-0.6b` | 0.950 | 0.950 | 0.960 | 12 s |
| `baai/bge-m3` | 0.950 | 1.000 | 0.975 | 26 s |

The set is too small to rank the top three; bge-m3 misses one question at
rank one and has it at rank two. The device takes bge-m3 because it is what
the device can run itself, in about a gigabyte on CPU, where the Qwen sizes
either share the GPU with generation (4b) or do not fit an 8 GB host (8b).
The direct-execution host takes pplx-embed-v1-4b for its quality and its
speed; it is not convertible to llama.cpp, which does not matter to a host
that runs no local model. Each is served by one or two providers on
OpenRouter; the capability daemon retries a throttled call with backoff and
honours `Retry-After`, and the store's job queue waits out anything longer.
The set grows as real facts accumulate and the table is rerun before any
change of default.

---

## 9. Events

All emitted with a string literal name so `internal/catalog` can scrape them.

| Event | On | Body |
|---|---|---|
| `memory.extraction_context` | task ledger | `requesterName`, `activeCircleID`, `securityLevelRank`, `requiredClasses`, `platform` |
| `memory.extraction_completed` | task ledger | `jobID`, `episodeID`, `factCount`, `supersededIDs`, `reinforcedIDs`, `candidateCount` |
| `memory.extraction_failed` | task ledger | `jobID`, `attempts`, `terminal`, `error` |
| `memory.recall_injected` | task ledger | `profileLineCount`, `recalledCount`, `characters`, `mode`, `degradedReason` |
| `memory.recall_failed` | task ledger | the error |

---

## 10. Tests that define done

- A reader whose rank is below a fact's label never receives it through
  `memory_search` or the launch-time recall, across all three scopes.
- A superseded fact disappears from search and profile while its row survives.
- A `temporary` fact past `valid_until` disappears; one before it is returned.
- The rendered memory section never exceeds its ceilings, on a fixture with 200
  long facts.
- An `extract` job survives a process restart mid-flight and completes.
- Ingest output naming a `relatedFactID` outside the candidate list fails the
  job with `memory.extraction_failed`; no fact is written.
- A finished task run, observed through the real transition observer, ends as
  rows in `memory_episode`, `memory_fact`, and `memory_profile` in Postgres,
  and `Recall` returns them.
- One live run against the real low-tier model and the real embedding model
  (`BLUECLAW_LIVE_LLM_TEST=1`): a Korean transcript yields a preference and a
  temporary fact with its expiry, a correcting transcript supersedes the team
  fact, and recall ranks the corrected fact first. It costs a few cents and
  runs on demand, never in CI.
- `memory_forget` with an ID not surfaced in the run fails closed and writes
  nothing.
- The prompt-budget fixture passes with the memory section present.

---

## 11. Rollout

Three pull requests on `.dependency/blueclaw`, each with its submodule bump:

1. Migration 030, the store, the search query, the job table and worker skeleton.
   No callers change.
2. Ingest, the three tools, profile build, launch-time recall, the render
   budget, the transition observer and the worker. The old store stays wired
   only where no database is configured and for the virtual sessions the
   simulate harness runs.
3. Removal (§7), migration 031, the virtual-session store on the in-memory
   repository, config and docs. Done; `memory.SecurityLabel` replaced the
   five-scope namespace list every launch used to assemble.

Each step is verified through `./internkim dev simulate` for the loop behaviour
and `./internkim dev fleet run` for the Postgres extensions on the guest image.

---

## 12. Deliberately not done

- **Derived facts.** supermemory infers new facts from patterns. That is an
  automatic behaviour on weak signals, exactly what the repository rules forbid
  without a trigger, a blast radius, and an off switch. It can come later as a
  reviewable queue.
- **Graph edges beyond supersede.** `extends` is what a second fact with the
  same subject already is; a typed edge would add a table nobody queries.
- **Deletion by decay.** Age lowers an episode's rank; only a person or an
  administrator removes a memory.
- **A cross-encoder reranker.** Measured first, added if recall@10 says so.
- **Central storage.** Decided in `saas-design.md` §7.1.
- **Circle nesting on the central plane.** `circle_member` on Supabase holds
  people only; `memberCircles` exists in blueclaw's policy document today and
  admind does not yet write it. That is the next change on the internkim side.
