# Natural-language onboarding

A person should be able to reach a working company by talking, whatever they
are talking to: a terminal agent, a chat app, a bot in a messenger. The bot
does the tedious part and the person only clicks what they are told to click.

## The scope this document was given

Everything up to and including host install is done with a harness the person
already has, Claude Code or Codex or Pi, reaching the plane over the public MCP
endpoint. InternKim's own agent takes over once the host exists.

That was decided after the analysis below was written, and it is why this
document stops where it does. Two of the three things that stopped Bluecollar
founding a company dissolve under it: the bootstrap never happens there, so
there is nothing to fix. Section 1 says which two, section 7 says what
therefore does not get built. The third one survives, it is larger than
onboarding, and it is left open rather than answered here.

Its siblings sit on either side.
[`agent-driven-onboarding.md`](./agent-driven-onboarding.md) records what the
authorization chain does today and what was changed so that signing up stops
throwing the authorization away.
[`single-binary-host.md`](./single-binary-host.md) picks up the last step of
the script below, host install, and settles what one artifact should ship. This
document is the middle: which surfaces the chain actually serves, what the
standards offer for the ones it misses, and what the whole conversation looks
like written out.

Evidence is file:line against `main` at `cbe82b258`, plus live reads of
`intern.kim` and `api.intern.kim` on 2026-09-22. Blueclaw paths are relative to
`.dependency/blueclaw`.

## 1. Four surfaces

The premise under examination is that the loopback redirect ties the agent to
the browser, so the flow serves a CLI and fails everywhere else. That premise
holds for one of the four surfaces, and the reason it fails elsewhere is
different from the reason expected.

### A CLI on the person's own machine: works

`POST https://api.intern.kim/v1/mcp` with no bearer answers `401` and
`WWW-Authenticate: Bearer resource_metadata="https://api.intern.kim/.well-known/oauth-protected-resource/v1/mcp"`
(read live). `web/tests/integration/public-api-mcp-authorization.test.ts:183-208`
drives registration, consent and `tools/list` against a real project, and since
#1902 `main` carries the pending authorization through sign-up, so a person
with no account finishes in one window.

### A hosted chat app: works, and loopback was never required

`redirect_uris` is whatever the client sends at dynamic registration. The
plane already anticipates a redirect that is not loopback:
`web/src/lib/consent-return.ts:11-13` classifies any `https:` destination as
leaving this computer, and `web/src/routes/oauth/consent/+page.svelte:74-80`
answers that classification with a destructive alert naming the receiving host.
`docs/api/index.mdx:105-109` documents the same distinction in prose.

So a hosted client registers `https://its-own-host/callback`, carries the
OAuth `state` parameter to tie the browser visit back to the conversation it
came from, and receives the code on its own server. This is the ordinary
web-application shape every Slack app and every hosted connector uses.

What is missing is smaller than a grant type. Nobody has registered a client
with an `https:` redirect against the production Supabase project, so whether
hosted dynamic registration accepts one is untested here; the integration test
registers `http://127.0.0.1:33418/callback` only
(`public-api-mcp-authorization.test.ts:93`). And no document tells a builder
that this is the path, or that `state` is theirs to carry.

### A third-party messenger bot: works, same shape

The bot's backend is a public HTTPS server. It posts the authorization URL into
the conversation, the person opens it on the phone they are already holding,
the redirect lands on the bot. The `state` parameter carries the conversation
identifier. Same verdict and same untested precondition as the hosted chat app.

### Bluecollar, InternKim's own agent in the messenger: out of scope

This is the surface that fails, and the redirect has nothing to do with it.
Three reasons were found. The scope above removes the need to answer the first
two; they are recorded because the next reader will find them again and should
not have to work out a second time what they mean.

