# Runbook: device unreachable / "overloaded" (Jetson)

Symptom: the device feels overloaded or is unreachable — even when **no one is
sending messages**. Remote SSH and the admin/recovery HTTPS endpoints time out
or flap. This runbook explains how to tell the real cause apart, because the
symptom looks the same for several very different problems.

## First insight: "idle but unreachable" rules out message-driven causes

If the device is unreachable while traffic is zero, the load/blocker is a
**constant baseline**, not message processing. That immediately rules out:

- connector enrichment (`/posts` fetches per inbound message)
- agent task launches / addressing classification
- a recent code deploy's per-message cost

Do not waste time on those when the device is idle. Look for something that runs
regardless of user activity (network, a crash-looping service, stray tenants).

## Reading the failure signal

Remote control (SSH and `internkim recover ...`) all ride the **Cloudflare
tunnel** to the device. When the tunnel or the origin is unhealthy you get
distinct errors — learn to read them:

| Signal | Meaning |
|---|---|
| SSH `banner exchange timeout` | tunnel connected but `sshd` did not answer — either CPU starvation OR an unstable tunnel |
| Cloudflare `530` / `error code: 1033` | tunnel connector (`cloudflared`) is **not connected** to the Cloudflare edge |
| Cloudflare `502` (fast) | tunnel is up, but the **origin** (`admind`) is down / not answering |
| `HTTP 000` / `context deadline exceeded` | no response at all (edge or DNS path dead) |

A flapping sequence `1033 → 502 → brief 200 → 1033` means the tunnel keeps
dropping and reconnecting. That is usually a **network** problem, not CPU.

## Diagnose without SSH (HTTP recovery actions)

`internkim recover ssh -action <action>` calls an admind HTTP endpoint, so it
works even when `sshd` is starved — as long as the tunnel + admind are up.
Actions (see `internal/admind/recovery.go`):

- `status` — cheap; reports `ssh` / `cloudflared` / `cloudflared-node-ssh` states
- `journal-tail` — recent journal (great for spotting crash loops / log spam)
- `restart-ssh`, `restart-cloudflared-node-ssh` — restart the SSH path
- `reboot` — schedules `systemctl reboot` ~3s out
- `stop-tenant-pilots` — disables `internkim-tenant-*` / `internkim-mattermost-pilot-*` units, prints `free -m`
- `remove-tenant-pilots` — same but deletes the unit files (survives reboot; `stop` alone does not)
- `unlock-mattermost-admin`

### Watch-then-strike (when windows are narrow)

When the tunnel only opens for a second at a time, polling the heavy action
blindly rarely lands. Poll the cheap `status` and fire the real action the
instant it responds:

```sh
for i in $(seq 1 30); do
  st=$(timeout 18 ./internkim recover ssh -action status 2>&1)
  if echo "$st" | grep -qiE "5[0-9][0-9]|1033|deadline|timed out|context"; then
    sleep 30                      # still down, wait
  else
    ./internkim recover ssh -action stop-tenant-pilots   # window open — strike now
    break
  fi
done
```

## Decision tree

1. Run `journal-tail` (retry until a window catches it).
   - **Journal is full of `cloudflared` errors** — `Failed to refresh DNS local
     resolver: ... i/o timeout`, `DialContext error: dial tcp ...:7844: i/o
     timeout`, `TLS handshake with edge error: connection reset by peer`,
     repeating `Lost connection / Retrying` every 1–3 min:
     → **NETWORK UPLINK problem** (WiFi / DNS / router). DNS lookups timing out
     means it is the network path, not CPU (a CPU-pegged box still resolves
     DNS). Go to "Fix: network".
   - Journal shows a specific `internkim-*` service restarting/erroring in a
     loop → that service is the culprit; pull its unit logs.

2. Run `stop-tenant-pilots`.
   - Lists/stops `internkim-tenant-*` / `internkim-mattermost-pilot-*` units and
     `free -m` jumps → **stray pilot tenants** were the load (they belong on the
     Mac-Studio containers, not the Jetson; a reboot revives `stop`-only ones —
     use `remove-tenant-pilots` to make it stick).
   - **Stops nothing** (no units) and SSH still dead → pilots are not the cause.

3. Only if you can get SSH: `top -bn1`, `ps -eo pcpu,pmem,comm --sort=-pcpu`,
   `free -h` to confirm a real CPU/RAM hog (e.g. a model server hot loop).

## Fix: network (most common "idle but unreachable")

Root cause when the journal shows DNS/dial/TLS timeouts: the device cannot hold
a stable outbound connection, so `cloudflared` thrashes and every remote path
dies. Software changes will not fix this. In order of effectiveness:

1. **Wire the Jetson via Ethernet** instead of WiFi — most reliable fix.
2. **Hidden SSID** makes Linux reconnection slower and flakier (the client must
   actively probe for the SSID by name), which widens every disconnect window.
   The setup configures this via `nmcli ... wifi.hidden yes`
   (`internal/cli/wifi_profiles.go`), but only when the device was set up as
   hidden. Verify it is actually set:
   ```sh
   nmcli -f 802-11-wireless.hidden connection show <connection-name>
   ```
   If it shows `no` for a hidden network, that is the bug — fix it, or just
   broadcast the SSID (hidden gives negligible security and costs reliability):
   ```sh
   nmcli connection modify <connection-name> wifi.hidden yes && nmcli connection up <connection-name>
   ```
3. WiFi: check signal strength / congestion, move closer, change channel.
4. Check the router and its DNS; reboot the router.
5. On the LAN, restart the tunnel after fixing the link:
   `systemctl restart cloudflared cloudflared-node-ssh`.

## LAN fallback (when you are on the same network)

Cloudflare-independent. The Jetson's LAN IP has been `192.168.0.248`.

```sh
nmcli dev wifi                       # WiFi signal / link quality
top -bn1 | head -15                  # CPU/RAM hogs
systemctl list-units 'internkim-tenant-*' 'internkim-mattermost-pilot-*'
journalctl -p err -n 50 --no-pager
systemctl restart cloudflared cloudflared-node-ssh   # restore remote access
```

## Not this (ruled out in the 2026-06-20 incident)

- **Pilots** — `stop-tenant-pilots` found zero units; they were already dead.
- **A code deploy / enrichment** — message-driven, cannot explain idle load.
- A separate real CPU flood exists historically (capabilityd hammering the
  self-hosted Mattermost `/posts`, ~98% CPU) — that one is **message-driven**
  and has a 50/s rate-limit backstop; do not confuse it with the idle-network
  case. The enrichment-delay root fix is still open.
