# Agent-driven onboarding

An agent should be able to drive the beginning of InternKim — sign-up, company
registration, invitations, host install — and hand off to a browser window
where a human must genuinely act, the way `claude /login` does. This records
what exists, where the chain breaks, and the smallest change that closes it.

Evidence is file:line against `origin/main` `8a0e9f5dc`, plus live reads of
`intern.kim` and `api.intern.kim` on 2026-09-21. The line numbers under "What
exists today" and "Where the chain breaks" describe that revision, before the
change this document then goes on to record.

## What exists today

### The browser handoff is already built

`https://api.intern.kim/v1/mcp` answers an unauthenticated POST with `401` and
`WWW-Authenticate: Bearer resource_metadata="https://api.intern.kim/.well-known/oauth-protected-resource/v1/mcp"`
(verified live; emitted by `web/src/lib/server/public-api/protected-resource.ts:32-38`,
attached at `web/src/routes/api/v1/mcp/+server.ts:13`). That metadata document
names the plane's own Supabase Auth as the authorization server
(`protected-resource.ts:19-31`), and live it resolves to
`https://mutvimjbvmoludotyehk.supabase.co/auth/v1`.

Reading that authorization server's `/.well-known/oauth-authorization-server`
live gives the whole contract the plane can serve:

| | value |
|---|---|
| `grant_types_supported` | `authorization_code`, `refresh_token` |
| `response_types_supported` | `code` |
| `code_challenge_methods_supported` | `S256`, `plain` |
| `registration_endpoint` | `/auth/v1/oauth/clients/register` |
| `token_endpoint_auth_methods_supported` | `client_secret_basic`, `client_secret_post`, `none` |

Dynamic registration is enabled deliberately: `supabase/config.toml:181-184`
sets `[auth.oauth_server] enabled = true`, `allow_dynamic_registration = true`,
`authorization_url_path = "/oauth/consent"`.

`web/tests/integration/public-api-mcp-authorization.test.ts:183-208` drives the
whole loop against a real project: a client registers itself with
`token_endpoint_auth_method: 'none'` and a loopback redirect
(`http://127.0.0.1:33418/callback`, line 93), is sent to `/oauth/consent`, is
approved, and its access token then lists tools. The same test proves the token
is revocable from the app's connected-apps screen and that the next call fails.

So `claude mcp login internkim` (`docs/api/index.mdx:91-92`) is a real
handoff, of exactly the shape Claude Code's own `/login` uses: dynamic
registration, PKCE, loopback redirect, browser window, CLI waits.

### What the human sees at the consent screen

`web/src/routes/oauth/consent/+page.svelte:45` wraps the page in `WebAuthGate`
(`web/src/lib/components/web-auth-gate.svelte`), passing
`returnPath = pathname + search`, which carries the `authorization_id`.

An unsigned visitor whose plane serves companies
(`web-auth-gate.svelte:38`) is offered a passkey button, an email/password
form, and two links (`web-auth-gate.svelte:138-141`):

- `/auth/claim` — claim an account that was already invited
- `/auth/claim?new-company=1` — found a new company

Neither link carries `returnPath`. The `signupURL()` helper that does build a
return parameter (`web-auth-gate.svelte:54-56`) is only used by the other
branch, the device path.

### The Supabase auth surface that is live

`supabase/config.toml` is the schema of record for the production project
`mutvimjbvmoludotyehk`:

- `[auth] enable_signup = true` (line 29), `enable_anonymous_sign_ins = false` (line 30)
- `[auth.email] enable_signup = true`, `enable_confirmations = true` (lines 76-77), `otp_length = 8`
- `[auth.email.smtp] enabled = false` (line 82) — Supabase's built-in mailer, and `[auth.rate_limit] email_sent = 2` (line 43)
- `[auth.passkey] enabled = true` (lines 113-114), `rp_id = "intern.kim"`, `rp_origins = ["https://intern.kim"]` (lines 116-118)
- `[auth.captcha] enabled = false` (lines 47-48)

