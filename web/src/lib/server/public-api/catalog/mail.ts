import { z } from 'zod';

import {
  CapabilityAnsweredBy,
  CapabilityEstimatedLatency,
  CapabilityModelVisibility,
  CapabilitySideEffect,
} from './protocol';

import type { CapabilityToolDefinition } from './definition';

const mailConnectionStartInputSchema = z.strictObject({});

const mailConnectionStartInputIntentSchema = z.strictObject({});

const mailConnectionStatusInputSchema = z.strictObject({});

const mailMessageListInputSchema = z.strictObject({
  cursor: z.string().describe("Pagination cursor from a previous list response. Omit on the first call.").optional(),
  limit: z.int().describe("Maximum number of messages to return. Defaults to 20.").optional(),
  mailbox: z.string().describe("IMAP mailbox folder to list, e.g. 'INBOX', 'Sent', 'Archive'. Defaults to INBOX when omitted.").optional(),
});

const mailMessageMarkInputSchema = z.strictObject({
  flagged: z.boolean().describe("Set to true to star/flag the message, false to remove the flag. Omit to leave the flagged state unchanged.").optional(),
  mailbox: z.string().describe("IMAP mailbox folder of the message, e.g. 'INBOX'. Must come from a prior list or search result."),
  seen: z.boolean().describe("Set to true to mark as read, false to mark as unread. Omit to leave the read state unchanged.").optional(),
  uid: z.string().describe("IMAP UID of the message to mark. Must come from a prior list or search result — never invent a UID."),
});

const mailMessageMoveInputSchema = z.strictObject({
  mailbox: z.string().describe("Current IMAP mailbox folder of the message, e.g. 'INBOX'. Must come from a prior list or search result."),
  targetMailbox: z.string().describe("Destination IMAP folder, e.g. 'Archive', 'Trash', 'Work/Projects'. The folder must already exist."),
  uid: z.string().describe("IMAP UID of the message to move. Must come from a prior list or search result — never invent a UID."),
});

const mailMessageReadInputSchema = z.strictObject({
  mailbox: z.string().describe("IMAP mailbox folder the message is in, e.g. 'INBOX'. Must come from a prior list or search result."),
  uid: z.string().describe("IMAP UID of the message to read. Must come from a prior mail_message_list or mail_message_search result — never invent a UID."),
});

const mailMessageSearchInputSchema = z.strictObject({
  cursor: z.string().describe("Pagination cursor from a previous search response. Omit on the first call.").optional(),
  limit: z.int().describe("Maximum number of matching messages to return. Defaults to 20.").optional(),
  mailbox: z.string().describe("IMAP mailbox folder to search, e.g. 'INBOX'. Defaults to all folders when omitted.").optional(),
  query: z.string().describe("Search terms matched against subject, sender, and body, e.g. 'invoice Q2 2026'. Required."),
});

const mailMessageSendInputSchema = z.strictObject({
  bcc: z.array(z.string()).describe("BCC recipient email addresses. Recipients cannot see each other.").optional(),
  body: z.string().describe("Email body text. Plain text or HTML."),
  cc: z.array(z.string()).describe("CC recipient email addresses.").optional(),
  subject: z.string().describe("Email subject line."),
  to: z.array(z.string()).describe("Primary recipient email addresses, e.g. [\"alice@example.com\"]. At least one required."),
});

const mailMessageSendInputIntentSchema = mailMessageSendInputSchema.partial();

const mailConnectionStartResultSchema = z.strictObject({
  status: z.literal("configuration_required"),
  provider: z.literal("manual"),
  setupURL: z.string().describe("Page where the requester enters their mail account. Share it with the requester."),
});

const mailConnectionStatusResultSchema = z.strictObject({
  email: z.string(),
  fromAddress: z.string(),
  displayName: z.string(),
  imapHost: z.string(),
  imapPort: z.int(),
  imapSecurity: z.string(),
  imapUsername: z.string(),
  smtpHost: z.string(),
  smtpPort: z.int(),
  smtpSecurity: z.string(),
  smtpUsername: z.string(),
  defaultMailbox: z.string(),
  sentMailbox: z.string(),
  isConfigured: z.boolean().describe("True when the account can read and send mail. When false, offer mail_connection_start."),
  hasIMAPPassword: z.boolean(),
  hasSMTPPassword: z.boolean(),
});

const mailMessageUIDSchema = z.int().describe("IMAP UID of the message. Pass it as a string, with its mailbox, to mail_message_read.");

