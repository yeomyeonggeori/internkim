export const capabilityNamespaceSummaries = {
  artifact: 'Review rendered output against what it was meant to show.',
  attendance: 'Clock-ins and clock-outs, and the working hours and leave policy they are judged against.',
  browser: 'Operate a web page step by step: open it, read it, click, fill, and screenshot.',
  calendar: 'The company calendar: events, and the approved leave that falls in a time window.',
  company: "The company's own record: its profile and settings, holidays, metrics over time, milestones and assets, and a data room of registered documents searchable by question.",
  computer: "Carry out a goal on the requester's own computer.",
  crm: 'Organizations and contacts the company deals with, deals moving through a pipeline, and the work recorded against them.',
  document: 'Read a document in the workspace as text.',
  image: 'Look at an image, or generate a new one.',
  leave: 'Leave requests and balances: file, correct, approve, and set yearly entitlements.',
  mail: "The requester's email: connect an account, then read, search, file, and send.",
  message: 'Messages in the company messenger: read a conversation, search, send, edit, and delete.',
  notification: 'What the requester is told about, and which conversations are muted.',
  person: 'The company directory: who works here, what it holds about them, and inviting someone new.',
  schedule: 'Reminders and recurring work done later, once or on a repeat.',
  site: 'Publish a website built in the workspace, preview it, or take it down.',
  task: "The team's work items: add, find, update, and who takes part.",
  team: "The organization chart's teams: create, rename, move, and remove.",
  web: 'Search the public web and read its pages.',
} as const;

export type CapabilityNamespace = keyof typeof capabilityNamespaceSummaries;