`POST /auth/v1/passkeys/options` answers `401` rather than `404` live, so the
passkey endpoints exist on the hosted project.

### The two email-free account paths

`web/src/lib/server/control-plane.ts:262-291` `issueTemporaryPassword` creates a
`email_confirm: true` Supabase account and returns the password in the HTTP
response. Two callers reach it: `inviteMember` (`control-plane.ts:246`), used by
`POST /api/member/invite` and by the `person_invite` catalog tool, and
`resetMemberPassword` (`control-plane.ts:259`).

`AUTH_CLAIM_WITHOUT_EMAIL` (`web/src/lib/server/claim-without-mail.ts:10-12`)
turns the first-claim flow into an on-screen password when a deployment cannot
send mail. It is read only by `POST /api/auth/claim` and by
`requestCompanySignupEmail`, which refuses with `503` when it is set
(`web/src/lib/server/company-signup.ts:35`).

Whether it is set on production could not be determined from the repository:
the Cloudflare Pages environment is not tracked here, and `POST /api/auth/claim`
returns `{sent:true}` for an unknown address before reaching the check
(`web/src/routes/api/auth/claim/+server.ts:30`), so no read-only probe
distinguishes it. Indirect evidence says it is off: `docs/quickstart.mdx:33-36`
tells a founder to enter a code from the email, and the magic-link template is
configured (`supabase/config.toml:87-93`).

The invite path is unambiguously live: `person_invite` is in the shipped
catalog and its description promises the temporary password in the answer
(`web/src/lib/server/public-api/catalog/people.ts:198-218`).

### What an agent can and cannot do today

`callingMember` (`web/src/lib/server/member-request.ts:92-111`) accepts either a
personal access token or a bare Supabase access token, and an OAuth access token
takes the second branch (`member-request.ts:41-42`), arriving with
`fullPublicAPIPermission` and an empty `tokenName`.

| Step | Catalog tool | HTTP route | Repo script | Web page |
|---|---|---|---|---|
| Create an account | not found | `POST /api/auth/claim`, `POST /api/auth/signup` (both send an emailed OTP) | not found | `/auth/claim` |
| Found a company | not found | `POST /api/company` (`web/src/routes/api/company/+server.ts:24-75`) | not found | `/start` |
| Invite a person | `person_invite` (`catalog/people.ts:198`) | `POST /api/member/invite` | not found | `/organization` invite dialog |
| Issue a token | not found | `POST /v1/token` (`web/src/routes/api/v1/token/+server.ts:15-37`) | not found | `/settings` |
| Install / connect the host | not found | `POST /api/company/host-setup` issues the connection file (`web/src/routes/api/company/host-setup/+server.ts:14-23`); the install itself has no route | `tools/install-company-host` | `/settings/setup` |
| Connect a messenger | not found | not found | part of `tools/install-company-host` (Buzz images, relay key, `chatd` identity: lines 116-157) | not found |

A sweep of `web/src/lib/server/public-api/catalog/` for *host*, *device*,
*install*, *setup*, *provision*, *onboard*, *signup*, *company creation*,
*token issuance* and *messenger* found no tool. The only near-miss is
`mail_connection_start` / `mail_connection_status` (`catalog/mail.ts:148-173`),
scoped to the requester's own mailbox. The catalog holds 88 tools across 14
files, assembled in `catalog/tools.ts:1991-2011` and served from the generated
snapshot `pkg/capabilityprotocol/generated/capability-tools.json`
(`web/src/lib/server/public-api/catalog.ts:1`).

Two rows deserve emphasis. `POST /api/company` authenticates through
`memberAccessTokenOf` (`api/company/+server.ts:29`), which does not require the
caller to belong to a company, so an OAuth-authorized agent can already found
one. `POST /api/company/host-setup` refuses a personal access token outright
(`web/src/lib/server/company-host-setup.ts:13`) and requires an active admin
member (lines 14-20), so an OAuth token passes where an issued key does not.
Both routes live under `/api/`, which `movesToTheOneAddress` exempts from the
308 to the one address (`web/src/lib/server/company-host-redirect.ts:19-20`), so
both answer on `api.intern.kim`. Verified live: `POST /api/company/host-setup`
with no bearer returns `401`.

