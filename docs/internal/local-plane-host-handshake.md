# The host handshake on a local plane

[`native-install-rig.md`](./native-install-rig.md) said step 5 could not be
closed on this Mac, because the token `POST /api/agent/host-session` mints is
`HS256` locally and the connection gateway verifies `ES256` or `RS256` against a
published key set. That is true of the token the rig signs today. The conclusion
drawn from it, that no local stack can produce a token the gateway accepts, is
wrong.

A local Supabase stack already holds an `ES256` signing key, publishes its public
half at `/auth/v1/.well-known/jwks.json`, and hands PostgREST, Realtime and
Storage the same key set. Its private half sits in the auth container's
environment. Point `SUPABASE_JWT_SIGNING_KEY` at that key and the host's own
handshake passes, with nothing about the stack changed and no weakening of what
the installed box does.

Everything below was run against the local plane on 2026-09-23 with Supabase CLI
2.116.0 and GoTrue v2.196.0.

## What the stack holds

`supabase status -o env` names `JWT_SECRET` and no signing key, which is the
whole basis of the older claim. The key is one level down:

```
docker inspect supabase_auth_<project_id> --format '{{range .Config.Env}}{{println .}}{{end}}' | grep GOTRUE_JWT_KEYS
```

```
GOTRUE_JWT_KEYS=[{"kty":"EC","kid":"b81269f1-21d8-4f2e-b719-c2240a840d90","use":"sig",
"key_ops":["sign","verify"],"alg":"ES256","ext":true,"d":"…","crv":"P-256","x":"…","y":"…"}]
```

`d` is the private half. The same `kid` is what the stack publishes and what
every verifier it starts is configured with:

| Reader | Where its key set comes from | Holds `b81269f1…` |
|---|---|---|
| the gateway | `GET /auth/v1/.well-known/jwks.json` | yes |
| PostgREST | `PGRST_JWT_SECRET` | yes, beside the `oct` shared secret |
| Realtime | `API_JWT_JWKS` | yes, beside `API_JWT_SECRET` |
| Storage | `JWT_JWKS` | yes, beside `AUTH_JWT_SECRET` |

So one value satisfies both ends at once, which is what
`native-install-rig.md` said did not exist locally.

The CLI compiles this key in as a constant; no project generates its own.
`supabase init` in an empty directory under a different `project_id`,
started on its own ports, published the identical `kid`, `x` and `y`, and those
three strings appear verbatim in the CLI binary. It is a well-known development
key of the same standing as `super-secret-jwt-token-with-at-least-32-characters-long`
and the demo `anon` key.

## What was run

A `wrangler dev` gateway from the canonical
`workers/connection-gateway/wrangler.jsonc` with only `main` and `vars`
rewritten, exactly as `tools/verify-personal-settings.ts` and the rig already do,
and the app under `vite dev` with `SUPABASE_JWT_SIGNING_KEY` set to the JWK
above. Then the relay's own two steps, in the relay's own order: `POST
/api/agent/host-session` carrying the agent key as a bearer token, then a
websocket to `/company/<id>/host` carrying the minted token as a bearer header.
`host/relay/gateway-socket.ts` does the same two calls with the same headers.

```
host-session 200 {"companyID":"d73b624e-…","accessToken":"eyJhbGciOiJFUzI1NiIs…"}
token header { alg: "ES256", typ: "JWT", kid: "b81269f1-21d8-4f2e-b719-c2240a840d90" }
token claims { role: "authenticated", email: "host.d73b624e-…@agent.internkim.invalid",
               app_metadata: { company_id: "d73b624e-…" }, iss: "http://127.0.0.1:54321/auth/v1" }
gateway host handshake: 101 open
my_app_company() d73b624e-… (expected d73b624e-…)
```

The last line matters as much as the handshake: the same token still reads
through PostgREST under row level security, which is the refusal the older
document predicted a self-issued key would trade the first one for.

Then the round trip, with both sides authenticating the way they do in
production. A member signed in through GoTrue with a password and carried the
access token as the `internkim.bearer.` subprotocol to `/company/<id>/client`;
the host held the socket it had just opened and answered the routed call.