const mailMessageSummarySchema = z.strictObject({
  uid: mailMessageUIDSchema,
  mailbox: z.string(),
  subject: z.string(),
  from: z.string(),
  date: z.string(),
  preview: z.string(),
  isRead: z.boolean(),
});

const mailMessageListResultSchema = z.strictObject({
  messages: z.array(mailMessageSummarySchema),
  nextCursor: z.string().describe("Cursor for the next page. Empty when there are no more messages."),
  uidNext: z.int().optional(),
  uidValidity: z.int().optional(),
});

const mailMessageReadResultSchema = z.strictObject({
  uid: mailMessageUIDSchema,
  mailbox: z.string(),
  subject: z.string(),
  from: z.string(),
  to: z.string(),
  cc: z.string(),
  date: z.string(),
  body: z.string().describe("Plain-text body."),
  bodyHTML: z.string().describe("HTML body, present only when the message has one.").optional(),
  isRead: z.boolean(),
});

const mailMessageSendResultSchema = z.strictObject({
  sent: z.literal(true),
  appendedTo: z.string().describe("Mailbox the sent copy was saved to.").optional(),
  appendWarning: z.string().describe("Why the sent copy could not be saved. The message itself was still sent.").optional(),
});

export const mailToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: "mail_connection_start",
    namespace: "mail",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_mail",
    policyResource: "tool:mail_connection_start",
    description: "Start the email account connection flow and return setup instructions for the requester. Requires the user to be present (RequiresUserPresence=true). Only call this when mail_connection_status reports mail is not connected.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Interactive,
    inputSchema: mailConnectionStartInputSchema,
    inputIntentSchema: mailConnectionStartInputIntentSchema,
    result: { schema: mailConnectionStartResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Connect,
    requiresUserPresence: true,
    requiresApproval: true,
  },
  {
    name: "mail_connection_status",
    namespace: "mail",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_mail",
    policyResource: "tool:mail_connection_status",
    description: "Check whether the requester's email account is connected. Call this before any mail read or write operation when you are unsure if mail is set up.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: mailConnectionStatusInputSchema,
    result: { schema: mailConnectionStatusResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: "mail_message_list",
    namespace: "mail",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_mail",
    policyResource: "tool:mail_message_list",
    description: "List emails in a mailbox folder with optional pagination. Use this to browse recent messages; use mail_message_search when you need to find by keyword or subject.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: mailMessageListInputSchema,
    result: { schema: mailMessageListResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: "mail_message_mark",
    namespace: "mail",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_mail",
    policyResource: "tool:mail_message_mark",
    description: "Set the read (seen) or starred (flagged) status of an email. The UID and mailbox must come from a prior list or search result. Omit a flag field to leave it unchanged.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: mailMessageMarkInputSchema,
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
  },
  {
    name: "mail_message_move",
    namespace: "mail",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_mail",
    policyResource: "tool:mail_message_move",
    description: "Move an email to a different mailbox folder (e.g. Archive, Trash). The UID and mailbox must come from a prior list or search result. Use this to archive or sort messages.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: mailMessageMoveInputSchema,
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
  },
  {
    name: "mail_message_read",
    namespace: "mail",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_mail",
    policyResource: "tool:mail_message_read",
    description: "Fetch the full content of a single email by its mailbox name and UID. The UID must come from a prior mail_message_list or mail_message_search result — never invent a UID.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: mailMessageReadInputSchema,
    result: { schema: mailMessageReadResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: "mail_message_search",
    namespace: "mail",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_mail",
    policyResource: "tool:mail_message_search",
    description: "Search emails by keyword, sender, or subject. Returns matching messages with their UIDs for use with mail.message.read. Do not put pagination cursors in the query field.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: mailMessageSearchInputSchema,
    result: { schema: mailMessageListResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: "mail_message_send",
    namespace: "mail",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_mail",
    policyResource: "tool:mail_message_send",
    description: "Send an email from the requester's connected mail account. Provide at least one recipient in 'to', a subject, and a body. Requires approval before sending.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: mailMessageSendInputSchema,
    inputIntentSchema: mailMessageSendInputIntentSchema,
    result: { schema: mailMessageSendResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.ExternalSend,
    requiresApproval: true,
    completionEvidence: { mode: "success", action: "send_email", targetKind: "email" },
    idempotency: { supported: true, required: false, scope: "operation" },
  },
];
