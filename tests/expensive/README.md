# Expensive Tests

Each JSON file is one ordered scenario. Its steps share one session, stop at the
first failure, and pass only when every step satisfies its strict assertions.

**Nothing runs these today.** They were driven through a real Mattermost, and
that driver went with Mattermost. They are kept as the specification a Buzz
driver has to satisfy, and `internkim test expensive` refuses and says so.

Four of them still speak the old dialect and need rewriting before a Buzz driver
can run them: `05-message-lifecycle` names a town-square channel in its prompt,
and `06`, `07` and `08` assert `conversationType: "O"`, which is Mattermost's
name for a public channel.

Approval is the other gap. The scenarios that mutate wait on an approval button
in a messenger, and the Buzz path has no such surface — blueclaw's
`/admin/api/run/approve` bypasses the messenger entirely — so a driver has to
decide what "the user approved" means before these can assert on it.

The Linux acceptance gate in the meantime is the fleet:

```bash
./internkim dev fleet run --scenario buzz-attachment
./internkim dev fleet run --scenario buzz-direct-message
```

Retained scenario directories keep screenshots, downloaded files and website
screenshots under `evidence/`. Result, event, timing and Playwright trace data
stays under `diagnostics/`. The machine-readable `manifest.json` indexes
user-visible evidence separately from diagnostic paths.