**A stranger cannot talk to it at all.** `ConnectorRuntime.authorizeSender`
(`internal/connectors/runtime.go:796-822`) resolves a sender through a cached
platform-account mapping, then `identity.resolve` over chatd, then
`RememberPlatformAccount`, which assigns a person only when the email already
appears in the policy projection
(`internal/identity/identity_service.go:193-206`). An unresolved sender reaches
`admitInboundTurn` with an empty person and `IsAllowed` false, and
`refuseUnauthorizedSender` (`internal/connectors/inbound_turn.go:95-116`) either
replies that the account matches nobody it knows about or drops the message
silently when it was not addressed to the bot.
`TestAHostThatCannotAnswerLeavesTheAccountUnmatched`
(`internal/connectors/unknown_account_test.go:37-49`) fixes that as intended
behaviour: a directory that cannot answer admits nobody and invents nobody.
Under the scope above this is the gate doing its job. The guest tier it looks
like it is missing would exist so that a person with no company could reach the
agent, and that person is now talking to their own harness.

**It does not exist until after the thing it would be used to set up.**
Blueclaw runs inside the company host, which `tools/install-company-host`
installs from a connection file that `POST /api/company/host-setup` mints for an
active administrator of a company that already exists
(`web/src/lib/server/company-host-setup.ts:14-22`). A person with no company has
no Blueclaw to talk to. This is the same fact as the one above seen from the
other side, and the scope settles both at once: the harness that installs the
host is the thing that exists before the host does.

**It has no per-person credential at the plane.** Blueclaw attaches no
authorization header to anything; it calls `capabilityd` over loopback, a Unix
socket or vsock (`internal/capability/client.go:87-161`), and passes
`requesterPersonID`, `requesterEmail`, `requesterName` and
`requesterPlatformUserID` in the JSON body
(`internal/agentruntime/capability_tools.go:537-552`). `docs/architecture.md:566-573`
states the consequence: capabilityd acts with its own authority and takes the
requester's identifier for attribution rather than authorization. One
company-scoped authority serves every
person, and per-person authorization lives in Blueclaw's own
`internal/access/access.go`, which the same document calls unfinished work to
move behind the socket.

This one the scope does not touch. It is not about who can start a company; it
is about whether the plane can tell two colleagues apart once the company is
running. Open question 2 carries it.

The part of Bluecollar's story that stays in scope already works. Onboarding a
colleague into an existing company needs no browser handoff: `person_invite`
creates a confirmed account and returns a one-time password in the tool result
(`web/src/lib/server/public-api/catalog/people.ts:202`), twelve characters from
a thirty-two character alphabet, about sixty bits, with no modulo bias
(`web/src/lib/server/control-plane.ts:297-302`).

### The verdict

Today's flow serves three of the four surfaces. Two of those three are
undocumented as onboarding paths and rest on one untested precondition. The
fourth is not a surface this document has to serve. The harness that installs
the host is the one that founds the company, and Bluecollar begins where its
own host does.

## 2. What the standards offer

### RFC 8628, the device authorization grant

The client posts to a device authorization endpoint and receives a
`device_code`, a short `user_code`, a `verification_uri`, an
optional `verification_uri_complete`, an `expires_in` and a polling `interval`.
The human opens the verification URI on any device, authenticates, types the
user code and approves. The client polls the token endpoint with the device
code, reading `authorization_pending`, `slow_down`, `access_denied` or
`expired_token` until a token arrives.

What a careless implementation gets wrong:

- **The user code is short by design, so the verification endpoint carries the
  whole brute-force defence.** §5.1 asks for a code a human will type, which
  bounds its entropy; §5.2 therefore requires rate limiting or an equivalent.
  Shipping a six-character code and no attempt counter lets an attacker grind
  codes and take whichever pending authorization they land on.
- **The device code is the secret and the user code is not.** The token
  endpoint authenticates the polling client by the high-entropy device code. An
  implementation that accepts the user code there has turned the typed string
  into a bearer credential.
- **Single use, short expiry, and an honoured `interval`.** A server with no
  `slow_down` answer has no lever against a client that polls hard.
