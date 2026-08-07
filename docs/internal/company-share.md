# Shared company page

The shared company page is available at `/company/` to anyone who knows its password. It is not tied to a staff identity or described as an investor-only page.

## Source of truth

The publication configuration is stored at `/workspace/.protected/company-share.json`. The directory is service-owned with mode `0755` and the file has mode `0644`, so task actors can read the configuration but cannot replace or delete it. The password hash and access-session version remain in admind private state with mode `0600` and never appear in the workspace JSON.

English is always the base language. Additional BCP 47 language tags such as `ja`, `ko`, or `pt-BR` are optional. Localized narratives, metric labels, and milestone copy use those language tags as JSON keys.

## API

An administrator or an authorized built-in tool can read and replace the configuration through:

- `GET /admin/api/company-share`
- `PUT /admin/api/company-share`
- `POST /admin/api/company-share/publish`

There is no delete endpoint. `PUT` accepts one JSON object up to 1 MiB, rejects unknown fields and trailing JSON, validates language tags and field limits, and preserves the current password when `password` is empty. A non-empty password must contain at least eight characters and rotates all existing access sessions.

The published snapshot is deliberately separate from the editable configuration. Publishing copies only the selected company fields, metrics, records, documents, and aggregated team activity into the public projection.
