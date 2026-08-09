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
dies. Software changes will not fix this.

### Localize first: local WiFi vs WAN uplink (2026-06-20 LAN diagnosis)

Before touching the WiFi, prove **which hop** is bad. "Network problem" has two
very different homes and they need opposite fixes:

- **device → router (local WiFi)** — fixed by Ethernet, signal, channel.
- **router → ISP (WAN uplink)** — Ethernet does **nothing**; it's the router/ISP.

From a LAN shell on the device, compare the gateway against the WAN:

```sh
iw dev wlP1p1s0 link | grep -E 'signal|bitrate'   # local RF quality
iw dev wlP1p1s0 get power_save                     # expect: off
ping -c 20 -i 0.2 192.168.0.1                      # GATEWAY: local hop
ping -c 30 -i 0.2 1.1.1.1                           # WAN: beyond the router
ping -M do -c1 -s 1472 198.41.200.193              # path MTU to CF edge (1500 ok?)
```

Read it like this:

| Gateway ping | WAN ping | Verdict |
|---|---|---|
| loss / high RTT | (any) | **local WiFi** — Ethernet/signal/channel will help |
| clean (~1ms, 0%) | RTT spikes (100–300ms+), high `mdev`, ~0% loss | **WAN/ISP uplink jitter** — Ethernet will NOT help |
| clean | clean | not the network — look at origin/service |

The 2026-06-20 case was the **second row**: signal `-46 dBm` @ 866 Mbit/s,
powersave `off`, gateway `~1.2ms / 0% loss`, full 1500 MTU to the edge, but WAN
RTT intermittently `5ms → 320ms` (`mdev 108ms`) at **0% loss**. That jitter alone
made `cloudflared` edge TLS handshakes (`:7844`) time out → all four tunnel
connections dropped, then re-registered minutes later → user sees `502` /
SSH `banner exchange timeout` in the gap, then self-heal. The device, WiFi, DNS,
MTU, and origin (`:18080 → 200`) were all healthy.

`mdev`/jitter spikes at **0% packet loss** is the bufferbloat signature: the
router's upload queue filling under load. Fix order for the WAN case:

1. **Reboot the router; update its firmware.**
2. **Enable SQM / QoS (bufferbloat control)** on the router — directly targets the
   0%-loss latency spikes.
3. **ISP line quality** — sustained WAN jitter beyond the router is the ISP;
   escalate or add a secondary uplink.
4. Software only mitigates, never cures: making `cloudflared` more jitter-tolerant
   (`--protocol quic`, reconnect/keepalive tuning) shortens flap windows but does
   not fix the uplink.

### Fix order when it IS the local WiFi

1. **Wire the Jetson via Ethernet** instead of WiFi — most reliable fix *only when
   the gateway hop itself is bad* (see localization above).
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

## Tunnel recovery: how it works, and how to improve it

We cannot fix the WAN/ISP jitter from the device, so the strategy is **recover
fast and reliably from each flap** instead of preventing it. Three layers, all
provisioned (`internal/cli/wifi_profiles.go`, `setup_flow.go`, `firstboot.go`):

1. **`cloudflared --protocol quic`** (the tunnel transport). QUIC runs over UDP
   with its own loss/jitter recovery and per-connection independence. Observed
   2026-06-20: under the same WAN jitter, only one of the four edge connections
   drops at a time (the others keep the tunnel at HTTP 200) and it **re-registers
   in ~1s**, versus HTTP/2 where all four dropped together and stalled for
   minutes. UDP `:7844` outbound must be open (verified). If an ISP ever throttles
   UDP, revert to `http2` (`--protocol`) — the one knob.
2. **`internkim-tunnel-recovery` watchdog** (the backstop). A timer fires every
   60s and runs `/usr/local/lib/internkim/tunnel-recovery.sh`, which reads the
   `cloudflared` journal for the last 75s: if there were disconnect events but
   **zero** `Registered tunnel connection`, the connector is stuck, so it
   `systemctl restart cloudflared`. This bounds a stuck-down window to ~75–135s
   and forces fresh edge-IP selection. It is a no-op when the tunnel is healthy
   (recent registration) or quiet (no disconnects), so it never churns a working
   tunnel.
3. **systemd `Restart=always`** only catches a *process exit*; during a flap
   `cloudflared` stays alive and retries internally, so layers 1–2 are what
   actually carry recovery. (`internkim-wifi-recovery` every 2 min and
   `internkim-network-snapshot` every 5 min cover the WiFi-link and diagnostics
   sides separately.)

Verify the recovery layers on the device:

```sh
grep -h ExecStart /etc/systemd/system/cloudflared*.service        # expect --protocol quic
systemctl is-enabled internkim-tunnel-recovery.timer              # expect enabled
journalctl -t internkim-tunnel-recovery -n 20 --no-pager          # watchdog restart history
journalctl -u cloudflared --since=-10min | grep -oE 'protocol=(quic|http2)' | sort | uniq -c
```

### To improve recovery further (in rough order of leverage)

1. **Cure the source (outside the device):** the real fix is the WAN/ISP — enable
   **SQM/QoS (bufferbloat control)** on the router, reboot/update it, or escalate
   the line to the ISP. Everything below only mitigates.
2. **Second uplink / failover:** an LTE or second-WAN path with `cloudflared`'s
   HA connections riding both removes the single-uplink dependency. Highest-impact
   software-reachable option when the primary ISP cannot be fixed.
3. **Metrics-based watchdog instead of journal grep:** run `cloudflared` with
   `--metrics 127.0.0.1:<port>` and have the watchdog read the
   `cloudflared_tunnel_ha_connections` gauge (restart when it hits 0). More precise
   than parsing log lines, and immune to journal wording changes. Current ceiling:
   the watchdog keys on log strings (`Registered tunnel connection`, `Lost
   connection`, …) — if cloudflared changes its log format, update the patterns.
4. **Tune the watchdog window/interval** (`window=-75s`, `OnUnitActiveSec=60s` in
   `wifi_profiles.go`): shorter = faster restart but more risk of restarting a
   tunnel that was about to recover on its own. 75s/60s was chosen so QUIC's ~1s
   native reconnect almost always wins first and the watchdog only fires on a
   genuine multi-minute stall.
5. **External watcher** (off-device): a second host that probes the public URL and
   triggers `internkim recover ssh -action restart-cloudflared-node-ssh` when it
   stays down — covers the case where the device itself is too wedged to self-heal.
6. **Wired Ethernet** only helps if the *local* hop degrades (gateway ping loss);
   it does nothing for WAN/ISP jitter. See "Localize first" above before reaching
   for it.

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