Neither is documented as an agent-reachable step, and neither appears in the
catalog, so nothing tells an agent they exist.

### The host install path

From nothing to a running host (`docs/quickstart.mdx:31-80`):

1. Found the company at `https://intern.kim/auth/claim?new-company=1`.
2. Download `internkim-host.json` from `/<company>/settings/setup`. The file
   carries `schemaVersion`, `appURL`, the company row, the plane's public
   `projectURL` and `publishableKey`, `gatewayURL`, and a 64-hex `agentKey`
   (`web/src/lib/company/host-setup.ts:15`). The key is minted by `issueAgentKey`
   (`web/src/lib/server/control-plane.ts:364-380`), which stores only
   `api_key_hash` in the Supabase `agent` table
   (`supabase/migrations/20260803000009_agent_table.sql:1-19`) under the fixed
   name `company-computer` (`host-setup.ts:3`).
3. On the company computer:

```bash
git clone --recurse-submodules https://github.com/yeomyeonggeori/internkim.git
cd internkim
python3 tools/install-company-host ~/Downloads/internkim-host.json
```

The installer checks for Docker with Linux containers
(`tools/install-company-host:60-66`), prompts for an OpenRouter key through
`getpass` (lines 103-113), writes the agent key `0600` (line 99), builds two
images from the clone (lines 116-125), derives Buzz identity keys inside the
built image (lines 128-136), and runs `docker compose up --detach --wait`
(line 206). State lands in `~/.internkim/companies/<company-id>`.

`INTERNKIM_REGISTER_SECRET` and `POST /api/register` belong to the frozen device
path, which `docs/device.mdx:9-12` says nothing new is designed against. A
customer never holds that secret (`docs/internal/jetson-setup-and-support.md:166`).

**Should a catalog tool ever do this?** No, and the reason is not a preference
about scope. Every input the installer needs exists only on the target machine:
the Docker daemon, a filesystem to write `0600` secrets into, a git clone, and a
secret the operator types into a hidden prompt. A remote tool call has none of
them, so a `host_install` tool could only ever return instructions, which is a
document pretending to be a tool. The part that *is* remote, minting the
connection file, is already an HTTP route with an admin gate. The honest split
follows the machine boundary: the plane issues the credential, the installer
runs where the computer is. An agent working on the company computer is on the
right side of that line and can do the whole thing today by calling
`POST /api/company/host-setup` and then running the script.

## Where the chain broke

**An agent with no credential can start the chain, but the browser window it
opens throws the authorization away the moment the human signs up, so the agent
has to ask the human to start over.**

Two places do the throwing.

First, the unsigned case. The consent screen's sign-up links
(`web-auth-gate.svelte:139-141`) carry no return parameter, and `/auth/claim`
reads none: its four exits all call `goto(homePath)`
(`web/src/routes/auth/claim/+page.svelte:94, 127, 138, 246`), and `homePath` is
`/attendance/` (`web/src/lib/home-path.ts:1`). The `authorization_id` is gone.

Second, the signed-in-without-a-company case.
`web/src/routes/+layout.ts:20-23` redirects any authenticated account that
belongs to no company to `/start`, for every path except `/start` and
`/auth/*`, and discards the path it came from. `/oauth/consent` is such a path.
So even a person who already has an account gets bounced out of consent, founds
a company at `/start`, and lands nowhere near where they were.

The chain is recoverable, in that re-running `claude mcp login internkim` after
founding works, because the browser session persists. It costs a second human
turn and a piece of knowledge the agent has no way to acquire.

A third, smaller break follows. An account with no company that *does* complete
consent receives a working token whose every call answers `403 refused`
(`member-request.ts:99`). The word `refused` names nothing the agent could act
on.

## The genuine human steps

