# Data room threats

What the clearance model of [`dataroom-standard.md`](./dataroom-standard.md)
does not cover by itself, and what covers it. Row-level security answers the
one question of who may read a document; every threat below is a way around
that question rather than through it.

| Threat | Cover |
|---|---|
| A document carries instructions, and the agent follows them | After a task has read a document at clearance 2 or 3, its outbound tools (`mail_message_send`, `message_send` to an external channel, `site_serve`) go behind the approval gate for the rest of the task. The task remembers the highest clearance it read. |
| Memory extraction repeats a document to someone below its clearance | Graphiti skips the results of `company_document_*` tools and the files they deliver. A document reaches memory only as its `id` and title. |
| The ledger becomes a second copy | `task_event` records a document's `id`, `sha256` and byte count, never its text. |
| A superseding or published copy points outside its clearance | `supersedes` and `published_from` are composite foreign keys inside the company; the tool refuses a target in another domain; the record only lets a member who can read a row update it, so raising a document's clearance requires having read it. |
| Clearance read from a stale or forged source | `member.clearance` is the only source and the policy reads it on every query. There is nothing to reload. |
| A weakly identified requester reads clearance 3 | A person resolved from a messenger mapping is the same member row as one signed in on the web, so the record cannot tell them apart. Until the host can cap a messenger-resolved session at 2, a company that wants the distinction gives clearance 3 only to people who reach the agent from the web or a companion. This is open. |
| Listing leaks the shape of what one cannot read | `company_document_list` and the exported `INDEX.md` are built from the rows the reader's session returns, so a hidden domain is absent rather than counted. |
| An ambient task acts as nobody or as the wrong person | A scheduled or channel-ambient task runs as the message author; a task with no person behind it holds clearance 0. |
| A parser exploit or a forged derivative | Derivation runs as the requester in their own `tmp/`; the uploaded text is that requester's claim; `sha256` in the row names the original and `check` recomputes it. |
| A signed URL is handed onward | The URL lasts ten minutes and names a path whose clearance digit the bucket policy checked when issuing it. Passing it on is the holder's act, logged against the document `id`. |
| The public mirror publishes more than clearance 0 | The service writes `/workspace/shared/public/dataroom/` from rows at clearance 0 only, and `check` compares the mirror with those rows. |

The first two are host behaviour and belong to blueclaw's task loop and the
memory sidecar; the composite keys and policies are the record's; the mirror
and `check` are the skill's. Each row names one of the three, so a change in
one place is the whole change.
