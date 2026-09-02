import { z } from 'zod';

import {
  CapabilityAnsweredBy,
  CapabilityEstimatedLatency,
  CapabilityModelVisibility,
  CapabilitySideEffect,
} from './protocol';

import type { CapabilityToolDefinition } from './definition';

const googleCalendarEventInputSchema = z.strictObject({
  attendees: z.array(z.string()).describe("Attendee email addresses to invite, e.g. [\"alice@example.com\", \"bob@example.com\"].").optional(),
  description: z.string().describe("Event notes or agenda visible to all attendees.").optional(),
  end: z.string().describe("Event end time as ISO 8601 with timezone, e.g. 2026-06-23T15:00:00+09:00. Must be after start."),
  location: z.string().describe("Physical or virtual location, e.g. 'Room 3B' or 'https://meet.google.com/xyz'.").optional(),
  start: z.string().describe("Event start time as ISO 8601 with timezone, e.g. 2026-06-23T14:00:00+09:00. Resolve relative times before calling."),
  title: z.string().describe("Event title shown in Google Calendar, e.g. 'Design Review'."),
});

const googleDocsCreateInputSchema = z.strictObject({
  body: z.string().describe("Initial document body as plain text or Markdown. Omit to create an empty document.").optional(),
  title: z.string().describe("Title of the new Google Doc, e.g. 'Q3 OKR Review'."),
});

const googleDriveImportPptxInputSchema = z.strictObject({
  path: z.string().describe("Absolute workspace path to the .pptx file to import, e.g. /workspace/shared/presentation.pptx."),
  title: z.string().describe("Title for the Google Slides presentation. Defaults to the filename without extension.").optional(),
});

const googleEventListInputSchema = z.strictObject({
  end: z.string().describe("End of the listing window as ISO 8601 with timezone. Pair with start to bound the window.").optional(),
  limit: z.int().describe("Maximum number of events to return. Defaults to 20.").optional(),
  query: z.string().describe("Free-text filter matched against event titles. Do not put dates here — use start/end instead.").optional(),
  start: z.string().describe("Start of the listing window as ISO 8601 with timezone, e.g. 2026-06-23T00:00:00+09:00. Omit to default to the current time.").optional(),
});

const googleGmailSendInputSchema = z.strictObject({
  bcc: z.array(z.string()).describe("BCC recipient email addresses. Recipients cannot see each other.").optional(),
  body: z.string().describe("Email body text. Plain text or HTML."),
  cc: z.array(z.string()).describe("CC recipient email addresses.").optional(),
  subject: z.string().describe("Email subject line."),
  to: z.array(z.string()).describe("Primary recipient email addresses, e.g. [\"alice@example.com\"]. At least one required."),
});

const googleSheetsCreateInputSchema = z.strictObject({
  sheets: z.array(z.string()).describe("Names of sheets (tabs) to create, e.g. [\"January\", \"February\"]. Omit to create a single default sheet.").optional(),
  title: z.string().describe("Title of the new Google Spreadsheet, e.g. 'Budget Tracker 2026'."),
  values: z.array(z.array(z.string())).describe("Initial cell values as a 2D array of strings (rows × columns), e.g. [[\"Name\",\"Amount\"],[\"Alice\",\"500\"]]. Written to the first sheet.").optional(),
});

export const googleWorkspaceToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: "google_calendar_event",
    namespace: "google",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_google",
    policyResource: "tool:google_calendar_event",
    description: "Create a new Google Calendar event. Provide title, start, and end as ISO 8601 timestamps. Add attendees as email addresses. Returns a link to the created event.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: googleCalendarEventInputSchema,
    sideEffect: CapabilitySideEffect.ExternalWrite,
    completionEvidence: { mode: "success", action: "write_calendar", targetKind: "calendar" },
  },
  {
    name: "google_docs_create",
    namespace: "google",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_google",
    policyResource: "tool:google_docs_create",
    description: "Create a new Google Doc in the requester's Google Drive. Provide a title and optionally the initial body text (plain text or Markdown). Returns a link to the created document.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: googleDocsCreateInputSchema,
    sideEffect: CapabilitySideEffect.ExternalWrite,
  },
  {
    name: "google_drive_import_pptx",
    namespace: "google",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_google",
    policyResource: "tool:google_drive_import_pptx",
    description: "Import a PPTX file from the workspace into Google Drive as a native Google Slides presentation. Provide the workspace path to the .pptx file. Returns a link to the created presentation.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: googleDriveImportPptxInputSchema,
    sideEffect: CapabilitySideEffect.ExternalWrite,
  },
  {
    name: "google_event_list",
    namespace: "google",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_google",
    policyResource: "tool:google_event_list",
    description: "List events from the requester's Google Calendar within an optional time window. Use start and end (ISO 8601) to bound the window; use query to filter by event title keyword.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: googleEventListInputSchema,
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: "google_gmail_send",
    namespace: "google",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_google",
    policyResource: "tool:google_gmail_send",
    description: "Send an email via the requester's connected Gmail account. Provide at least one recipient in 'to', a subject, and a body. Requires approval before sending.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: googleGmailSendInputSchema,
    sideEffect: CapabilitySideEffect.ExternalSend,
    requiresApproval: true,
    completionEvidence: { mode: "success", action: "send_email", targetKind: "email" },
    idempotency: { supported: true, required: false, scope: "operation" },
  },
  {
    name: "google_sheets_create",
    namespace: "google",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_google",
    policyResource: "tool:google_sheets_create",
    description: "Create a new Google Spreadsheet in the requester's Google Drive. Optionally provide sheet names and initial cell values. Returns a link to the created spreadsheet.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: googleSheetsCreateInputSchema,
    sideEffect: CapabilitySideEffect.ExternalWrite,
  },
];