A human must prove or decide something in exactly three places.

**Owning the address.** The emailed one-time code is the only proof the plane
has that whoever typed an address can read mail at it. Nothing else on the
public path substitutes for it. The alternatives that exist are not
substitutes: `person_invite` and `POST /api/member/invite` skip the proof
because an existing active admin has vouched
(`web/src/lib/server/public-api/record/people-tools.ts:135`), and
`AUTH_CLAIM_WITHOUT_EMAIL` skips it because the deployment cannot send mail at
all, refusing every later attempt on the same address
(`claim-without-mail.ts:14-16`). Both are scoped so that somebody already
accountable stands behind the new account.

**Consenting to give a client access.** The consent screen is the whole point
of the OAuth handoff. Anyone may register a client under any name, so the screen
shows where the access goes and warns when the redirect leaves the computer
(`consent/+page.svelte:68-79`). An agent approving its own access would make the
screen decorative.

**Root on their own machine.** The Docker daemon, the OpenRouter key, and the
`0600` secrets directory are the customer's.

Everything else on the path is human-only because nobody wrote the API, and in
two cases the API is written and merely undocumented:
founding a company (`POST /api/company`) and minting the host connection file
(`POST /api/company/host-setup`).

## How other agent products solve the first turn

| Pattern | What the human does | Does the plane support it |
|---|---|---|
| Claude Code `/login` | browser opens, sign in, CLI's loopback receives the code | yes, this is the shape already implemented |
| `claude mcp login <server>` | same, discovered from the `401` challenge | yes, `public-api-mcp-authorization.test.ts` drives it |
| MCP authorization spec browser handoff (RFC 9728 metadata, DCR, PKCE) | as above | yes |
| Device authorization grant (RFC 8628: print a code, poll) | type a short code on another device | no; the authorization server advertises only `authorization_code` and `refresh_token` |
| Pasted authorization code (no loopback) | copy a code from the browser into the terminal | untested; the redirect is whatever the client registers, so a client that registers an out-of-band-style redirect may work, but nothing in this repository exercises it |

The device grant is the one worth naming explicitly, because it is the obvious
thing to reach for when a CLI wants a human elsewhere. Supabase Auth does not
implement it, and adding it means writing an authorization server the plane
deliberately does not own.

## What was built

The existing handoff survives sign-up. Nothing new was introduced; one parameter
that already existed is carried four links further.

The affordance an agent with no credential uses is the one that was already
documented:

```bash
claude mcp add --transport http internkim https://api.intern.kim/v1/mcp
claude mcp login internkim
```

The client opens the consent URL. The person, who has never heard of InternKim,
sees the sign-in card with its two links, picks "start a company", proves the
address with the emailed code, sets a password, registers a passkey, names the
company, and arrives back at the consent screen, where they approve. The CLI's
loopback fires. The agent learns it may continue because `tools/list` answers
with tools instead of `401`.

### One name for the return path

Before the change the concept had one spelling and five hand-written copies of
it: the identifier `returnPath`, the query parameter `return`, and the value
`pathname + search`, written out with `encodeURIComponent` in
`web-auth-session.ts`, `web-auth-gate.svelte`, `+layout.ts`,
`runs/+page.svelte` and `app-navigation.svelte.ts`. Validation had **three**
copies of a *different* rule, all under `web/src/lib/notifications/` and
identical to each other: `ownPath` in `pending-destination.ts`, `safeOpenPath`
in `opened-notification.ts`, and a second private `ownPath` in `arriving.ts`.
The third survived the first pass of this change and was caught in review; the
conformance test in this change was rewritten to look for the rule's shape
rather than its name, which is what would have found it.

`web/src/lib/return-path.ts` now holds all three operations, and every producer
and consumer goes through it:

| | |
|---|---|
| `ownPath(offered)` | the validator the three notification copies were |
| `withReturnPath(destination, returnPath)` | writes the parameter, in front of any fragment and after any existing question |
| `returnPathOf(url)` | reads and validates it back |