- **The approval is bound to nothing.** §5.4 and the OAuth security best
  current practice both say it: the browser that approves never touches the
  client that asked. An attacker who persuades a victim to type the attacker's
  user code receives the victim's token. Real campaigns have exploited this
  against large providers, and the mitigations that followed are all outside
  the protocol: show the client name, warn, restrict which clients may use the
  grant, shorten expiry.

That last property is the decisive one here. The grant works precisely because
requester and approver are unconnected, which is exactly what makes a code an
agent recites to a person a poor thing to trust.

### CIBA

The client posts to a backchannel authentication endpoint with a hint naming
which human, plus a `binding_message`. The provider pushes to that human's
enrolled authentication device. The client then polls, waits for a ping and
polls, or receives a push, depending on the delivery mode it registered.

CIBA closes the binding hole: naming the user up front means a passer-by cannot
be made to approve, and the binding message is shown on both devices so the
human can tell which request they are answering. The costs are a confidential
registered client, an authentication device already enrolled, and an identifier
the client is entitled to assert, which is itself an enumeration and
notification-spam surface. The provider has to implement it, and Supabase does
not.

### Pairing codes on streaming devices

A code on the television, a page on the phone. Functionally the device grant
without the specification. Its real defence is situational: the person is
standing in front of the screen that produced the code. A bot reciting a code
over chat has no such evidence.

### QR pairing

`verification_uri_complete` in a square. The code moves from fingers to camera
and the binding property is unchanged, except that a physically presented QR
restores the standing-in-front-of-it evidence and one pasted into a chat
removes it again.

### Magic link plus poll

The client mints an opaque high-entropy link bound to a pending request, the
human opens it and authenticates, the client polls. Compared with the device
grant it is strictly better on entropy, because there is no short code to guess
and nothing convenient to phish by voice, and identical on binding unless the
link is additionally tied to an identity the requester named in advance, at
which point it has become CIBA with an email in place of a push.

### Does any of this fit a messenger where the client is the bot?

The short user code exists because the requesting device has no keyboard worth
using and no clipboard. In a chat the person can tap a link. A device grant
there is a code-entry ceremony with no input-constrained device anywhere in
the picture, and it pays for that ceremony with the weakest binding property of
any option listed.

## 3. Whether to build it here, and as what

### Search first: who already owns this shape

Six candidates, found by searching the schema, the catalog, the relay, the
companion path and Blueclaw.

| Existing thing | Where | What it owns |
|---|---|---|
| The Supabase OAuth authorization | `/oauth/consent`, `authorization_id` | binds a pending authorization to a person, delivers to a waiting client by redirect |
| `public.credential` | `supabase/migrations/20260803000001_core_schema.sql:102` | a settled link from a member to an external identity |
| `public.agent` | `supabase/migrations/20260803000009_agent_table.sql` | the company computer's hashed, named, revocable key |
| `CompanionPairingCode` | `internal/admind/companion.go:28-38` | a code binding a pending pairing to an owner, redeemed once for a token |
| `capabilityprotocol.RecoveryAction` | `pkg/capabilityprotocol/protocol.go:292-298` | the agent handing a person something to do out of band, over DM |
| `MagicLinkService` | `.dependency/blueclaw/internal/auth/magic_link_service.go` | issue and consume a 32-byte one-time token bound to a person |

Two of these deserve a closer look, because between them they already are the
mechanism this document was asked to consider building.

**The companion pairing code is a working RFC 8628 user-code half.** A `POST`
to `/pairing-codes` mints an eight-character code formatted `XXXX-XXXX`
(`companion.go:1195-1198`, thirty-two bits) bound to an owner email, person,
platform and platform user, with a ten-minute expiry
(`companion.go:233-248`). `POST /_internkim/companion/pair` redeems it once and
answers a companion identifier and a token whose hash alone is stored
(`companion.go:291-330`). The web app shows the person a command containing the
code (`companionPairCommand`, `companion.go:1186-1188`) and a deep link
(`companion.go:1182-1184`). What it lacks is the thing §5.2 asks for: the
redeem path has no attempt counter, so its thirty-two bits stand alone. It sits
on the customer's own host rather than the plane, which is what keeps that
survivable, and it belongs to the frozen device half.