```
member token alg ES256
host token alg ES256
host socket open
browser sees the host connected: true
host received messenger.conversations from member 1690f89a-…
browser received {"kind":"result","requestID":"ddf111ab-…","status":200,
                  "body":{"messages":[{"from":"이샘플","text":"안녕하세요"}],"answeredBy":"the host"}}
```

Neither probe used `GATEWAY_SERVER_KEY` or `GATEWAY_ADMIN_TOKEN`. The gateway
config given to `wrangler dev` carried an admin token nothing called.

### The refusals

Three tokens against `/company/<id>/host` on the same gateway, to show the check
is still a check:

| Token | Answer |
|---|---|
| `HS256` off the shared secret, what the rig signs with today | `401 the token is signed with HS256` |
| `ES256` off a key this stack does not publish | `401 the issuer published no key with that id` |
| `ES256` off the stack's own published key | verified, and reached the durable object |

The middle row is the cost the older document feared, and it is real: a key the
app mints for itself is a key nothing local verifies. The cost only applies to a
key the stack does not already hold.

## What would change

`signing_key_of_secret` in `web/scripts/local-plane-signing-key.sh` is the one
definition, sourced by `web/scripts/test-integration.sh`,
`web/scripts/e2e-central.sh`, `web/scripts/e2e-organization-central.sh`,
`tools/start-local-fleet-central-plane` and `tools/verify-personal-settings.ts`.
`tools/native_install_rig.py` keeps a Python copy of the same three lines, which
is a second hand-kept copy of one rule and should collapse into the first.

The replacement reads the running stack rather than deriving anything from
`JWT_SECRET`: take `GOTRUE_JWT_KEYS` from `supabase_auth_<project_id>` and emit
its first entry. Reading it beats hardcoding the constant, because a CLI upgrade
that changes the key then changes nothing else.

The rig's step 5 table can then stop saying the message round trip waits, and
`native-install-rig.md`'s "The one thing no local plane can do" becomes a
smaller claim about what remains.

## What a green local run would and would not cover

Covered, and not covered before:

- the app mints a host token a published key verifies
- the gateway accepts the host's handshake on the strength of that token alone
- the token still reads through row level security, so the relay's own reads work
- a signed-in member reaches the company over the gateway and an answer returns
  over the host's socket

Not covered, and unchanged by any of this:

- **The guest's relay was not the host in these probes.** A test script held the
  host socket and answered the routed call. Running the real `internkim-relay`
  needs the Debian package and the arm64 guest the rig builds, which was outside
  this investigation. The authentication it performs is byte-for-byte what was
  run here; its messenger is not.
- **Hostname resolution and the browser download of the connection document.**
  The rig substitutes an address the guest can route to.
- **Passkeys.** Local GoTrue answers `404` at `/auth/v1/passkeys`.
- **The production plane's key.** `web/scripts/issue-record-signing-key.ts`
  registers an `ES256` key through the management API, and the local path
  exercises no management API. What matches is the token's shape and the
  verification both ends perform, not the registration.

## Why `signing_keys_path` was not taken

Supabase CLI 2.116.0 accepts `signing_keys_path` under `[auth]`, and it works: a
scratch project pointed at a self-generated `ES256` key published that key and
configured PostgREST with it. Two things argue against it here. It drops the
`oct` shared secret from PostgREST's key set, so anything still presenting a
legacy `HS256` token is refused. And `supabase config push` sends the whole
`config.toml` with no dry run, so a local-only setting in that file is one push
away from reaching the project. Whether the push carries this particular setting
was not tested, and there is no safe way to test it. Reading the key the stack
already has needs no entry in `config.toml` at all.

## Residue

Both probes created a company, an agent row, a member and an account on the
local plane and deleted all four afterwards. One earlier run exited before its
cleanup and left a company behind; it was removed, and a query for companies
matching `probe` now returns nothing.

Deleting host accounts also removed
`host.000000cc-0000-0000-0000-000000000001@agent.internkim.invalid`, which
belongs to the seeded fixture company. It is not in `supabase/seed.dev.sql`:
`keepHostAccount` creates it on demand, so the next `POST
/api/agent/host-session` for that company makes it again.

The scratch Supabase project was stopped and its directory deleted. The shared
local plane's own containers were read and never restarted.