Merging the validators closed a hole. The rule all three carried was "starts
with `/`, does not start with `//`", which accepts `/\evil.test`; a browser
reads those backslashes as slashes and leaves the origin. `ownPath` resolves the
candidate against a host nothing can be and compares origins, so a path that
leaves gives itself away whichever spelling it uses, and a candidate that is no
URL at all answers empty rather than throwing. The push-notification paths that
carried the old rule inherit both.

The Go function `safeWebReturnPath` (`internal/admind/web_session.go:397`) is
not a fourth copy. It is a stricter allowlist of six path prefixes, it belongs
to the frozen device path, and it is in another language. Searching by name, by
the rule's shape, by the `'//'` literal and by every consumer of `openPath`
found nothing else.

### The four hops

1. `web/src/lib/components/web-auth-gate.svelte` — both sign-up links are built
   with `withReturnPath`, as `signupURL` already was in spirit.
2. `web/src/routes/auth/claim/+page.svelte` — all four exits go to
   `returnPathOf(page.url) || homePath`.
3. `web/src/routes/+layout.ts` — the bounce for an account that belongs to no
   company redirects to `withReturnPath('/start', returnPath)`, where it used
   to redirect to bare `/start`. One rule for every page, so nothing
   special-cases `/oauth/consent`.
4. `web/src/routes/start/+page.svelte` — founding returns to the carried path,
   and so does the case where the visitor turns out to belong to a company
   already.

### The refusal says what is missing

`web/src/lib/server/member-request.ts` separated the two reasons a caller
resolves to no member. A personal access token whose member row has gone is
told so. A signed-in account that belongs to no company is told that, and where
to start one. The address is derived through `theAppAddressOf`, which moved from
`digest-app-url.ts` to `company-host-redirect.ts` so it sits beside
`theOneAddressOf`, the zone's one home.

### Four decisions review asked for out loud

**The founder never skips the temporary passwords.** `/start` returns to the
carried path by itself only when nobody was invited
(`start/leaving-after-founding.ts`). When there are invitations the card stands,
because it is the one and only place each temporary password is readable, and it
grew a button that continues to the carried path once they have been written
down. A guard alone would have stranded that founder, since the card's only exit
was `/settings/setup`; a button alone would have made somebody who invited
nobody click for nothing.

**The parameter is a one-click path into a consent prompt, and that is accepted.**
`/start?return=%2Foauth%2Fconsent%3Fauthorization_id%3D<theirs>` lands a person
on an approve card for a client they never started, at the moment they are
primed to click through. What the attacker gains over sending a plain consent
link is the timing. The card is still the gate: it names the client and the
account, and it warns in red when the access is received off this computer. The
gap was that nothing told the person they might not have started this, so the
card now says to deny it if they did not. Removing the parameter would remove
the feature it exists for.

**`withReturnPath` splits on `#` first.** No call site passes a fragment today,
so this is a trap waiting: appending after one produces
`/start#frag?return=…`, which a browser never sends. Three lines beat a
precondition nobody reads.

**A stale authorization is now a likely ending, and the copy says so.** The
round trip is consent, an emailed code, a password, a passkey, a company name,
founding, and back. That is minutes. If the authorization
record expires meanwhile the consent card shows its unreadable state, which had
no button and told the person nothing about the company they had just made. It
now says the company is set up either way and to start connecting again from the
app. What the record's lifetime actually is could not be read from this
repository: it is Supabase's, not ours, and `config.toml` does not name it.

### The documentation

`docs/api/index.mdx` and `docs/api/index.ko.mdx` gained a "Starting from
nothing" subsection inside the MCP section: that the browser window works for
somebody with no account, that `POST /api/company` founds a company for a
signed-in account, that `POST /api/company/host-setup` answers the connection
file and refuses an `ik_` token, and that the install itself runs on the company
computer.

## What it touched