**Blueclaw's magic-link service is dead scaffolding.** `IssueMagicLink` mints
thirty-two random bytes, stores only a SHA-256 hash against a person and a task
run with a fifteen-minute lifetime, and `ConsumeMagicLink` refuses an expired or
already-consumed token (`internal/auth/magic_link_service.go:29-70`). It has no
production caller anywhere in the submodule: the only call sites are
`internal/task/task_auth_service.go:24-29`, which wraps it, and
`tests/integration/magic_link_login_own_task_list_test.go:18,23`. No route
exchanges a token for a task session, and the page that would terminate a click,
`admin/src/routes/login/[token]/+page.svelte`, renders the token as text and
calls nothing. The store is an in-memory map, so a restart forgets every
outstanding link.

### The four shapes, judged

**Our own `/pair` page implementing the user-code half while Supabase keeps
identity.** Rejected, and the earlier reasoning needs correcting on the way.
`agent-driven-onboarding.md` rejected the device grant because Supabase does not
implement it and because adding it would mean standing up an authorization
server beside Supabase's. The second half of that is wrong: nothing here would
issue identity tokens. It would mint a code, bind it to whoever approves it in
a plane page they are signed into, and hand over a credential of a kind the
plane already mints. That is a pairing endpoint, and `companion.go` shows it is
perhaps two hundred lines. The rejection survives the correction on other
grounds. It buys RFC 8628's unbound approval, the one property that makes a
code an agent reads out dangerous, and it buys it to solve an input constraint
none of the four surfaces has.

**Redirect back to a plane page showing a code the person reads to the bot.**
Rejected, and worse than the first. Whatever the person recites is exchangeable
for a token, so a live credential lands in the messenger's message history, in
the model's context window, and in the task event ledger, which holds tool
inputs in full. Revocation loses its handle, because the bot now holds
something indistinguishable from any other text the person typed. And it
reintroduces the copy-paste step the redirect exists to remove.

**A plain signed link the agent sends, with polling and no code.** This is the
only one worth keeping, and keeping it dissolves it. The standard version of
the same idea is what the hosted chat app and the messenger bot already do: the
link is Supabase's own `/oauth/authorize` URL, the correlation is `state`, and
the credential arrives at the bot's server through its registered redirect
without passing through the conversation. That is strictly better than a
bespoke link and a bespoke poll, because it adds no table, no endpoint and no
second secret. The shape collapses into surface B.

**Do nothing new, and tell messenger users to run a CLI once.** Rejected for
third-party bots, which need no CLI. For founding a company it stopped being a
fallback anyone has to accept: the scope makes the terminal harness the
founding surface by decision, and a new customer has no Bluecollar to talk to
in any case.

### The recommendation

Build no pairing mechanism. Three things instead, smallest first.

1. Document the hosted-client path beside "Starting from nothing" in
   `docs/api/index.mdx`: register an `https:` redirect, carry `state`, expect
   the consent screen to warn the person that access leaves their computer.
2. Settle the untested precondition by registering one client with an `https:`
   redirect against production dynamic registration. It is a write, and it is
   reversible from the connected-apps screen.
3. Give host install the shape the companion already has, a single command
   carrying its own credential, in place of a file downloaded in a browser and
   carried to another machine. Section 6 sets out the two ways to do that.

### The attack this has to survive

Making the off-computer redirect the recommended path makes the consent
screen's warning load-bearing.

