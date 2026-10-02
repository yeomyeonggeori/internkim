# Security policy

## Reporting a vulnerability

Report it privately through GitHub: open the repository's "Security" tab and
choose "Report a vulnerability", or go to
<https://github.com/yeomyeonggeori/internkim/security/advisories/new>.
Do not open a public issue, pull request or discussion for it.

Say what you found, the steps that reproduce it, and what an attacker gains.
A report with a working reproduction is triaged first. You will get an
acknowledgment, and a fix is coordinated with you before anything is disclosed.

## In scope

- Reading or writing another member's or company's data through the company
  web app, the public API (`/api/v1`) or the Supabase schema, including a row
  level security policy that admits a caller it should refuse.
- Acting as someone else: session or `ik_` token handling, the Linux identity
  a tool runs as, the workspace permission boundary.
- Credential exposure: a secret reaching a log, a browser, a workspace file or
  a published page.
- The relay and admind accepting a caller they should refuse.

## Out of scope

- Findings that need a machine or account the attacker already controls.
- Denial of service by volume against a company's own hardware.
- Issues in a dependency with no path to this product. Report those upstream.
- Model output that is wrong or unwise but crosses no permission boundary.

[Boundaries](https://docs.intern.kim/boundaries) describes what the system relies on.