| | |
|---|---|
| New | `web/src/lib/return-path.ts`, `routes/start/leaving-after-founding.ts`, three unit test files |
| Return path | `web-auth-gate.svelte`, `auth/claim/+page.svelte`, `+layout.ts`, `start/+page.svelte`, `web-auth-session.ts`, `runs/+page.svelte`, `app-navigation.svelte.ts` |
| Validator merge | `notifications/pending-destination.ts`, `notifications/opened-notification.ts`, `notifications/native-device.ts`, `notifications/arriving.ts` |
| Refusal | `server/member-request.ts`, `server/company-host-redirect.ts`, `server/digest-app-url.ts` |
| Consent copy | `oauth/consent/+page.svelte`, `oauth/consent/consent-text.ts` |
| Documentation | `docs/api/index.mdx`, `docs/api/index.ko.mdx` |

No new route, table, tool, or environment variable.

### What the tests cover, and what they do not

`web/tests/unit/return-path.test.ts` covers the three operations: the backslash
the old rule accepted, the unterminated IPv6 literal that makes the parser
throw, and the collision case `//internkim.invalid/evil`, which degrades to the
local path `/evil` and never reaches the sentinel host. That last one is the
property the whole design rests on, so it is pinned.

`web/tests/unit/sign-up-keeps-the-authorization.test.ts` walks the hops as the
pages compute them and asserts the `authorization_id` survives, then asserts
that nothing else under `src` writes the parameter or judges a path by its
leading slashes. An earlier draft also asserted that each page contained a
particular line; review showed those passed on wrong plumbing, such as the two
sign-up links swapped, so they were removed.
`web/tests/unit/start-leaving-after-founding.test.ts` covers the rule that keeps
a founder on the card that holds the temporary passwords.
`web/tests/integration/public-api-mcp-authorization.test.ts` gained an account
that belongs to no company and asserts the `403` names that and `/start`.

What no test covers is the browser, and one hop lives only there: the consent
page handing its own address to the gate, which is where the whole path starts.
`web/tests/unit` has no rendering harness, the integration suite drives
Supabase's OAuth API and never renders `/oauth/consent`, and the sign-up half
needs a mailbox. The honest cover is Playwright against the local plane. It
would need a `supabase db reset`, which is why it is not in this change.

### Checked against the repository's rules

- *No half-wired automatic behaviour.* Nothing fires on its own. Every step is
  a link somebody clicks or a call an agent makes.
- *One source of truth.* Founding stays in `POST /api/company`; the host
  credential stays in `POST /api/company/host-setup`; the sign-up surface stays
  `/auth/claim`; the return path gained one, which is most of the change.
- *Deterministic gates only for wide or irreversible actions.* Every gate that
  existed is kept: the emailed code, the consent screen, one company per
  account, the admin check on the host key, and the `409` unless
  `replaceExisting` (`company-host-setup.ts:46`). The origin check on the return
  path is a new one, and it guards a redirect, which is wide.
- *The LLM judges meaning, code supplies facts.* The refusal supplies a fact the
  runtime holds and the agent cannot see. No prompt or classifier was added.
- *No mechanism whose main output is side effects.* The output is a URL that
  survives one more navigation.
- *Delete half-baked features.* The automatic hop back from `/start` fires on
  an explicit parameter, its blast radius is one navigation inside this origin,
  and it is off whenever the parameter is absent or points anywhere else.

## What was decided against

These are settled, not open.

**A device authorization grant.** It is the pattern to reach for when a terminal
wants a human elsewhere, and the plane's authorization server does not implement
it. Building it means standing up an authorization server beside Supabase's,
which contradicts passkeys being Supabase's own and `config.toml` being the
schema of record.

**A `company_create` catalog tool.** The catalog's entry condition is that the
caller already belongs to a company: `callingMember` resolves the member row and
its `company_id` before any tool is dispatched
(`member-request.ts:96-99`), and every `RecordContext` carries `companyID`.
Such a tool would have to run for a caller the catalog cannot describe. The
anti-abuse control does not move either: one account founds at most one company
because `claimMemberFor` refuses a second (`api/company/+server.ts:50-51`), and
the account itself costs an emailed code, so a tool would inherit both gates
without adding one and would create a second place where founding is
implemented.