An attacker registers a client under a plausible name, since dynamic
registration is open by design (`supabase/config.toml:181-184`). They get a
victim to open the authorization URL, in a chat or an email. The victim
approves. The attacker's server receives the code, and the token it exchanges
for carries `fullPublicAPIPermission`
(`web/src/lib/server/member-request.ts:41-42`), which is everything that
person can do.

What stops it: the victim is signed in as themselves and reads the client name
and the receiving host inside a destructive alert before approving
(`consent/+page.svelte:71-79`); PKCE binds the code to the client that
registered it, so intercepting the code alone yields nothing; the app appears
under connected apps and revoking it refuses its next call, which
`public-api-mcp-authorization.test.ts` asserts. What does not stop it: nothing
checks the client, and `docs/api/index.mdx:107` says so to the person's face.

That residual is the one every provider with open dynamic registration carries,
and it is smaller than what the rejected alternatives carry. The victim here
performed a navigation they could inspect. Under a device code they would have
typed a string into a page they trusted, with nothing on screen tying that
string to the thing that asked for it.

## 4. The conversation, written out

Each step carries one of three marks. Today means it works now.
Undocumented means the route or tool exists and nothing tells an agent.
Missing means what it says.

### A new customer, from nothing

The surface is a terminal agent on the founder's own laptop. The scope fixes it
there, and section 1 finds independently that it is the only one where a person
with nothing can reach an agent that can reach the plane. Step 12 is the
handover: from there the company has a host, and its own agent exists to be
talked to. A hosted chat app substitutes for the harness wherever a browser
window is opened, with the redirect landing on its own server instead of a
loopback port.

**1. The person says what they want.** "A friend said I should try InternKim
for my company." The agent explains what InternKim is and what setting it up
needs: one computer that stays powered on and awake, Docker with Linux
containers, an OpenRouter key. It asks for nothing yet.

Agent calls nothing. **Missing**, as prompt knowledge. The prerequisites are
written in `docs/quickstart.mdx:15-27` for a human reading a docs site, and
nothing puts them where an agent with no company will find them.

**2. The agent connects.** "I will connect to InternKim. A browser window
opens and you sign in there."

```bash
claude mcp add --transport http internkim https://api.intern.kim/v1/mcp
claude mcp login internkim
```

**Today**, documented at `docs/api/index.mdx:91-92`.

**3. The person signs up inside the window it opened.** The browser lands on
`intern.kim/oauth/consent?authorization_id=…`. They have no account, so
`WebAuthGate` shows the sign-in card with two links. They click the one that
starts a company and type their address.

**Today**, since #1902. Before it the authorization died here.

**4. The person proves the address.** They open their mail, read an eight-digit
code, type it, choose a password, and register a passkey or skip it.

**Today**, and genuinely human. The emailed code is the only proof of address
the public path has, standing where no captcha does
(`supabase/config.toml:47-48`). It carries a live risk: `[auth.rate_limit]
email_sent = 2` with Supabase's own mailer
(`supabase/config.toml:43,82`), unresolved in `agent-driven-onboarding.md`.

**5. The person names the company.** The form at `/start` opens carrying the
return path. They type a name and an address, and land back at consent.

**Today.** The agent cannot help, and should not want to: the person is looking
at a form asking the same question the agent would ask.

**6. The person approves.** The consent card names the client and says the
answer returns to `127.0.0.1`. They allow it. The loopback fires and the agent
holds a token.

**Today.**

**7. The agent looks around.** `tools/list` answers with tools instead of
`401`, which is how it learns it may continue. It reads
`company_settings_get`.

**Today.**

**8. The agent asks about the computer.** "Your company has no computer
connected yet. Is there a machine that can stay powered on? Are you sitting at
it, or do you reach it over SSH?"

`GET /api/company/host-setup` answers `hasConfiguration` and `lastSeenAt`
(`web/src/lib/server/company-host-setup.ts:24-36`), so the agent can check
rather than guess. **Undocumented**: `docs/api/index.mdx:118` names the `POST`
alone.

