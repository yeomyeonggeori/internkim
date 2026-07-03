# InternKim Companion Browser Bridge

Manifest V3 extension that reports DOM element coordinates from a plain,
un-automated Chrome session to the companion Go daemon. It carries no CDP
automation flags and sets no `navigator.webdriver` signal itself — see
`.claude/plans/warm-kindling-sunrise.md` for why this replaces CDP-driven
automation.

## Loading

This is never published to the Chrome Web Store. The companion daemon (or a
developer, for manual testing) launches Chrome with:

```
--load-extension=<path to this directory>
--disable-extensions-except=<path to this directory>
```

No `--enable-automation` or `--remote-debugging-port` flags are present in
that launch command (see `internal/browser/extension_runtime.go`,
`ExtensionInputRuntime.launchCommand`).

## Wire protocol

The extension's background service worker (`background.js`) is the daemon
side of `ws://127.0.0.1:<port>` and must match the Go structs field-for-field.
The message envelope, message types, and payload shapes are defined in:

- `internal/browser/extension_protocol.go` — `ExtensionBridgeMessage` and
  payload structs (`ExtensionReadyPayload`, `ExtensionDOMSnapshot`,
  `ExtensionElementDescriptor`, `ExtensionElementRect`,
  `ExtensionViewportGeometry`, `ExtensionResolvedRef`), plus the message-type
  constants.
- `internal/browser/extension_bridge.go` — `ExtensionWebSocketBridge`, the
  hand-rolled WebSocket server this extension connects to, and the
  request/response round-trip (`RequestID` correlation).

Do not duplicate that schema here; read those files directly when changing
either side of the protocol.

`content.js` and `background.js` also exchange an internal, extension-only
message shape over `chrome.runtime.sendMessage`/`chrome.tabs.sendMessage`
(`{internkimBridgeType: "snapshot" | "resolveRef" | "domChanged", ...}`).
That shape is not part of the daemon contract and can change freely as long
as `background.js` keeps translating it into `ExtensionBridgeMessage` frames.

## Open integration point: port discovery

`ExtensionWebSocketBridge` binds to an ephemeral port by default
(`ListenAddress` empty → `127.0.0.1:0`), and the port is only known at
runtime via `bridge.Port()` after `Start(ctx)` — there is currently no
mechanism in `internal/browser` for getting that port to the extension before
Chrome launches.

This extension resolves the port by fetching its own bundled
`runtime-config.json` (`background.js`, `internkimResolvePort`):

```json
{ "port": 12345 }
```

That file does not exist in this directory yet — writing it is the
responsibility of whichever task wires up the actual Chrome launch (see the
plan's `internal/companion/executor.go` / `ExtensionInputRuntime.launchChrome`
integration). The expected flow is: call `bridge.Start(ctx)`, read
`bridge.Port()`, write `runtime-config.json` into the extension directory
pointed at by `ExtensionInputRuntime.ExtensionPath`, then launch Chrome. Since
Chrome reads an unpacked extension's directory fresh on each load, a file
written just before launch is picked up without repackaging.

If `runtime-config.json` is missing or unreadable (for example, during manual
smoke testing), `background.js` falls back to a fixed default port,
`internkimDefaultBridgePort` (currently `8787`). Point a manually-started
`ExtensionWebSocketBridge{ListenAddress: "127.0.0.1:8787"}` at that port to
test without wiring up the config file, or use the config file to test
against an arbitrary ephemeral port.

## Manual smoke test

1. Build a minimal Go test harness (or a `go run` scratch file) that starts
   an `ExtensionWebSocketBridge{ListenAddress: "127.0.0.1:8787"}`, calls
   `Start(ctx)`, then `WaitForReady(ctx)`, then drives `RequestSnapshot`,
   `ResolveRef`, and `Navigate` from the terminal so you can see each
   response as it arrives.
2. Launch Chrome by hand with:
   ```
   /path/to/Google\ Chrome --load-extension=$(pwd)/companion/browser-extension \
     --disable-extensions-except=$(pwd)/companion/browser-extension \
     --no-first-run --user-data-dir=/tmp/internkim-extension-smoke
   ```
3. Open `chrome://extensions`, confirm the extension loaded with no manifest
   errors, and open its service worker's inspector (`service worker` link)
   to watch `console` output and confirm the WebSocket connects (`onopen`)
   and sends a `ready` frame.
4. In the harness, call `RequestSnapshot` and confirm the returned
   `ExtensionDOMSnapshot.Elements` matches what's visible on the current tab
   (navigate the tab manually first, since step 1's harness does not open a
   tab itself).
5. Call `ResolveRef` with a `ref` value copied from the snapshot output and
   confirm `Found: true` with a matching `Rect`.
6. Call `Navigate` to a second URL and confirm the tab actually navigates and
   a fresh `ExtensionDOMSnapshot` for the new page comes back as `navigated`.
7. Kill the service worker from `chrome://extensions` (or wait past 30s
   idle) mid-session and confirm the next `chrome.alarms` tick (within
   ~25s) reconnects the WebSocket and re-sends `ready` without a full
   extension reload.

There is no automated Go integration test for this extension from this task
alone — `ExtensionBridge` is an interface specifically so `internal/browser`
tests can inject a fake connection instead of a real browser. Wiring an
end-to-end test that drives a real Chrome + this extension is out of this
task's scope.
