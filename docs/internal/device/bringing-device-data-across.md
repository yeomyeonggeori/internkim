# Bringing device data across

- The carry is a recovery action, not a script. `internkim recover -action
  calendar-carry-into-the-record`, `attendance-carry-into-the-record` and
  `task-carry-into-the-record` each ask the device what the record has not
  taken, write it as the person it belongs to, and report what was refused.
  Run the matching `*-record-coverage` action first to see the count without
  writing anything. The read-and-import scripts they replaced pulled from
  endpoints that no longer exist.
- Import history as it happened. When the record refuses a row the past
  violated, report it instead of reshaping it into something the device never
  recorded. When the record would *accept* a row by quietly rewriting it — a
  completed task ending in the future is restamped with today — refuse it
  yourself and name it, because a silent rewrite reads as a successful import.
- Work whose every named person belongs to no member of the company is somebody
  else's; skip it instead of adopting it. Never filter by a person's name.