**9. The agent mints the connection.** `POST /api/company/host-setup` with
`{"replaceExisting": false}` answers the file, carrying `schemaVersion`,
`appURL`, the company row, the plane's public project URL and publishable key,
the gateway URL and a sixty-four hex `agentKey`
(`web/src/lib/company/host-setup.ts:15`).

**Documented since #1902**, at `docs/api/index.mdx:116-120`. It is in no
catalog.

**10. The person gets the file onto the company computer.** The agent has the
JSON in its own process, on the laptop. The installer wants it on the other
machine.

**Today**, and this is the worst step in the script. Section 6 is about it.

**11. The person installs.** Three lines on the company computer, a hidden
prompt for the OpenRouter key, several minutes of Docker build.

```bash
git clone --recurse-submodules https://github.com/yeomyeonggeori/internkim.git
cd internkim
python3 tools/install-company-host ~/Downloads/internkim-host.json
```

**Today**, hardcoded as a literal at
`web/src/routes/settings/setup/+page.svelte:18`. The hidden prompt is
avoidable: `--model-key-file` reads the key from a private file
(`tools/install-company-host:180`).

**12. The agent waits.** It polls `GET /api/company/host-setup` until
`lastSeenAt` moves, and says so when it does.

**Undocumented**, same route as step 8.

**13. The person adds a colleague.** "Add 박예시, she runs operations." The
agent calls `person_invite`, which requires approval
(`catalog/people.ts:218`), and receives a member identifier, an address and a
temporary password.

**Today**, a catalog tool.

**14. The person passes the password along.** The agent gives them the address
and the one-time password and says it is readable once. The person sends it
however they normally would.

**Today.** The security shape is a sixty-bit one-time secret carried out of
band by a human an active administrator has vouched for
(`web/src/lib/server/public-api/record/people-tools.ts:135`).

**15. The person asks about the messenger.** "Can we use this from Slack?"

**Missing**, with a caveat worth stating before anyone builds it. Buzz, the
messenger the company actually uses, is stood up by the installer in step 11,
so it is already connected. What has no route, no tool and no document is
connecting a messenger the company already uses. Whether that is wanted is a
product question this document cannot settle.

### Somebody invited to a company that already exists

Four turns, and nearly all of it already works.

**1.** A colleague sends them an address and a one-time password, from step 14
above. **Today.**

**2.** They tell their agent to connect them, and it runs the same two
commands. **Today.**

**3.** The browser opens at consent. They sign in with the address and the
temporary password. No emailed code: the invite path skipped that proof because
an administrator stood behind it. They approve. **Today.**

**4.** Every tool works from the first call, because they belong to a company.
**Today.**

This case needs no company creation, no host, no mailbox and no new mechanism.
It is the shape the rest of the script should be measured against.

## 5. What the agent needs that it does not have

| What | Shape it should take | Status |
|---|---|---|
| What InternKim needs before it works: a computer that stays on, Docker, an OpenRouter key | something the agent knows how to say | missing |
| That the host is a prerequisite, and that a company without one is half-built | something the agent knows how to say | missing |
| That `person_invite` answers a password a human must carry, readable once | tool description, already says it | present |
| Founding a company for an authorized account with none | a route the agent is told about, `POST /api/company` | documented |
| Reading whether the computer is connected | a route the agent is told about, `GET /api/company/host-setup` | undocumented |
| Minting the connection file | a route the agent is told about, `POST /api/company/host-setup` | documented |
| Getting the connection onto the company computer in one paste | installer and one served script | missing, section 6 |
| Registering a hosted client with an `https:` redirect and correlating by `state` | documentation for whoever builds surfaces B and C | missing |
| Connecting a messenger the company already uses | undecided | missing, and possibly not wanted |

### Does the case against a `company_create` tool survive this script?

It survives, and the script strengthens it.

