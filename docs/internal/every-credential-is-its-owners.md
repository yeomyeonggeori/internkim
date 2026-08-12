# Every credential is its own owner's

## The rule

The running system holds no account that stands for somebody else. Each
person's work on the messenger is done with a credential that person owns, and
no part of the product keeps an administrator session to do work on their
behalf.

## Where it is broken today

The browser already obeys the rule. `messengerCredential()` reads the signed-in
member's own credential from `/api/member/messenger-credential`, puts it in the
call as `actor`, and chatd's `PersonalGateway` serves every `person.*` call with
it. Files, tasks and memory reach admind carrying the requester's own address.

The rule breaks one step earlier, at the point the credential is *made*. The
relay signs in to Mattermost as one person — a real employee, whose account was
locked on 2026-08-12 when the recorded password stopped matching — and uses that
session for two jobs:

| job | what it does with the admin session |
|---|---|
| `provisionMemberCredentials` | mints a personal access token for every user and posts them to the central plane |
| `refreshContacts` | reads the whole roster and writes the `contact` table |

Both are the last places where one account acts for everyone. Two smaller
dependencies hang off the same connection record: the relay reads the messenger
base URL from it to fetch emoji and profile pictures, and the connection stores
the administrator's username and password.

## The shape

### A member registers their own credential

There is no path today for a person to supply their own credential;
`NoMessengerCredentialError` is raised and nothing catches it. The path is:

```
browser (signed in)
  → its own private Realtime channel  member:<memberID>
    → relay
      → chatd  person.credential.issue
        → the platform adapter turns what the person gave into a durable credential
      → central plane: keep the credential for that member, write their contact row
```

The relay learns which member is asking from the channel the call arrived on.
`listenTo(memberID)` binds the callback per member, so registration is the one
capability that authenticates by channel rather than by `actor`. Every other
call keeps requiring an `actor`, and keeps re-deriving identity from it.

### What a person hands over is the platform's to say

Asking for a password would be one messenger's shape written into every layer
above it. A Nostr-keyed messenger wants a secret, a hosted one wants an OAuth
redirect, and a self-hosted one wants a sign-in. The Chat SDK does not settle
this: `@chat-adapter/shared` carries `AuthenticationError` and token encryption,
and nothing about how a person's credential is obtained in the first place.

So the gateway declares it and the product renders it.

```ts
type CredentialRequirement = {
	kind: 'sign-in' | 'secret' | 'redirect';
	fields: { name: string; label: string; isSecret: boolean }[];
	redirectURL?: string;
};

credentialRequirement(): CredentialRequirement;
issueCredential(answers: Record<string, string>): Promise<IssuedCredential>;
```

The settings screen asks the company's messenger what it needs, draws those
fields, and sends the answers back. It names no platform. An adapter that wants
a sign-in takes a login and a password, exchanges them for something durable and
keeps neither; an adapter that wants a secret takes the secret; an adapter that
wants a redirect sends the person out and finishes when they return.

`issueCredential` returns the durable credential together with the identity it
resolves to, so registration proves the credential works before anything is
stored. Which durable form comes back is the adapter's decision, and callers see
one shape either way.

### The directory stops being swept

`contact` rows exist to answer three questions:

| question | asked by | answered after this change by |
|---|---|---|
| which member holds this external id | `memberOfExternalID`, for addressing an answer | the row written when that member registered |
| which members are these external ids | `tellThoseAddressed`, for notifications | the same rows |
| what is this author called | `nameOf`, for the notification title | the arrival itself |

The first two only ever concern members, and a member's row is written the
moment they register. The third is the only one that needs a name for somebody
who may not be a member. Whatever reports an arrival is already inside the
messenger and already holds the author; today the Mattermost plugin sends
`authorExternalID` and stops there. Sending the display name beside it removes
the last reason for the relay to read a roster.

### The relay stops knowing what a messenger is

`asset.emoji` and `asset.picture` are reads made with the member's own token
against the messenger's base URL. Moving them to chatd as person capabilities
lets `host/relay/mattermost.ts` and `host/relay/member-tokens.ts` be deleted
whole, and leaves the relay speaking only to Supabase, the central plane, admind,
maild and chatd.

### The connection stops carrying an account

What remains of a company's messenger connection is where the messenger is.
`settings.username` and `secret` go away, and with them the question of whose
password the product is holding.

## One passage, two transports

Everything that reaches the messenger goes through one API, wrapped as tools.
The browser reaches it over Supabase Realtime through the relay, because a
person on another network cannot open a socket to the company's machine. The
agent reaches it over HTTP. The transports differ because the network forces
them to; the API and the authorisation on it do not.

