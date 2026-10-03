# Expensive Tests

Each JSON file is one ordered scenario. Its steps share one session, stop at the
first failure, and pass only when every step satisfies its strict assertions.

**Nothing runs these.** `internkim test expensive` refuses and says so. They are
the specification a Buzz driver has to satisfy.

Scenarios `05-message-lifecycle`, `06`, `07` and `08` need rewriting before a
driver can run them: `05` names a town-square channel in its prompt, and the
others assert `conversationType: "O"`, a public channel in another messenger's
vocabulary. The scenarios that mutate wait on an approval button, and the Buzz
path has no such surface, so a driver must define what "the user approved"
means before it can assert on it.

The Linux check of the member-to-agent round trip is `tools/test-native-install`.
Attachments, inbound mentions and agent-initiated direct messages have no automated
Linux check.

Retained scenario directories keep screenshots and downloaded files under
`evidence/`. Result, event, timing and Playwright trace data
stays under `diagnostics/`. The machine-readable `manifest.json` indexes
user-visible evidence separately from diagnostic paths.