The structural objection stands. `callingMember` resolves the member row and
its company before any tool is dispatched, and a caller belonging to no company
is refused there (`web/src/lib/server/member-request.ts:97-108`), so a
`company_create` tool would have to run at a moment the dispatcher refuses by
construction. Every `RecordContext` carries a company identifier. Admitting one
tool past that precondition means the catalog's entry condition stops being an
entry condition, for the sake of a single call.

The script adds a second objection the earlier analysis could not see. At the
moment founding happens, step 5, the person is looking at a browser form
asking for a company name and an address. A tool would have the agent ask the
same two questions in chat while that form sits open, and would make founding
happen in two places. The existing route covers the narrower case the script
does surface, an authorized account that has an account but no company, and it
already accepts an OAuth access token
(`web/src/routes/api/company/+server.ts:29`). A documented route is the right
size for that.

`POST /api/company` also takes an `invited` array of addresses
(`api/company/+server.ts:57-61`), so founding and inviting the first colleagues
are already one call for whoever reaches it that way.

## 6. Host install

This is step 10 and step 11 of the script, and
[`single-binary-host.md`](./single-binary-host.md) takes the question further
than this section does: what the customer should receive, and whether a
database can be supervised rather than run in a container.

`agent-driven-onboarding.md` concluded that a remote tool could only ever return
instructions, because every input the installer needs lives on the target
machine. Tested against the script, the claim about the tool is right and the
claim about the ceiling is not.

| Input | Where it comes from today | Could the plane hand it over |
|---|---|---|
| `internkim-host.json` | downloaded in a browser | it is already the plane's, minted by `POST /api/company/host-setup` |
| OpenRouter key | hidden prompt, or `--model-key-file` | it is the customer's key, and holding it would be a new custody decision |
| Docker with Linux containers | checked locally (`tools/install-company-host:59-66`) | no |
| the repository clone | `git clone --recurse-submodules` | published images would remove the clone and the build |
| postgres password, Buzz seed, relay key, media keys | generated locally by `secrets.token_hex(32)` (`install-company-host:76-86`) | they are the host's own and must stay there |

Of five inputs, one already comes from the plane and three of the four local
ones need no human: they are generated or checked. Exactly one is typed, and
the installer already accepts it as a file.

So the ceiling is not three commands and a file the person downloaded on
another machine. It is one line, the shape this repository already ships for
the companion:

```
curl -fsSL https://intern.kim/companion/install.sh | sh
internkim-companion pair --device-url <url> --code <code>
```

(`internal/capabilities/protocol.go:173`, `internal/admind/companion.go:1186`.)

The host has the same problem and a different answer. Two ways to close it, and
the difference is one short-lived object.

**Without a new object.** The one-liner carries the connection file the admin's
session already minted, base64 encoded, and the installer reads it from the
argument. Nothing new exists server-side. The cost is a sixty-four hex company
credential sitting in shell history on the company computer. That is a real
leak, and the repository has already accepted the same trade once, at lower
value, by putting the companion code on a command line.

**With one short-lived voucher.** The admin's session mints a single-use,
short-expiry token that the box exchanges once for the connection file. The
credential never touches the command line, the window is minutes instead of
however long `~/Downloads` keeps things, and single use means a second install
has to be authorized again. The cost is the one object this document otherwise
argues against.

A voucher is not a pending
authorization: nobody approves it, an already-authenticated administrator mints
it, and a machine redeems it. There is no user code, no polling, no consent
page, and nothing for an agent to read aloud, because the person pastes it from
the screen that produced it. The right comparison is `tailscale up --authkey`,
`kubeadm join --token` and a GitHub runner registration token, all of which are
a pre-authorized short-lived join token pasted into one command.

Recommendation: the voucher, on the grounds that shell history holding a
company's agent key is worse than one table row with an expiry. If that is
judged too much for the gain, the no-object version is still a large
improvement over a file crossing machines by hand.