The agent has a gate in front of it. The gate holds no operations, so it cannot
drift from the passage: it reads the descriptor, compares it against the grant,
and either forwards or asks a person. It forwards the delegation itself rather
than a person's credential, so the passage resolves the grant and enforces the
scope where the work happens, and no component holds the standing ability to act
as somebody it is not.

Everything the gate needs is already declared on the descriptor.
`RequiresApproval` says whether a person must agree, `ApprovalScope` says how far
that agreement reaches, `SideEffect` says how much it costs to be wrong, and
`RequiresUserPresence` says whether the operation may run with nobody there. The
gate reads those fields and learns nothing about what an individual operation
does. A gate that starts branching on operation names has stopped being a gate.

### A ceiling on one transport must not become a ceiling on the API

Realtime carries a bounded payload, and the relay refuses an answer over
`ANSWER_BYTE_CEILING` with a 413. On 2026-08-12 a real answer hit it.

The cause was a collection with no bound, and the two collections need different
answers because their callers want different things.

A message page is a window on a sequence, so pagination is already the right
mechanism and only its stopping rule is wrong: fifty posts however long they run
becomes a byte budget, with `hasMoreBefore` set from it. That closes the case
completely, because one post can never fill the channel on its own, so a page of
at least one always fits.

The emoji set is not a window. The browser needs every entry to render `:name:`
at all, so paging it means several round trips to assemble something the caller
always wants whole. The size comes from putting bytes inside the list: each
image is inlined as a data URL at up to 100 KB. The split is per item rather
than per page. An index of names is small and arrives once; an image is fetched
when it is first drawn and then held, which costs nothing again because emoji do
not change. The id an image is fetched by stays inside the relay, which already
read it while listing, so nothing above has to know how a platform keys an
emoji.

Both are in place: the byte budget in chatd's `listMessages`, the index and
`asset.emoji.image` in the relay. When the relay's assets move to chatd in steps
1 and 2, the emoji capability carries this shape with it.

> Pagination is right when the caller wants a window on a sequence. When the
> caller wants the whole set and the entries are heavy, split the entry.

What is left is the genuinely large single thing, a file or an attachment, which
no page size can cut down. The rule is uniform and belongs to the API:

| result | delivery |
|---|---|
| under the threshold | in the answer |
| over the threshold | a reference in the answer, bytes fetched separately |

A caller never learns the ceiling and an operation never learns which transport
carries it. Without this, the passage is one API with two usable surfaces, which
is the leak this section closes.

Where those bytes sit is a product decision. Supabase Storage is the ready
answer and the browser already talks to it, at the cost of company bytes resting
in the central plane, which `saas-design.md` §6 says the central plane is not
for. Short-lived objects with a signed URL and a delete on collection keep the
seconds and give up the permanence. Emoji and avatars are company-wide already
and cost little either way; message attachments and file contents are the part
worth deciding deliberately.

## Order of change

Each step is safe on its own, and the seven members who already hold credentials
keep working throughout.

1. **chatd serves a person's emoji and pictures.** Additive; nothing calls it
   yet.
2. **The relay reads assets through chatd.** Deletes the relay's messenger
   client. Ships together with step 1, because it is one contract across two
   components.
3. **chatd issues a credential from a person's own sign-in.**
   `person.credential.issue`, with a Mattermost adapter and a Buzz adapter.
4. **A member connects their own messenger account.** The relay serves the
   registration call by channel, keeps the credential and writes the contact
   row; the settings page gains the screen that was missing.
5. **The arrival carries the author's name.** Plugin and relay together.
6. **The relay stops minting and stops sweeping.** `provisionMemberCredentials`,
   `refreshContacts` and `member-tokens.ts` are removed.
7. **The connection drops the account fields.**

Steps 1–2 and 3–4 are each one deployable contract. Step 6 is what ends the
outage class this document exists for, and it cannot land before step 4 gives
new people a way in.

One more follows, and it is the larger half:

8. **The agent's messenger tools go through the passage.** Today
   `chatd_platform_adapter.go` calls `reply.send` as the bot, outside the actor
   boundary entirely. This is the step that makes the agent act as the person
   who asked, and the gate is what it gains on the way.

## What this does not do

Nothing here changes how an answer is authorised. `serveCall` still refuses a
call with no actor, still asks chatd who the actor is, and still addresses the
answer to the member that credential belongs to. Registration is an addition
beside that gate rather than a hole in it.

Nothing here invents a service account. An earlier draft proposed moving the
administrator credential from the relay into chatd. That relocates the problem.

## Evidence to keep

- A member with no credential can register one and reach their messenger,
  proven end to end in `web/scripts/check-relay.ts` against a messenger nobody
  runs.
- The relay serves every capability with no messenger account configured at all.
- `TestFleetAccountUpsertPayloadCarriesNoOrganizationFields` has a sibling: the
  company connection payload carries no account field once step 7 lands.
