# Blueclaw OSS Boundary

> Superseded 2026-09-03 by #1347: Google Workspace was removed entirely. No Google code, tool, skill, route, secret or provisioning step remains, so what follows describes a path that is gone.

## Goal

Blueclaw should be releasable as an open-source agent runtime without carrying internkim appliance code, product integrations, or user-local secrets. The boundary is capability-based: Blueclaw decides what capability it needs, while internkim decides where and how that capability runs.

## Layers

| Layer | Ownership | Public boundary |
|---|---|---|
| Blueclaw runtime | Blueclaw OSS | Conversation runtime, policy, task state, memory, scheduler, skill selection |
| Capability protocol | Shared contract | Versioned JSON request, response, descriptor, routing, denial, and resource scope types |
| Companion runtime | Reusable provider core | User-local browser and local model execution |
| internkim product | Private appliance layer | Device provisioning, Cloudflare, Mattermost, Slack, Signal, Google, OpenRouter, admin UI, fleet, packaging |

## Blueclaw OSS Should Keep

- Capability client code that calls a configured provider endpoint.
- Provider-neutral tool names and descriptor handling.
- Policy decisions about which actor may request which tool, read from the descriptor's `sideEffectClass` and `requiresApproval` rather than from a list of tool names.
- The requirement side of a skill: the tool names its `tool-references` state, and refusing to offer a skill whose tools this runtime was not given.
- Task, memory, scheduler, and skill orchestration.
- Runtime configuration fields that describe the capability endpoint, not provider implementation details.

## Blueclaw OSS Should Not Keep

- Provider tokens, browser cookies, local file paths, local model paths, or device secrets.
- internkim-specific URLs, release downloads, command names, service names, or filesystem paths.
- Cloudflare, Mattermost, Slack, Signal, Google Workspace, OpenRouter, Jetson, guest provisioning, or fleet code.
- Companion pairing broker state or signed upload storage.
- Product-specific capability catalogs such as `flow.*`, `site.app.*`, and Google bridge tools.
- Prose describing a product tool's parameters or its approval rules. A skill states the workflow and the judgment; the descriptor states the fields, the enums and whether the call is gated, and the runtime reads it.

## Capability Protocol

Blueclaw's `protocol/` package defines the shape of a descriptor and defines no tool. internkim authors the catalog in `web/src/lib/server/public-api/catalog/` and generates `pkg/capabilityprotocol/generated/` from it, hashing the catalog into a manifest through the protocol package's `buildProtocolManifest`. Blueclaw's own `protocol/generated/` covers the envelope schemas alone, which is why the two manifests differ and neither is compared to the other: the identity check reads what internkim stamps into `runtime.json` and what capabilityd reports, both from internkim's generated directory.

The alternative was to keep generating both directories from one place, which keeps a single manifest but leaves blueclaw's tree holding a generated catalog its own tooling could neither produce nor verify.

The public protocol lives in `pkg/capabilityprotocol`. It contains only shared schema and standard capability descriptors:

- `Descriptor`
- `RegistryResponse`
- `ToolInvokeRequest`
- `ToolInvokeResponse`
- `ResourceScope`
- `CompanionJobEnvelope`
- `DenialResult`
- execution modes
- backend names
- availability codes

internkim-specific capability catalogs are authored in the web app beside the routes that answer them, and `internal/capabilities` assembles the generated descriptors into the sets each process offers.

## Companion Boundary

Companion is a trusted user-local capability provider. Its reusable core can advertise and execute browser and local model capabilities.

The internkim companion binary is the product around that core: pairing links, the background service, release packaging. Product controls should use product-owned device endpoints, not companion broker endpoints.

## Runtime Settings Boundary

Device runtime settings belong to the internkim appliance layer. A paired Companion may authenticate a request to change a runtime setting, but the endpoint should not live under the companion broker namespace.

Current rule:

- Companion broker: `/_internkim/companion/*`
- Device runtime settings: `/_internkim/runtime/*`

The remote model setting uses `/_internkim/runtime/remote-model` because it writes the Blueclaw runtime configuration and restarts the Blueclaw service.

## Migration Order

1. Keep `internal/capabilities` as the product catalog wrapper and move shared protocol types to `pkg/capabilityprotocol`.
2. Move device runtime settings out of `/_internkim/companion/*`.
3. Neutralize Companion UI copy that exposes Blueclaw internals where product-neutral language is enough.
4. Extract reusable Companion execution core from internkim app shell details.
5. Keep `internal/runtime/blueclaw` as internkim's appliance adapter for installing and supervising Blueclaw.
