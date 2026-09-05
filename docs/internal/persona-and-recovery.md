# Persona delivery and task recovery

admind owns the configured `identity.json` and `soul.json`. Blueclaw owns the
copies its instruction and identity readers consume. The device host's staging
workspace and the guest's ext4 workspace are separate filesystems. Writing a
document in the first cannot update the second.

admind publishes both documents through `PUT /admin/api/persona/agent`. Blueclaw
validates the complete bundle before writing either document, installs each file
atomically, refreshes its backup, removes retired workspace persona files, and
returns the installed documents. admind compares the acknowledgment with its
configured documents. Startup publication retries after a failed delivery because
the runtime can start after admind. An update whose delivery fails reports the
failure while retaining the configured document for a later publication.

`user.json` stays in each requester's private home and uses the existing persona
user API and POSIX actor. Agent publication does not rewrite requester documents.

`BOT_PROFILE.yaml`, `BOT_PROFILE.md`, `IDENTITY.md`, and `SOUL.md` are retired
workspace artifacts. The runtime does not read them. admind retains a one-time
reader for the old configuration's `bot-profile.yaml` when no identity JSON
exists, preserving configured aliases and introduction during migration. Once
JSON exists, that old configuration has no authority.

A previous assistant's task summary can be wrong about an entity, a required
date, a tool call, or whether a write happened. Connector retry context therefore
includes recent tool-result events as typed recorded attempts: observation ID,
tool, input, failure and effects. The window holds twelve attempts; inputs over
2 KiB and older attempts carry explicit omission markers. Assistant-written
claims remain hypotheses. Current intake results supersede an old inferred
outcome contract when available.

Bluecollar's recovery budget limits spending. It does not require spending.
The generated native action schema exposes `fail` as soon as a recorded failure
creates recovery debt. Guidance that permits stopping must name an action the
model can actually call. Runtime recovery instructions remain system messages;
only executed `continue` observations become native tool-call/result pairs.
Otherwise one failed call can appear to the model as two attempted writes.
The model chooses a correction or independent route from the observed failure
and available tools, or reports the blocker immediately. A result-validation
failure can happen after a mutation; it does not establish that nothing was
saved. Recovery must inspect current state when possible before repeating an
uncertain write.

Regression coverage includes persona API installation and runtime loading,
visible publication failures, retired-file removal, bounded recorded attempts,
replacement of stale outcome interpretations, and early failure with unused
recovery budget. Opt-in live tests in Bluecollar's `loop` package exercise
repairable input errors, an unavailable server repair, and reinterpretation of
an old participant mistake using the current identity and tool contract.
The unrecoverable live case requires an explicit `fail` action and bounds model
requests. A failed run caused by a model transport or action-parsing error does
not pass as a successful early-stop decision.
`fail.message` carries the final reply. After the referenced failure facts pass
validation, the runtime delivers that reply through the existing terminal report
path, without generating another recovery decision and another reply. Older
actions without a final message retain the existing failure-report generator.