**A `host_install` or `host_connect` catalog tool.** Every input the installer
needs exists only on the target machine, so such a tool could only return
instructions.

**An agent-facing account-creation API.** This is the one that would remove an
anti-abuse control. The emailed code is the only proof of address the public
path has, and no captcha stands behind it (`config.toml:47-48`). An API that
mints accounts without it hands anyone an unlimited supply.

**Turning `AUTH_CLAIM_WITHOUT_EMAIL` on in production.** It exists so a
deployment with no mail is usable at all. Production has mail.

**An onboarding session object, a polling endpoint, or an "onboarding status"
tool.** There is nothing to store. The OAuth authorization is the session and
`tools/list` succeeding is the completion signal, so a status surface would be a
second answer to a question the `401` already answers.

**A new CLI command.** `./internkim` is the device-era binary
(`internal/cli/main.go:159-209`) and the host installer is already a standalone
Python script, so a `signup` verb would put a third sign-up surface next to the
web page and the API.

[`natural-language-onboarding.md`](./natural-language-onboarding.md) takes the
next question: which surfaces this chain serves, what the standards offer for
the ones it misses, and the whole onboarding written out as a conversation. It
corrects one line of reasoning below, on the device authorization grant.

## Open questions

The first three need somebody with access this document does not have. They all
bear on whether the path above actually works for a stranger on production.

**1. Is `AUTH_CLAIM_WITHOUT_EMAIL` set on the production Pages project?** It
decides what the very first screen does. Off, a founder types an address and
waits for an eight-digit code in their mail. On, the first person to name an
address is handed a password on screen and every later attempt on that address
is refused, and `POST /api/auth/signup` answers `503` instead of sending
anything, so "start a company" from the consent screen fails outright. The
repository cannot answer it: the Pages environment is not tracked here, and
`POST /api/auth/claim` returns `{sent:true}` for an unknown address before
reaching the check, so no read-only probe distinguishes the two. Everything
indirect says off. Somebody with the Cloudflare dashboard can settle it in a
minute, and it should then be written down.

**2. Is `[auth.rate_limit] email_sent = 2` two per hour for the whole project?**
`supabase/config.toml:43` sets it and `[auth.email.smtp] enabled = false` means
Supabase's own mailer sends. If the limit is per project rather than per
address, three people founding companies in the same hour means the third gets
no code, and the failure appears to them as a sign-up that silently does
nothing. This change makes the path easier to reach, so it makes that ceiling
easier to hit. Configuring real SMTP raises it; the question is whether it needs
raising yet.

**3. Where is `/v1/mcp` rewritten to `/api/v1/mcp`?** The `401` challenge, the
protected-resource metadata and the documented client address all name
`/v1/mcp`, and `web/src/routes` has no route at that path: the only handler is
`/api/v1/mcp`. It answers on production, so some Cloudflare rule does the
rewrite, and that rule is not in this repository and is named in no document.
Every MCP client that ever connects depends on it. If it is lost or edited, the
symptom is the whole public MCP surface returning 404 with nothing in the tree
to explain why, so it belongs somewhere findable.

The rest are smaller.

4. Does Supabase's dynamic client registration accept a loopback redirect on the
   hosted project? `public-api-mcp-authorization.test.ts` registers
   `http://127.0.0.1:33418/callback` and it works against the local plane, but
   registering a client on production is a write nobody has made.
5. `POST /api/v1/mcp` attaches the `WWW-Authenticate` challenge; the REST
   catch-all `web/src/routes/api/v1/[...path]/+server.ts:49` does not. Should
   `/v1/tools` carry it too, so a non-MCP client can discover the same sign-in?
6. Should the consent screen say something when the visitor has no company yet?
   They are now carried through founding and back without being told that is
   what is happening. A line of copy may be warranted, which is a wording
   decision rather than a mechanism.
