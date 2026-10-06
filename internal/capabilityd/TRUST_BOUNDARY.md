# capabilityd Trust Boundary

`capabilityd` serves the host Unix socket at `/run/internkim/capability.sock`.
The daemon creates the socket with mode `0660` and assigns the configured group,
which defaults to `blueclaw`.

Blueclaw is the trusted caller for `ToolInvokeContext` fields received over this
socket. The socket path itself is not a workspace path.

Requester task code runs as projected `bc_person_*` identities through the
Blueclaw POSIX helper. Those identities use a requester primary group plus
`bc_shared` and non-admin circle groups. They are not members of the `blueclaw`
service group and should not be able to open the host capability socket
directly.

`RequesterPersonID`, `IsScheduledRun`, `HoldID`, and `ScheduledApprovedCall` are
therefore trusted at the Blueclaw runtime boundary, not at the model/tool-code
boundary. `capabilityd` still rejects malformed requester IDs at decode time,
and requires a requester ID when the scheduled-run flag is present, as defense in
depth.