What an attacker gets either way: whoever holds the voucher or the encoded file
within its lifetime can install a host for that company and hold its agent key.
That is the same blast radius as today's downloaded file, with a shorter window
and a single use. What stops abuse is expiry, single redemption and the admin
gate that mints it. What does not is pasting it into a chat, which is why the
command belongs on a screen the person is already looking at and not in
anything an agent recites.

## 7. What would not be built

**A device authorization grant, or our own `/pair` page.** Section 3 gives the
reasoning and corrects `agent-driven-onboarding.md`'s version of it. The mechanism is
small and the codebase already contains one. The objection is that it buys the
weakest binding property of the options surveyed in order to solve an input
constraint no surface here has.

**CIBA.** It fixes the binding hole the device grant leaves, and it needs a
confidential client, an enrolled authentication device per person and a
provider that implements it. Supabase implements neither it nor the device
grant, and the plane holds Supabase as the schema of record.

**Any code the person reads to the bot.** It puts a live credential into
message history, model context and the task ledger.

**A `company_create` catalog tool.** Section 5.

**A guest tier for Bluecollar, or anything else that lets a stranger talk to
it.** It would exist so that somebody with no company could ask the agent to
make one, and the scope puts that conversation in the harness instead. The
identity gate stays as `internal/connectors/unknown_account_test.go:37-49`
fixes it: a directory that cannot answer admits nobody.

**A `host_install` or `host_connect` catalog tool.** The inputs live on the
target machine. What is remote is already a route.

**An onboarding session object, a status endpoint or an onboarding-status
tool.** Three existing answers already cover the question. `tools/list`
succeeding says the client is authorized. `GET /api/company/host-setup` says
whether the computer is connected. The `403` from `callingMember` says the
account belongs to no company yet, and names where to start one. A fourth surface would be a copy of one of them.

**An agent-facing account-creation API.** The emailed code is the only proof of
address on the public path, and no captcha stands behind it. An API that mints
accounts without it hands anyone an unlimited supply.

**A messenger-connection tool.** Step 15 of the script wants one, and the
installer already stands up the messenger the company uses. Until somebody says
which external messenger and why, building one risks answering a question
nobody asked.

**Building anything on Blueclaw's magic-link service.** It is complete, correct
and reachable from nothing but one integration test. Its store is an in-memory
map that a restart empties, and the page meant to terminate a click prints the
token into the DOM. It is scaffolding for a task-inbox login that was never
finished, and the repository's rule on discovering dead code points at deleting
it rather than growing it. That deletion is a separate change from anything
here, and it should check first whether the removal was deliberate.

One observation that is not a recommendation. The companion pairing code is
thirty-two bits with no attempt counter on redemption
(`internal/admind/companion.go:300-307`), which is the mistake RFC 8628 §5.2
exists to prevent. It survives because the endpoint sits on the customer's own
host, and it belongs to the frozen device half, so this is recorded rather than
proposed.

## Open questions

The three open questions in `agent-driven-onboarding.md` still stand and bear
on this one as well. Three more come out of this analysis.

**1. Does production dynamic registration accept an `https:` redirect?** Every
word of the hosted-chat-app and messenger-bot verdict depends on it, and only a
write settles it. The integration test registers a loopback redirect only.

**2. Is the per-person authorization gap at capabilityd going to close?** This
is the one block from section 1 that the scope leaves standing, and it is not
an onboarding question. The plane does not distinguish person A from person B:
capabilityd acts with its own company-scoped authority and takes the requester
identifier for attribution, and `docs/architecture.md:566-573` already calls
moving that decision behind the socket unfinished. Whatever closes it will be
the first thing in the product that tells two colleagues apart at the plane,
which makes it a question about what the product is. It wants its own design
and its own evidence, at a moment somebody chooses.

**3. What does "connect our messenger" mean?** Step 15 of the script is the
only genuinely empty cell in section 5 that nobody has scoped. It may be
Slack, it may be nothing, and it may already be answered by Buzz.
