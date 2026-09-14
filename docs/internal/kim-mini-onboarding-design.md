# Kim mini: joining a company with nothing but power

Kim mini is a small computer sold with 김인턴 installed: a customised EDATEC
ED-CLAWBOX on a Raspberry Pi CM5, fanless, in a 4 GB model (64 GB microSD) and a
16 GB model (256 GB SSD in the M.2 slot). It is optional hardware. The same
software runs on any computer a company already has, and the host bundle in
`host/README.md` is what both install.

This document is how a box that arrived in the post becomes one company's host
without a monitor, a keyboard, or anyone from us touching it.

## Constraints

- **Every box leaves the factory identical.** No per-box password, sticker, QR
  code, serial registered to a buyer, or key burned in at assembly. Anything that
  differs between two boxes is put there by the person setting it up.
- **Plugging it in is the whole installation.** Power, optionally a LAN cable,
  and a phone or laptop to finish.
- **A stranger may get there first.** Someone in radio range can configure an
  empty box before its owner does. That is acceptable as long as the owner can
  always take it back.
- **The host stays outbound-only once it belongs to a company**
  (`nothing-reaches-in.md`). The setup page below is the single listener, it
  exists only while the box is empty, and it closes the moment the box is
  claimed.

## Setting it up

1. The box boots empty. It generates a keypair on first boot; the private key is
   written to its own disk and never leaves it.
2. It broadcasts a Wi-Fi network named `Kim mini` with the password `kimmini`.
   The password is printed in the manual and on the product page. It keeps
   passers-by from joining by accident and protects nothing.
3. Joining opens a captive portal. If no LAN cable is connected, the portal
   first asks which office network to use.
4. At `intern.kim`, an administrator of the company asks for a connection code.
   The code is short, single-use, and expires after about ten minutes.
5. The portal takes the code. The box sends the code and its public key to the
   central plane, which binds that public key to the company.
6. The box turns off the portal and its own network, joins the office network,
   and starts the host bundle as that company's host.

The portal accepts injection only while the box is empty and never displays a
value back. A phone that later joins a claimed box's network finds no network
to join.

### What the server does with a code

The central plane already records which fleet belongs to which company:
`claimFleetForCompany` in `web/src/lib/server/control-plane.ts` upserts a
`credential` row of kind `fleet`. A redeemed code calls it with the box's public
key as the fleet identifier. An Ed25519 public key is 43 characters of
base64url, so `external_id` holds the key itself and no column is added.

The row is unique on `(company_id, kind)`
(`20260803000023_credential_company_conflict_target.sql`), so a company has one
host. Claiming a second box moves the company to the new one. That matches
`saas-design.md` §6, which calls the host the company's single persistent
assistant, and it is the first thing to revisit if a company ever wants two.

## The box's identity replaces the agent key

Today a host proves itself with a bearer agent key: a random secret the web app
issues once, the host keeps at `/root/.internkim/secrets/agent-key`, and every
`/api/agent/*` route checks against a SHA-256 hash in `agent.api_key_hash`
(`agentOfKey`, reached through `callingAgent` in
`web/src/lib/server/agent-request.ts`).

A box that holds a private key needs no shared secret. It signs a short
assertion (the company it believes it belongs to, a timestamp, a nonce) and
exchanges it for a short-lived session. `sessionForHost` already mints that
session as the magic-link account `host.<companyID>@agent.internkim.invalid`;
only its input changes from "an agent key whose hash matches" to "an assertion
whose signature matches the public key on the fleet credential". Every route
that resolves an agent from a bearer key resolves it from that session instead.

What changes for the operator:

| | agent key | box keypair |
|---|---|---|
| secret that crosses the network | the key, at issuance and on every call | none; only signatures and short sessions |
| a copied disk file gives | lasting access until someone revokes it | lasting access until someone resets or reclaims |
| a leaked request log gives | the key | an expired session |
| taking it back | revoke in the web app | reset the box or claim another |

A computer the company brings for itself can use the same shape: the host
bundle generates the keypair on install and prints a code prompt in place of a
captive portal.

## Models

A company picks one of two ways to pay for inference.

**Bring your own OpenRouter key.** The box calls OpenRouter directly. The
administrator pastes the key at `intern.kim`, the browser encrypts it to the
box's public key, and the ciphertext travels through the central plane to the
box, which decrypts it into `/root/.internkim/secrets/openrouter-key`. The plane
stores and relays a value it cannot read.

**Subscribe.** The box holds no provider key. It calls a model endpoint on the
central plane with its session; the endpoint checks that the company's
subscription is active and within its rate limit, then forwards to OpenRouter
with our key. That endpoint lives in the SvelteKit app under `/api/v1`
(`public-api-on-the-plane.md`), so a self-hosted plane carries it too.

An earlier gateway of this kind was removed in #1275 because nothing called it.
`capabilityd` still accepts `--openrouter-gateway-secret` and
`--openrouter-gateway-secret-header` (`cmd/internkim-capabilityd/main.go`),
which send a shared secret. With a subscription, capabilityd sends the host
session in their place, and the shared secret flags go away.

## The power button

Kim mini has one button, the power button. It carries two actions:

| press | effect |
|---|---|
| one press | clean shutdown |
| five presses in quick succession | factory reset |
| long press | hard power-off by the CM5 firmware; unavailable to us |

We own the button handler. systemd-logind is told to ignore it
(`HandlePowerKey=ignore`), and a small service reads the key events, counts
presses inside a short window, and either shuts down or resets.

A reset deletes the private key, the company binding, the local Postgres, agent
memory, and the workspace. It then generates a new keypair and reopens the
`Kim mini` network, so the box is indistinguishable from one that just arrived.
This is the whole answer to a stranger who configured the box first: five
presses and set it up again. The old public key stays on the other company's
fleet credential, where it can no longer sign anything.

## Open questions

- **Radio.** Whether the ED-CLAWBOX Wi-Fi can run an access point and a client
  connection at the same time, which step 3 needs when there is no LAN cable. If
  it cannot, the portal switches between the two and the page has to survive the
  gap.
- **Captive webviews.** iOS opens captive portals in a restricted webview that
  may not follow a link to `intern.kim`. The portal should work when the code is
  fetched on a second device or in a normal browser tab.
- **Button wiring.** That the case button is wired to the CM5 `PWR_BUT` line and
  arrives as a key event while the system is running.
- **Key storage.** The CM5 has no TPM, so the private key is a file on the
  microSD card or SSD. Removing the storage and copying it clones the box's
  identity; reclaiming the company from another box is the recovery.
- **Race at setup.** Anyone in range between power-on and claim can inject first.
  The reset button bounds the damage to the time it takes the owner to notice.
