# Bringing device data across

- Pull a device's data over **HTTP**, not SSH: sign into its own web app and
  read the endpoints it already serves (`/task/api/state`,
  `/attendance/api/summary`, `/calendar/api/events`), then feed the JSON to
  `web/scripts/import-flow-state.ts` and `import-attendance-events.ts`. Short
  requests survive a flapping uplink; an SSH session does not.
- Import history as it happened. When the record refuses a row the past
  violated, report it instead of reshaping it into something the device never
  recorded, and keep the export so the decision stays reversible.
- Work whose every named person belongs to no member of the company is somebody
  else's; skip it instead of adopting it. Never filter by a person's name.

