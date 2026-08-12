# Nothing reaches in

## The rule

The company's computer accepts no inbound connection, and the repository holds
no code that knows how anyone reaches it. How an operator gets a shell on that
machine is their configuration, not our product.

This supersedes `self-hosting-and-exposure-plan.md`, which planned the opposite:
an `EXPOSURE` setting with Cloudflare, WireGuard-over-VPS, direct and overlay
backends behind one interface. That plan existed to answer a question the
central plane has since answered instead.

## The question that went away

The old plan opened with a constraint:

> A staff member reaches a self-hosted box behind NAT, from anywhere, in any
> browser. Any box behind NAT needs exactly one public reachable point: a SaaS
> tunnel, a VPS relay, or a public IP with port forwarding.

That is true, and it no longer applies, because the browser stopped going to
the box. A person signs into the company web app on Cloudflare Pages, which
talks to Supabase; the relay on the company's computer answers over its own
outbound connection to Supabase Realtime. `host/README.md` states it as the
shape of the host bundle: nothing listens off loopback, so there is no tunnel,
no inbound port and no public hostname to maintain.

Every remaining public surface is a leftover of the appliance the product used
to be.

## What the tunnel is doing today

Not one thing. Five, with different owners.

| what | who wants it | what replaces it |
|---|---|---|
| the device's web (`/admin`, `/flow`, `/calendar`, `/attendance`) | staff | the company app on Pages |
| Mattermost's public URL | staff | the company's own messenger host |
| OTA upload over Admin HTTPS | the operator | see below |
| SSH | the operator | the operator's own `~/.ssh/config` |
| fifteen PoC tenants' ingress | nobody | deletion |

Roughly what each costs to keep:

```
web/src/lib/cloudflare.ts                     903   Access app, policy, DNS, tunnel API
web/src/routes/api/register/+server.ts        378   registering a device creates all of it
internal/tenantruntime/                     8,076   the PoC tenant runtime, tunnel included
internal/admind/cloudflare_access.go          309   the device trusts an Access email header
internal/provisioning/steps/step_tunnel.go     96
```

## SSH belongs in the operator's config

One place already has the right shape. `deploy_poc_container.go` reads the
proxy command off the target record:

```go
if target.SSHProxyCommand != "" {
    arguments = append(arguments, "-o", "ProxyCommand="+target.SSHProxyCommand)
}
```

`internal/cli/main.go` has the wrong one, hard-coded:

```go
client.proxyCommand = "env GODEBUG=netdns=go TUNNEL_EDGE_IP_VERSION=4 cloudflared " +
    "--edge-ip-version 4 --edge-bind-address 0.0.0.0 access ssh" +
    cloudflareAccessServiceTokenArguments() + " --hostname %h"
```

Both go. The product calls `ssh <host>` and nothing else. An operator who wants
that host reachable from another network writes it once:

```
Host device-*
  ProxyCommand cloudflared access ssh --hostname %h
```

Cloudflare, Tailscale, a jump host, WireGuard — the repository cannot tell which,
which is the point. What goes with the hard-coded command: the `--cloudflare-ssh`
flag, the LAN-probe-then-fall-back routing, and the diagnostic that explains a
banner timeout by naming a `cloudflared access login` the product should never
have known about.

## OTA was a symptom

`internkim deploy` builds a release manifest, uploads component bundles to Admin
HTTPS, and runs an apply engine on the device. About three thousand lines:

```
internal/admind/release_updates.go           1,235
internal/cli/release_command.go                945
internal/releaseset/                           474
internal/admind/release_uploads.go             330
workers/release-registry/                       87
```

It exists because the device is an appliance the customer cannot log into. Push
was the only direction available, so we built a package manager and a channel to
carry it.

The premise is gone twice over. The machine is one the company already owns and
an operator already has a shell on, and taking the tunnel away removes the
transport the upload used, so keeping OTA would mean keeping the tunnel *for
the deploy tool*. That is the tail wagging the dog.

What replaces it is what every other daemon uses. A signed tarball or an
OS package — Homebrew on macOS, apt or rpm on Linux — plus `systemctl restart`.
`host/entrypoint.sh` already knows the boot order, `internkim-relay.service`
already declares the restart policy, and `make build-relay` already produces one
executable that needs no Bun and no `node_modules`.

That leaves one honest gap. The apply engine also does staging, checksum
verification and a health check before it calls a release good, and a package
manager does the first two but not the third. The health check is worth keeping
as a command the operator runs against a machine they already have a shell on.

## Order of change

Each step stands alone, and the device keeps working through all of them. Steps
1 and 2 are done.

1. **The PoC goes.** `internal/tenantruntime`, the `poc-container` target kind,
   `poc/`, `web/src/routes/poc-admin`, and the two PoC documents. The largest
   single block, out of scope already, and it removes the fan-out that deploys
   to a machine nobody asked for.
2. **SSH becomes a proxy command.** Behaviour identical, the transport gone from
   the source.
3. **OTA becomes a package.** `internkim deploy` becomes build, ship, restart,
   verify over the same `ssh` as everything else. `--legacy-ssh` already does
   this; the work is making it the only path and deleting the engine the other
   one drives.
4. **Registration stops creating a tunnel, an Access application and a DNS
   record.** After this a newly registered device is reachable on its own
   network. This is the step that ends browser access to the device.
5. **admind stops trusting an Access email.** Nothing sends that header once
   step 4 lands. The Mattermost session and the Intern Kim session stay.

The package step comes before registration for a reason worth stating. A device
registered after step 4 has no tunnel, so it has no Admin HTTPS, so it has no
OTA upload path; doing them the other way round would leave a device that can
be set up and never updated.

Step 4 also does less to a running device than it sounds. Registration is what
*creates* a tunnel, and deleting that code tears none down, so the Jetson in
production keeps the tunnel it already has and keeps answering on it. What
changes is that no new device gets one.

Step 4 is the one with a decision inside it, and it is not a code decision: it
asks whether anyone still opens a device in a browser. Both surfaces are live
today. `zd2df6qt6jmc.intern.kim/flow/`, `/calendar/`, `/attendance/` and
`/tasks/` all answer 200 from the device, and `space.intern.kim` answers the
same screens from the central plane, so the question is which one people
actually use.

## What this does not do

It does not make the company's computer unreachable to its own agent. The relay,
chatd, capabilityd and blueclaw all speak outbound and keep doing so.

It does not remove Cloudflare from the product's own hosting. The company web app
is on Pages and stays there; what goes is the code that makes *the customer's*
machine a Cloudflare origin.

It does not settle which package format ships first. That is step 5's to answer
against a real machine.

## Evidence to keep

- `rg -i cloudflare` finds nothing outside `web/scripts` and the Workers that
  deploy our own hosting.
- The device answers on its own network with no tunnel process running.
- A deploy runs over plain `ssh` with no flag naming a transport.
- `host/README.md`'s claim that the bundle needs no tunnel is true of the whole
  repository, not just that bundle.
