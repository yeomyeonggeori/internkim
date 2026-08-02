# Blueclaw OSS Boundary

## Goal

Blueclaw should be releasable as an open-source agent runtime without carrying InternKim appliance code, product integrations, or user-local secrets. The boundary is capability-based: Blueclaw decides what capability it needs, while InternKim decides where and how that capability runs.

## Layers

| Layer | Ownership | Public boundary |
|---|---|---|
| Blueclaw runtime | Blueclaw OSS | Conversation runtime, policy, task state, memory, scheduler, skill selection |
| Capability protocol | Shared contract | Versioned JSON request, response, descriptor, routing, denial, and resource scope types |
| Companion runtime | Reusable provider core | User-local browser, file picking, confirmation, input, and local model execution |
| InternKim product | Private appliance layer | Device provisioning, Cloudflare, Mattermost, Slack, Signal, Google, OpenRouter, admin UI, fleet, packaging |

## Blueclaw OSS Should Keep

- Capability client code that calls a configured provider endpoint.
- Provider-neutral tool names and descriptor handling.
- Policy decisions about which actor may request which tool.
- Task, memory, scheduler, and skill orchestration.
- Runtime configuration fields that describe the capability endpoint, not provider implementation details.

## Blueclaw OSS Should Not Keep

- Provider tokens, browser cookies, local file paths, local model paths, or device secrets.
- InternKim-specific URLs, release downloads, command names, service names, or filesystem paths.
- Cloudflare, Mattermost, Slack, Signal, Google Workspace, OpenRouter, Jetson, Firecracker provisioning, or fleet code.
- Companion pairing broker state or signed upload storage.
- Product-specific capability catalogs such as `flow.*`, `site.app.*`, and Google bridge tools.

## Capability Protocol

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

InternKim-specific capability catalogs stay in `internal/capabilities`.

## Companion Boundary

Companion is a trusted user-local capability provider. Its reusable core can advertise and execute browser, file, user input, and local model capabilities.

The InternKim companion app remains a product shell around that core. It may include InternKim branding, pairing links, release packaging, tray behavior, and device runtime controls. Product controls should use product-owned device endpoints, not companion broker endpoints.

## Runtime Settings Boundary

Device runtime settings belong to the InternKim appliance layer. A paired Companion may authenticate a request to change a runtime setting, but the endpoint should not live under the companion broker namespace.

Current rule:

- Companion broker: `/_internkim/companion/*`
- Device runtime settings: `/_internkim/runtime/*`

The remote model setting uses `/_internkim/runtime/remote-model` because it writes the Blueclaw runtime configuration and restarts the Blueclaw service.

## Migration Order

1. Keep `internal/capabilities` as the product catalog wrapper and move shared protocol types to `pkg/capabilityprotocol`.
2. Move device runtime settings out of `/_internkim/companion/*`.
3. Neutralize Companion UI copy that exposes Blueclaw internals where product-neutral language is enough.
4. Extract reusable Companion execution core from InternKim app shell details.
5. Keep `internal/runtime/blueclaw` as InternKim's appliance adapter for installing and supervising Blueclaw.
