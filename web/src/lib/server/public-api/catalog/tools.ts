import { attendanceChangeReasons } from '$lib/attendance/change-reason';
import { z } from 'zod';
import { savedAttendanceEventSchema } from '$lib/attendance/recorded-attendance';
import { currentAttendanceSchema } from '$lib/attendance/current-attendance';
import { attendanceTeamPageInputSchema, attendanceTeamPageSchema } from '$lib/attendance/team-page';
import { taskSizes } from '$lib/task/task-sizes';

import {
  CapabilityAnsweredBy,
  CapabilityEstimatedLatency,
  CapabilityModelVisibility,
  CapabilitySideEffect,
  ResourceEffectIdentity,
} from './protocol';

import { browserControlToolDefinitions } from './browser';
import { companyToolDefinitions } from './company';
import { dataRoomToolDefinitions } from './data-room';
import { circleToolDefinitions } from './circle';
import { hostToolDefinitions } from './host';
import { crmToolDefinitions } from './crm';
import {
  taskLabelGetInputSchema,
  taskLabelGetResultSchema,
  taskLabelGetToolName,
  taskLabelVocabularySchema,
  taskVocabularySetInputIntentSchema,
  taskVocabularySetInputSchema,
} from './labels';
import {
  buildCapabilityCatalog,
  ResourceMutationEffect,
  type CapabilityToolCatalog,
  type CapabilityToolDefinition,
} from './definition';
import { mailToolDefinitions } from './mail';
import { notificationToolDefinitions } from './notifications';
import { peopleToolDefinitions } from './people';
import { settingsToolDefinitions } from './settings';
import {
  WorkspaceTaskInitialStatus,
  WorkspaceTaskScope,
  WorkspaceTaskSize,
  WorkspaceTaskStatus,
} from './workspace-task';

export {
  WorkspaceTaskInitialStatus,
  WorkspaceTaskScope,
  WorkspaceTaskSize,
  WorkspaceTaskStatus,
};

const dateDescription = 'Date in YYYY-MM-DD format.';
// Whose records to read is one question with three answers, and the commonest
// one costs nothing to ask for. A company-wide read is a constant rather than a
// roll call of every member.
function personHintsDescription(records: string): string {
  return `Names, @handles, or emails of the people whose ${records} to read. Omit it for the requester's own, which is the usual case. Do not send it together with scope.`;
}

function whoseScopeDescription(records: string): string {
  return `Set to all to read the whole company's ${records}. Omit it for the requester's own; name people in personHints instead of listing everyone. Sending both is refused.`;
}

const momentDescription = 'ISO 8601 with timezone for a moment, or YYYY-MM-DD for a whole day.';
const resourceIDSchema = z.string()
  .min(1)
  .regex(/^\S(?:.*\S)?$/, 'Resource identity must not have leading or trailing whitespace.');

export enum CalendarToolName {
  Add = 'event_add',
  List = 'event_list',
  Update = 'event_update',
  Delete = 'event_delete',
}

export enum CalendarEntrySource {
  Event = 'event',
  Leave = 'leave',
}

export enum MessageToolName {
  Context = 'message_context',
  Search = 'message_search',
  Send = 'message_send',
  Update = 'message_update',
  Delete = 'message_delete',
}

export enum MessageTargetType {
  DirectMessage = 'directMessage',
  CurrentThread = 'currentThread',
  CurrentChannel = 'currentChannel',
  Channel = 'channel',
}

export enum MessageSearchScope {
  CurrentThread = 'currentThread',
  CurrentChannel = 'currentChannel',
  DirectMessage = 'directMessage',
  Channel = 'channel',
}

export enum MessageAuthor {
  Assistant = 'assistant',
  Requester = 'requester',
  Anyone = 'anyone',
}

export enum MessageDeliveryStatus {
  Sent = 'sent',
  Updated = 'updated',
  Deleted = 'deleted',
}

export enum DocumentToolName {
  Read = 'document_read',
}

export enum ImageToolName {
  Read = 'image_read',
}

export enum BrowserToolName {
  Open = 'browser_open',
  Snapshot = 'browser_snapshot',
  Screenshot = 'browser_screenshot',
  Click = 'browser_click',
}

export enum ArtifactToolName {
  Review = 'artifact_review',
}

export enum WebToolName {
  Search = 'web_search',
  Fetch = 'web_fetch',
}

export enum ArtifactKind {
  Slides = 'slides',
  PowerPoint = 'pptx',
  Word = 'docx',
  PDF = 'pdf',
}

export enum ArtifactIssueSeverity {
  Blocking = 'blocking',
  Warning = 'warning',
  Information = 'info',
}

export enum ArtifactIssueCategory {
  TextFit = 'textFit',
  Layout = 'layout',
  VisualHierarchy = 'visualHierarchy',
  ContentDensity = 'contentDensity',
  TemplateSmell = 'templateSmell',
  Responsiveness = 'responsiveness',
  RenderFidelity = 'renderFidelity',
}

export enum ArtifactEvidenceMimeType {
  PNG = 'image/png',
  JPEG = 'image/jpeg',
}

export enum ScheduleToolName {
  List = 'schedule_list',
  Create = 'schedule_create',
  Update = 'schedule_update',
  Cancel = 'schedule_cancel',
}

const scheduleListInputSchema = z.strictObject({
  status: z.enum(['active', 'failed', 'expired']).optional(),
  limit: z.int().min(1).optional(),
});

const scheduleMomentDescription = 'Written as an RFC3339 date-time carrying an offset, such as 2026-09-20T09:00:00+09:00.';

const scheduleKindDescription = 'once runs a single time at runAt, interval runs every intervalSecond, cron runs on cronExpression.';

const scheduleTaskInstructionDescription = 'What the agent should do each time the schedule fires, written so it can be acted on without the conversation it was asked in.';

const scheduleDescriptionSchema = z.string().describe('Short name the schedule is listed under. Omit to name it after the instruction.').optional();

const scheduleCadenceFields = {
  runAt: z.string().describe(`When a once schedule runs. ${scheduleMomentDescription}`).optional(),
  expiresAt: z.string().describe(`When the schedule stops running. ${scheduleMomentDescription} It must be in the future.`).optional(),
  intervalSecond: z.int().min(1).describe('Seconds between the runs of an interval schedule, at least 1.').optional(),
  cronExpression: z.string().describe('Five-field cron expression for a cron schedule, read in timeZone.').optional(),
  timeZone: z.string().min(1).regex(/\S/).describe('IANA time zone the schedule is read in, such as Asia/Seoul. Omit for the company time zone.').optional(),
  maxRunCount: z.int().min(1).describe('How many times a finite schedule runs before it stops, at least 1.').optional(),
  repeatPolicy: z.enum(['finite', 'unbounded']).describe('Required for interval and cron: finite stops at expiresAt or maxRunCount, unbounded keeps running.').optional(),
};

const scheduleHintSchema = z.string().min(1).max(256).describe(
  'Identifies the schedule to act on: its exact schedule ID, or the exact CURRENT description of one of your own schedules as schedule_list shows it. Never a new or intended description. Resolved server-side; if it does not uniquely resolve, the call fails with a candidates list to retry against.',
);

export const scheduleCreateInputSchema = z.strictObject({
  taskInstruction: z.string().min(1).regex(/\S/).describe(scheduleTaskInstructionDescription),
  description: scheduleDescriptionSchema,
  kind: z.enum(['once', 'interval', 'cron']).describe(scheduleKindDescription),
  ...scheduleCadenceFields,
});

const scheduleCreateInputIntentSchema = scheduleCreateInputSchema.partial();

const scheduleUpdateObjectSchema = z.strictObject({
  scheduleHint: scheduleHintSchema,
  taskInstruction: z.string().min(1).regex(/\S/).describe(`New instruction. ${scheduleTaskInstructionDescription}`).optional(),
  description: scheduleDescriptionSchema,
  kind: z.enum(['once', 'interval', 'cron']).describe(`New cadence. ${scheduleKindDescription}`).optional(),
  ...scheduleCadenceFields,
});

const scheduleMutableFieldNames = Object.keys(scheduleUpdateObjectSchema.shape).filter(name => name !== 'scheduleHint');

function hasScheduleMutation(document: object): boolean {
  return scheduleMutableFieldNames.some(name => Object.hasOwn(document, name));
}

export const scheduleUpdateInputSchema = scheduleUpdateObjectSchema
  .refine(hasScheduleMutation, 'At least one schedule field must be updated.')
  .meta({ minProperties: 2 });

export const scheduleUpdateInputIntentSchema = scheduleUpdateObjectSchema.omit({ scheduleHint: true });

export const scheduleCancelInputSchema = z.strictObject({
  scheduleHints: z.array(z.string().min(1).max(256))
    .min(1)
    .meta({ uniqueItems: true })
    .describe('The schedules to cancel. Each one is an exact schedule ID, or the exact CURRENT description of one of your own schedules as schedule_list shows it, never a new or intended description. Every hint resolves before anything is cancelled, and a hint that does not uniquely resolve fails the call with a candidates list to retry against.'),
});

export const scheduleCancelInputIntentSchema = z.strictObject({});

export const scheduleMutationResultSchema = z.strictObject({
  scheduleID: z.string().min(1),
  description: z.string(),
  taskInstruction: z.string().min(1).regex(/\S/),
  timeZone: z.string().min(1).regex(/\S/),
  kind: z.enum(['once', 'interval', 'cron']),
  runAt: z.string().meta({ format: 'date-time' }).optional(),
  intervalSecond: z.int().min(1).optional(),
  cronExpression: z.string().optional(),
  maxRunCount: z.int().min(1).optional(),
  expiresAt: z.string().meta({ format: 'date-time' }).optional(),
  nextRunAt: z.string().meta({ format: 'date-time' }),
  conversationID: z.string(),
  replyTargetID: z.string(),
  agentProfileName: z.string(),
});

export const scheduleCancelResultSchema = z.strictObject({
  scheduleIDs: z.array(z.string().min(1))
    .min(1)
    .refine(identities => new Set(identities).size === identities.length, 'schedule identities are unique')
    .meta({ uniqueItems: true })
    .describe('The schedules that were cancelled.'),
  cancelled: z.array(z.strictObject({
    scheduleID: z.string().min(1),
    description: z.string(),
  })),
});

const scheduleListItemSchema = z.strictObject({
  scheduleID: z.string().min(1),
  taskInstruction: z.string().min(1).regex(/\S/),
  description: z.string().optional(),
  cadence: z.string(),
  cronExpression: z.string().optional(),
  runAt: z.string().meta({ format: 'date-time' }).optional(),
  status: z.enum(['active', 'failed', 'expired']),
  nextRunAt: z.string().meta({ format: 'date-time' }).optional(),
  lastRunAt: z.string().meta({ format: 'date-time' }).optional(),
});

const scheduleListResultSchema = z.strictObject({
  schedules: z.array(scheduleListItemSchema),
});

const scheduleToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: ScheduleToolName.List,
    namespace: 'schedule',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'workspace_schedule',
    policyResource: 'tool:schedule_list',
    description: 'List scheduled tasks created by the current requester. Filter by active, failed, or expired status and cap the result with limit. Use it to answer what reminders or recurring tasks are scheduled.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: scheduleListInputSchema,
    result: { schema: scheduleListResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: ScheduleToolName.Create,
    namespace: 'schedule',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'workspace_schedule',
    policyResource: 'tool:schedule_create',
    description: 'Schedule a reminder or a recurring alert: work the agent does later, once at a time you name or on a repeat. It arrives in the conversation it was asked in, so it can only be created from one. Use schedule_update to change one that already exists.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: scheduleCreateInputSchema,
    inputIntentSchema: scheduleCreateInputIntentSchema,
    result: {
      schema: scheduleMutationResultSchema,
      effects: [{
        objectType: 'schedule',
        effect: ResourceMutationEffect.Created,
        resultField: 'scheduleID',
        effectIdentity: ResourceEffectIdentity.ID,
      }],
    },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: 'success', action: 'write_schedule', targetKind: 'schedule' },
  },
  {
    name: ScheduleToolName.Update,
    namespace: 'schedule',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'workspace_schedule',
    policyResource: 'tool:schedule_update',
    description: 'Change explicit fields on a reminder or recurring alert the requester set up, including how often it repeats and when it next arrives. Use schedule_list first when neither the schedule ID nor its current description is known. At least one field to change is required.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: scheduleUpdateInputSchema,
    inputIntentSchema: scheduleUpdateInputIntentSchema,
    result: {
      schema: scheduleMutationResultSchema,
      effects: [{
        objectType: 'schedule',
        effect: ResourceMutationEffect.Updated,
        resultField: 'scheduleID',
        effectIdentity: ResourceEffectIdentity.ID,
      }],
    },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: 'success', action: 'write_schedule', targetKind: 'schedule' },
  },
  {
    name: ScheduleToolName.Cancel,
    namespace: 'schedule',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'workspace_schedule',
    policyResource: 'tool:schedule_cancel',
    description: 'Stop a reminder or recurring alert the requester set up, so it stops arriving. Use schedule_list first when neither the schedule IDs nor their current descriptions are known. A cancelled schedule never runs again; schedule_create makes a new one. Silencing a whole category of notice instead is notification_settings_set\'s.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: scheduleCancelInputSchema,
    inputIntentSchema: scheduleCancelInputIntentSchema,
    result: {
      schema: scheduleCancelResultSchema,
      effects: [{
        objectType: 'schedule',
        effect: ResourceMutationEffect.Deleted,
        resultField: 'scheduleIDs',
        effectIdentity: ResourceEffectIdentity.ID,
      }],
    },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: 'success', action: 'write_schedule', targetKind: 'schedule' },
  },
];

export enum CalendarReminderLeadHours {
  One = 1,
  Two = 2,
  Three = 3,
  Six = 6,
  Twelve = 12,
  TwentyFour = 24,
  FortyEight = 48,
}

const workspaceTaskSizeSchema = z.enum(WorkspaceTaskSize);

const taskSizeDescription = [
  'Choose a fixed effort size using the workspace rubric below and your judgment about the work. Estimate it even when the user gives no size; preserve an explicitly requested size. A deadline span is not effort. Calendar events use event_add, which calculates size from their duration.',
  ...taskSizes('en').map((size) => `${size.name}: ${size.developmentExample}; ${size.otherExample}; ${size.note}.`),
].join('\n');

const labelsTheRecordDecides = "Omit it otherwise: the record decides the business, type and size a new task leaves out, from the company's own definitions.";

const taskTypeDescription = 'Choose the matching company type from registeredLabels.types in task_list, using the meaning of the work. Read the current definitions when they are not already available. Send an empty string when no type fits; it is stored as null and displayed as Other in the user\'s language. Never invent a label.';
const workspaceTaskStatusSchema = z.enum(WorkspaceTaskStatus);

const taskParticipantSchema = z.strictObject({
  personID: z.string().optional(),
  displayName: z.string().optional(),
  email: z.string().optional(),
  mention: z.string().optional(),
});

const crmOrganizationHintDescription =
  'Names the organization this work is for, as a crm_organization_list result shows it, or its exact organization ID. Omit for work that is about nobody outside the company.';
const crmOpportunityHintDescription =
  'Names the deal this work belongs to, as a crm_opportunity_list result shows it, or its exact opportunity ID. The deal already says which organization it is with.';

export const taskResultSchema = z.strictObject({
  taskID: resourceIDSchema,
  parentTaskID: z.string().optional(),
  organizationID: z.string().optional(),
  opportunityID: z.string().optional(),
  requesterID: z.string().optional(),
  requesterName: z.string().optional(),
  createdAt: z.string().optional(),
  ownerID: z.string().optional(),
  ownerName: z.string().optional(),
  participantIDs: z.array(z.string()).optional(),
  participantNames: z.array(z.string()).optional(),
  participantPresentations: z.array(taskParticipantSchema).optional(),
  business: z.string().optional(),
  type: z.string().optional(),
  content: z.string().optional(),
  size: z.string().optional(),
  status: z.string().optional(),
  startDate: z.string().optional(),
  endDate: z.string().optional(),
  weekCode: z.string().optional(),
});

const parentTaskHintDescription =
  'The task this one belongs under: its exact task ID or its exact CURRENT title as it appears in a task_list result. Never a new or intended title. Omit for work that stands on its own.';

export const taskAddInputSchema = z.strictObject({
  title: z.string().describe(
    "Concise noun-phrase title for the work itself, in the user's language. Keep the user's exact title when they give one; otherwise write one rather than reusing their sentence. People belong in the person fields, not the title.",
  ),
  size: workspaceTaskSizeSchema
    .describe(`Only a size the user names. ${labelsTheRecordDecides}`)
    .optional(),
  status: z.enum(WorkspaceTaskInitialStatus)
    .describe('Initial task status. Defaults to planned. Work whose participants leave out the requester is added as requested, with the requester as the one who asked; only an administrator may add it in another status.')
    .optional(),
  business: z.string()
    .describe("The company business this work is for, from registeredLabels.businesses in a task_list result, whenever the conversation or the work itself shows which one. Omit it only when that is unknown: the record then decides it from the company's own definitions.")
    .optional(),
  type: z.string()
    .describe(`Only a type the user names. ${labelsTheRecordDecides}`)
    .optional(),
  startsAt: z.string().describe(`When the work starts. ${momentDescription} Resolve relative dates from the current date. Omit when the user did not specify one.`).optional(),
  endsAt: z.string().describe(`When the work is due. ${momentDescription} Resolve relative dates from the current date. Omit when the user did not specify one.`).optional(),
  participantPersonHints: z.array(z.string())
    .describe('Names, @handles, or emails of the people the task belongs to. Omit it when the work is the requester\u2019s own, which is the usual case; the name the requester used to address you is not a participant.')
    .optional(),
  parentTaskHint: z.string().max(256).describe(parentTaskHintDescription).optional(),
  organizationHint: z.string().max(256).describe(crmOrganizationHintDescription).optional(),
  opportunityHint: z.string().max(256).describe(crmOpportunityHintDescription).optional(),
});

export const taskAddInputIntentSchema = taskAddInputSchema.partial();

export const taskListInputSchema = z.strictObject({
  query: z.string()
    .describe("Free-text keyword filter matched against task titles and notes, e.g. 'budget'. Do not put dates, week codes, or person names here — use the dedicated fields instead.")
    .optional(),
  personHints: z.array(z.string()).describe(personHintsDescription('tasks')).optional(),
  scope: z.enum(WorkspaceTaskScope).describe(whoseScopeDescription('tasks')).optional(),
  weekFrom: z.number()
    .describe('Start of the week range as an offset from this week: 0 this week, -1 last week, 1 next week. Omit both weekFrom and weekTo to list the current week; widen the range for other periods.')
    .optional(),
  weekTo: z.number().describe('End of the week range as an offset from this week. Omit both weekFrom and weekTo to list the current week.').optional(),
  status: z.enum(WorkspaceTaskStatus)
    .describe('Filter by task status. Omit the field to return every status.')
    .optional(),
  organizationHint: z.string().max(256).describe(`Only the work for one organization. ${crmOrganizationHintDescription}`).optional(),
  opportunityHint: z.string().max(256).describe(`Only the work on one deal. ${crmOpportunityHintDescription}`).optional(),
  everyWeek: z.boolean()
    .describe('Every week there is, ignoring weekFrom and weekTo. Use it to search the whole history or to total work across all time.')
    .optional(),
  limit: z.number().describe('Maximum number of tasks to return. Omit to return every matching task.').optional(),
});

const taskHintSchema = z.string().min(1).max(256).describe(
  'Identifies the existing task to act on: its exact task ID or its exact CURRENT title as it appears in a task_list result. Never a new or intended title. Resolved server-side to the canonical task; if it does not uniquely resolve, the call fails with a candidates list of the matching tasks to retry against.',
);

const taskUpdateObjectSchema = z.strictObject({
  taskHint: taskHintSchema,
  title: z.string().describe('New task title.').optional(),
  status: workspaceTaskStatusSchema.describe('New task status.').optional(),
  size: workspaceTaskSizeSchema.describe(taskSizeDescription).optional(),
  business: z.string().describe('Business label, taken from registeredLabels.businesses in a task_list result.').optional(),
  type: z.string().describe(`${taskTypeDescription} Omit to preserve the current type; an empty string clears it.`).optional(),
  startsAt: z.string().describe(`When the work starts. ${momentDescription} An empty string takes the date off.`).optional(),
  endsAt: z.string().describe(`When the work is due. ${momentDescription} An empty string takes the date off.`).optional(),
  participantPersonHints: z.array(z.string())
    .describe('Names, @handles, or emails of everyone taking part, replacing the current participants. Send the whole set, not just additions.')
    .optional(),
  parentTaskHint: z.string().max(256).describe(`${parentTaskHintDescription} An empty string takes it out from under the task it was under.`).optional(),
  childTaskHints: z.array(z.string())
    .describe('Tasks to move under taskHint, each an exact task ID or exact CURRENT title from a task_list result. Every one of them must stand on its own already; the whole set moves or none of it does.')
    .optional(),
  organizationHint: z.string().max(256).describe(`${crmOrganizationHintDescription} An empty string takes the work off the organization it was for.`).optional(),
  opportunityHint: z.string().max(256).describe(`${crmOpportunityHintDescription} An empty string takes the work off the deal it belonged to.`).optional(),
});

export const taskUpdateInputSchema = taskUpdateObjectSchema
  .refine(hasMutationField, 'At least one task field must be updated.')
  .meta({ minProperties: 2 });

export const taskUpdateInputIntentSchema = taskUpdateObjectSchema.omit({ taskHint: true });

export const taskDeleteInputSchema = z.strictObject({
  taskHint: taskHintSchema,
});

export const taskDeleteInputIntentSchema = z.strictObject({});

export const taskListResultSchema = z.strictObject({
  tasks: z.array(taskResultSchema),
  count: z.number().int(),
  unfinishedCount: z.number().int().describe('Number of unfinished tasks among all matching rows before the optional limit is applied.'),
  scope: z.string(),
  weekFrom: z.number().int().optional(),
  weekTo: z.number().int().optional(),
  statusFilter: z.string().optional(),
  ownerID: z.string().optional(),
  registeredLabels: taskLabelVocabularySchema,
});

export const taskBoardInputSchema = taskListInputSchema.omit({ everyWeek: true, weekFrom: true, weekTo: true }).extend({
  boardWeek: z.string().regex(/^\d{4}-\d{2}-\d{2}$/)
    .describe('Monday of the displayed board week as YYYY-MM-DD. Includes carry-over work according to the board status rules.'),
});

export const taskBoardResultSchema = taskListResultSchema.extend({
  boardWeek: z.string(),
  childProgress: z.array(z.strictObject({
    parentTaskID: z.string(), completed: z.number().int(), total: z.number().int(), percent: z.number().int(),
  })),
});

export const taskDeleteResultSchema = z.strictObject({
  taskID: resourceIDSchema,
  deleted: z.literal(true),
});

const calendarParticipantResultSchema = z.strictObject({
  personID: z.string().optional(),
  name: z.string(),
  email: z.string().optional(),
});

export const calendarEventResultSchema = z.strictObject({
  eventID: resourceIDSchema,
  title: z.string(),
  note: z.string(),
  location: z.string(),
  startsAt: z.string(),
  endsAt: z.string(),
  isWholeDay: z.boolean(),
  participants: z.array(calendarParticipantResultSchema),
  notifyMinutesBefore: z.number().int().optional(),
  updatedAt: z.string(),
});

const calendarMutableFields = {
  title: z.string().describe('New event title.').optional(),
  note: z.string().describe('New event notes or agenda. Use an empty string to clear them.').optional(),
  location: z.string().describe('New physical or virtual location. Use an empty string to clear it.').optional(),
  startsAt: z.string().describe(`New event start. ${momentDescription}`).optional(),
  endsAt: z.string().describe(`New event end. ${momentDescription}`).optional(),
  isWholeDay: z.boolean().describe('Whether the event takes the whole day.').optional(),
  participantPersonHints: z.array(z.string())
    .describe('Names, @handles, or emails of everyone attending, replacing the current attendees. Send the whole set, not just additions.')
    .optional(),
  everyoneAttends: z.boolean()
    .describe('Set true when the event is open to the whole company, which leaves it with no attendee list. Leave it out when named people attend, or when nobody is named and the event is the requester\u2019s own.')
    .optional(),
  notifyMinutesBefore: z.number().int().min(0).describe('Minutes before the start to notify attendees. 0 leaves the event with no reminder.').optional(),
};

export const calendarAddInputSchema = z.strictObject({
  title: z.string().describe('Event title shown in the calendar.'),
  note: z.string().describe('Optional event notes or agenda visible to attendees.').optional(),
  location: z.string().describe('Optional physical or virtual location.').optional(),
  startsAt: z.string().describe(`Event start. ${momentDescription}`),
  endsAt: z.string().describe(`Event end. ${momentDescription} It must be after startsAt.`),
  isWholeDay: z.boolean().describe('Set true for an event that takes the whole day.').optional(),
  participantPersonHints: z.array(z.string())
    .describe('Names, @handles, or emails of the people attending. Naming nobody makes the event the requester\u2019s own.')
    .optional(),
  everyoneAttends: z.boolean()
    .describe('Set true when the event is open to the whole company, which leaves it with no attendee list. Leave it out when named people attend, or when nobody is named and the event is the requester\u2019s own.')
    .optional(),
  notifyMinutesBefore: z.number().int().min(0).describe('Minutes before the start to notify attendees. 0 leaves the event with no reminder.').optional(),
});

export const calendarAddInputIntentSchema = calendarAddInputSchema.partial();

const calendarMutableFieldNames = Object.keys(calendarMutableFields);

function hasCalendarMutation(document: object): boolean {
  return calendarMutableFieldNames.some(name => Object.hasOwn(document, name));
}

function calendarExpectedUpdatedAtSchema(action: 'update' | 'deletion') {
  return z.string()
    .describe(`The updatedAt the caller last read; the ${action} is refused when the event has changed since.`)
    .optional();
}

export const calendarListInputSchema = z.strictObject({
  startsAt: z.string().describe(`Inclusive start of the window. ${momentDescription}`).optional(),
  endsAt: z.string().describe(`Exclusive end of the window. ${momentDescription}`).optional(),
  weekFrom: z.number().int()
    .describe('Start of the week range as an offset from this week: 0 this week, -1 last week, 1 next week. Use instead of startsAt and endsAt when the user speaks in weeks.')
    .optional(),
  weekTo: z.number().int().describe('End of the week range as an offset from this week.').optional(),
  query: z.string().describe('Optional free-text filter matched against event titles, notes, and locations.').optional(),
  personHints: z.array(z.string()).describe("Only calendar entries whose participants include these exact person IDs, names, @handles, or emails. For a personal briefing, supply the requester's exact person ID or email. Omit to read the visible company calendar.").optional(),
  limit: z.number().positive().refine(Number.isInteger, 'Limit must be a whole number.').describe('Maximum number of events to return.').optional(),
});

const calendarEventHintSchema = z.string().min(1).max(256).describe(
  'Identifies the existing calendar event to act on: its exact event ID or its exact CURRENT title as it appears in a event_list result. Never a new or intended title. Resolved server-side; if it does not uniquely resolve, the call fails with a candidates list of matching events.',
);

const calendarUpdateObjectSchema = z.strictObject({
  eventHint: calendarEventHintSchema,
  expectedUpdatedAt: calendarExpectedUpdatedAtSchema('update'),
  ...calendarMutableFields,
});

export const calendarUpdateInputSchema = calendarUpdateObjectSchema
  .refine(hasCalendarMutation, 'At least one calendar event field must be updated.')
  .meta({ minProperties: 2 });

export const calendarUpdateInputIntentSchema = calendarUpdateObjectSchema.omit({
  eventHint: true,
  expectedUpdatedAt: true,
});

export const calendarDeleteInputSchema = z.strictObject({
  eventHint: calendarEventHintSchema,
  expectedUpdatedAt: calendarExpectedUpdatedAtSchema('deletion'),
});

export const calendarDeleteInputIntentSchema = z.strictObject({});

export const calendarEntryResultSchema = calendarEventResultSchema.extend({
  source: z.enum(CalendarEntrySource),
  readOnly: z.boolean(),
});

export const calendarListResultSchema = z.strictObject({
  events: z.array(calendarEntryResultSchema),
});

export const calendarDeleteResultSchema = z.strictObject({
  eventID: resourceIDSchema,
  deleted: z.literal(true),
});

const uniqueResourceIDArraySchema = z.array(resourceIDSchema)
  .min(1)
  .max(50)
  .refine(values => new Set(values).size === values.length, 'Resource identities must be unique.')
  .meta({ uniqueItems: true });

const uniqueMessageIDArraySchema = z.array(resourceIDSchema)
  .min(1)
  .max(25)
  .refine(values => new Set(values).size === values.length, 'Message identities must be unique.')
  .meta({ uniqueItems: true });

export const messageContextInputSchema = z.strictObject({});

export const messageSearchInputSchema = z.strictObject({
  messageIDs: uniqueMessageIDArraySchema.describe('Exact message IDs to read in full instead of searching. Returns each message\'s complete text rather than a preview. Use this before rewriting a long message.').optional(),
  scope: z.enum(MessageSearchScope)
    .describe('Where to search. Current conversation scopes use the channel or thread this request is running in.')
    .optional(),
  channelName: z.string().describe('Exact channel name as the messenger shows it.').optional(),
  channelID: resourceIDSchema.describe('Exact channel ID from message_context or a prior result.').optional(),
  personHint: z.string().describe('Exact name, @handle, or email of the direct-message counterpart.').optional(),
  authoredBy: z.enum(MessageAuthor).describe('Message author filter. Defaults to anyone.').optional(),
  queries: z.array(z.string().min(1)).describe('Keyword queries matched against message content.').optional(),
  limit: z.number().int().min(1).max(25).describe('Maximum messages to return. Defaults to 20.').optional(),
  cursor: z.string().describe('Pagination cursor from a previous message_search result.').optional(),
});

export const messageSendInputSchema = z.strictObject({
  targetType: z.enum(MessageTargetType).describe('Destination for the new message.'),
  message: z.string().min(1).regex(/\S/, 'Message must contain a non-whitespace character.'),
  channelName: z.string().describe('Exact channel name as the messenger shows it.').optional(),
  channelID: resourceIDSchema.describe('Exact channel ID from message_context or a prior result.').optional(),
  personHint: z.string().describe('Name, @handle, or email of one direct-message recipient. Omit for a direct message to the requester themself.').optional(),
  personHints: z.array(z.string().min(1)).max(50).describe('Direct-message recipients for one fan-out send.').optional(),
  pin: z.boolean().describe('Whether to pin the created message. Defaults to false.').optional(),
  attachments: z.array(z.string().min(1)).describe('Workspace file paths to upload with the message, copied exactly from the attachment catalog or a file tool result. Use this to deliver an original file, such as an inbound image, to the target.').optional(),
  reason: z.string().describe('Reason shown to the approver.').optional(),
});

export const messageSendInputIntentSchema = messageSendInputSchema.partial();

const messageUpdateObjectSchema = z.strictObject({
  messageID: resourceIDSchema.describe('Exact message ID from message_search or message.send.'),
  oldText: z.string().min(1).regex(/\S/, 'oldText must contain a non-whitespace character.').describe('Exact text as it currently appears in that message, copied verbatim from a message_search preview or from the message you sent. Must occur exactly once in the message. Quote only the span that changes, never the whole message.').optional(),
  newText: z.string().describe('Text that replaces oldText. Empty string removes the span.').optional(),
  isPinned: z.boolean().describe('Whether the message should be pinned.').optional(),
  attachments: z.array(z.string().min(1)).describe('Workspace file paths to upload into the edited message, copied exactly from the attachment catalog or a file tool result. Use this to add an original file, such as an inbound image, to a message already sent. The message keeps its text when no oldText is given.').optional(),
});

export const messageUpdateInputSchema = messageUpdateObjectSchema
  .refine(hasPairedMessageEdit, 'oldText and newText must be given together.')
  .refine(hasMutationField, 'At least one message field must be updated.')
  .meta({ minProperties: 2 });

export const messageUpdateInputIntentSchema = messageUpdateObjectSchema.partial();

export const messageDeleteInputSchema = z.strictObject({
  messageIDs: uniqueMessageIDArraySchema.describe('Exact message IDs from message.search.'),
});

export const messageDeleteInputIntentSchema = messageDeleteInputSchema.partial();

const messageSearchCandidateSchema = z.strictObject({
  messageID: resourceIDSchema,
  channelID: resourceIDSchema,
  rootMessageID: resourceIDSchema.optional(),
  userID: resourceIDSchema,
  authoredBy: z.enum(MessageAuthor),
  createdAt: z.number().int().nonnegative(),
  text: z.string().optional(),
  preview: z.string().optional(),
  editable: z.boolean().optional(),
  deletable: z.boolean(),
  protectedReason: z.string().optional(),
});

export const messageContextResultSchema = z.strictObject({
  platform: z.string().min(1),
  conversationID: z.string(),
  conversationType: z.string(),
  channelID: z.string(),
  channelName: z.string(),
  replyTargetID: z.string(),
  rootMessageID: z.string(),
  currentMessageID: z.string(),
  requesterPersonID: z.string(),
  requesterPlatformUserID: z.string(),
  botUserID: resourceIDSchema,
  botUsername: z.string().min(1),
});

export const messageSearchResultSchema = z.strictObject({
  scope: z.enum(MessageSearchScope),
  queries: z.array(z.string()),
  authoredBy: z.enum(MessageAuthor),
  messageIDs: z.array(resourceIDSchema),
  candidates: z.array(messageSearchCandidateSchema),
  nextCursor: z.string().optional(),
  hasMore: z.boolean(),
});

const messageDeliveryFailureSchema = z.strictObject({
  personHint: z.string().optional(),
  messageID: z.string().optional(),
  errorCode: z.string().min(1),
  message: z.string().min(1),
});

export const messageSendResultSchema = z.strictObject({
  messageIDs: uniqueResourceIDArraySchema,
  deliveryStatus: z.literal(MessageDeliveryStatus.Sent),
  failures: z.array(messageDeliveryFailureSchema).optional(),
});

export const messageUpdateResultSchema = z.strictObject({
  messageID: resourceIDSchema,
  deliveryStatus: z.literal(MessageDeliveryStatus.Updated),
  messageUpdated: z.boolean(),
  isPinned: z.boolean().optional(),
});

export const messageDeleteResultSchema = z.strictObject({
  messageIDs: uniqueResourceIDArraySchema,
  deliveryStatus: z.literal(MessageDeliveryStatus.Deleted),
  failures: z.array(messageDeliveryFailureSchema).optional(),
});

export const documentReadInputSchema = z.strictObject({
  path: resourceIDSchema.describe('Exact absolute /workspace path of the document to read.'),
  maxPages: z.number().int().min(1).max(500).describe('Maximum PDF pages to extract. Omit for the runtime default.').optional(),
  maxOutputBytes: z.number().int().min(1024).max(1000000).describe('Maximum Markdown bytes to return. Omit for the runtime default.').optional(),
});

export const documentReadResultSchema = z.strictObject({
  status: z.literal('ok'),
  path: resourceIDSchema,
  format: z.literal('markdown'),
  content: z.string(),
  warnings: z.array(z.string()),
  truncated: z.boolean(),
  backend: z.string().optional(),
  model: z.string().optional(),
});

const imageReadAttachmentSchema = z.strictObject({
  devicePath: resourceIDSchema,
  filename: resourceIDSchema,
  contentType: resourceIDSchema,
  sizeBytes: z.number().int().nonnegative(),
  contentBase64: z.string().min(1),
});

export const imageReadInputSchema = z.strictObject({
  path: resourceIDSchema.describe('Exact absolute /workspace path of the image to read.'),
});

export const imageReadResultSchema = z.strictObject({
  status: z.literal('ok'),
  path: resourceIDSchema,
  attachments: z.array(imageReadAttachmentSchema).min(1),
});

export const browserOpenInputSchema = z.strictObject({
  url: resourceIDSchema.describe('Absolute HTTP or HTTPS URL to open.'),
});

export const browserOpenInputIntentSchema = browserOpenInputSchema.partial();

export const browserOpenResultSchema = z.strictObject({
  url: resourceIDSchema,
  requestedURL: resourceIDSchema,
  title: z.string().optional(),
  snapshotText: z.string().optional(),
  interactiveRefs: z.array(resourceIDSchema).optional(),
  capturedAt: resourceIDSchema,
});

export const browserSnapshotInputSchema = z.strictObject({});

export const browserSnapshotResultSchema = z.strictObject({
  url: resourceIDSchema.optional(),
  title: z.string().optional(),
  snapshotText: z.string(),
  interactiveRefs: z.array(resourceIDSchema),
  hasMore: z.boolean(),
  capturedAt: resourceIDSchema,
});

export const browserScreenshotInputSchema = z.strictObject({});

export const browserScreenshotResultSchema = z.strictObject({
  ok: z.literal(true),
  action: z.literal('screenshot'),
  attachments: z.array(imageReadAttachmentSchema.omit({ devicePath: true })).min(1),
  capturedAt: resourceIDSchema,
});

const browserClickObjectSchema = z.strictObject({
  target: resourceIDSchema.optional(),
  ref: resourceIDSchema.optional(),
  selector: resourceIDSchema.optional(),
});

export const browserClickInputSchema = browserClickObjectSchema
  .refine(hasAnyField, 'A browser target, ref, or selector is required.')
  .meta({ minProperties: 1 });

export const browserClickInputIntentSchema = browserClickObjectSchema;

export const browserClickResultSchema = z.strictObject({
  ok: z.literal(true),
  action: z.literal('click'),
  target: resourceIDSchema,
  capturedAt: resourceIDSchema,
});

const artifactReviewIssueSchema = z.strictObject({
  severity: z.enum(ArtifactIssueSeverity),
  category: z.enum(ArtifactIssueCategory),
  target: z.string(),
  message: z.string(),
  suggestedFix: z.string(),
});

export const artifactReviewInputSchema = z.strictObject({
  artifactKind: z.enum(ArtifactKind),
  intent: resourceIDSchema,
  rubric: resourceIDSchema,
  evidence: z.array(z.strictObject({
    role: resourceIDSchema,
    path: resourceIDSchema,
    mimeType: z.enum(ArtifactEvidenceMimeType),
    label: resourceIDSchema,
  })).min(1).max(8),
  expectedText: z.array(z.strictObject({
    target: resourceIDSchema,
    text: z.string(),
  })).optional(),
  previousIssues: z.array(artifactReviewIssueSchema).optional(),
});

export const artifactReviewResultSchema = z.strictObject({
  passed: z.boolean(),
  issues: z.array(artifactReviewIssueSchema),
  acceptedWarnings: z.array(z.string()),
  summary: z.string(),
});

export const webSearchInputSchema = z.strictObject({
  query: z.string().min(1).regex(/\S/, 'Search query must contain a non-whitespace character.'),
  location: z.string().optional(),
  language: z.string().optional(),
  limit: z.number().int().min(1).max(10).optional(),
  allowedDomains: z.array(z.string().min(1)).optional(),
  excludedDomains: z.array(z.string().min(1)).optional(),
});

const webSearchResultItemSchema = z.strictObject({
  title: z.string(),
  url: z.string(),
  snippet: z.string(),
  source: z.string().optional(),
});

export const webSearchResultSchema = z.strictObject({
  provider: z.string(),
  remoteLLMInvolved: z.boolean(),
  compatibility: z.string(),
  query: z.string(),
  answer: z.string(),
  results: z.array(webSearchResultItemSchema),
});

function hasMutationField(document: object): boolean {
  return Object.keys(document).length > 1;
}

function hasPairedMessageEdit(document: { oldText?: string | undefined; newText?: string | undefined }): boolean {
  return (document.oldText === undefined) === (document.newText === undefined);
}

function hasAnyField(document: object): boolean {
  return Object.keys(document).length > 0;
}

const taskToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: 'task_add',
    namespace: 'task',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_task',
    policyResource: 'tool:task_add',
    description: 'Create a new workspace task with typed task fields. Never ask the user to classify it: send the business when you know it, and the record decides whatever business, type and size is left out from the company definitions. Use this to add a todo or assignment for the requester or another team member. Use task_update for existing work.',
    version: '7',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: taskAddInputSchema,
    inputIntentSchema: taskAddInputIntentSchema,
    result: {
      schema: taskResultSchema,
      effects: [{
        objectType: 'task',
        effect: ResourceMutationEffect.Created,
        resultField: 'taskID',
        effectIdentity: ResourceEffectIdentity.ID,
      }],
    },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: 'success', action: 'write_task', targetKind: 'task' },
  },
  {
    name: 'task_list',
    namespace: 'task',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_task',
    policyResource: 'tool:task_list',
    description: "List workspace tasks with optional filters. Use this to answer 'what tasks does X have', 'what is on my plate', or 'show incomplete items this week'. It reads the requester's own tasks unless personHints names other people or scope is all.",
    version: '4',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: taskListInputSchema,
    result: { schema: taskListResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'task_board_get',
    namespace: 'task',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_task',
    policyResource: 'tool:task_board_get',
    description: 'Get one task board week: its visible cards, including undated and overdue carry-over work, with direct-child progress across all weeks. Counts describe this board selection; use task_list for complete history and reports. Uses the same person scope and read permissions as task_list.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: taskBoardInputSchema,
    result: { schema: taskBoardResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'task_update',
    namespace: 'task',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_task',
    policyResource: 'tool:task_update',
    description: 'Update explicit fields on an existing task, including who takes part in it. Classification fields use registeredLabels from task_list: business is a workspace label, while organizationHint and opportunityHint link CRM records. Choose classification values from those labels and the work context when the requester delegates that judgment. taskHint is the exact task ID or exact task title from a task_list result, resolved server-side to the canonical task; use task_list first when neither is known. At least one mutable field is required.',
    version: '6',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: taskUpdateInputSchema,
    inputIntentSchema: taskUpdateInputIntentSchema,
    result: {
      schema: taskResultSchema,
      effects: [{
        objectType: 'task',
        effect: ResourceMutationEffect.Updated,
        resultField: 'taskID',
        effectIdentity: ResourceEffectIdentity.ID,
      }],
    },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: 'success', action: 'write_task', targetKind: 'task' },
  },
  {
    name: 'task_delete',
    namespace: 'task',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_task',
    policyResource: 'tool:task_delete',
    description: 'Permanently delete a task. taskHint is the exact task ID or exact task title from a task_list result, resolved server-side to the canonical task; use task_list first when neither is known. Requires approval; this action is irreversible.',
    version: '3',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: taskDeleteInputSchema,
    inputIntentSchema: taskDeleteInputIntentSchema,
    result: {
      schema: taskDeleteResultSchema,
      effects: [{
        objectType: 'task',
        effect: ResourceMutationEffect.Deleted,
        resultField: 'taskID',
        effectIdentity: ResourceEffectIdentity.ID,
      }],
    },
    sideEffect: CapabilitySideEffect.Destructive,
    requiresApproval: true,
    completionEvidence: { mode: 'success', action: 'delete_task', targetKind: 'task' },
  },
  {
    name: 'task_vocabulary_set',
    namespace: 'task',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_task',
    policyResource: 'tool:task_vocabulary_set',
    description: 'Set the task vocabulary: the business and task type labels this company files work under. Each list is written at once, so read registeredLabels from a task_list result first and send it back with what changes. A label a task still carries cannot be dropped. This is an administrator’s.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: taskVocabularySetInputSchema,
    inputIntentSchema: taskVocabularySetInputIntentSchema,
    result: { schema: taskLabelVocabularySchema, effects: [{ objectType: 'task_vocabulary', effect: ResourceMutationEffect.Updated, effectIdentity: ResourceEffectIdentity.Singleton }] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: 'success', action: 'write_task', targetKind: 'task_vocabulary' },
  },
  {
    name: taskLabelGetToolName,
    namespace: 'task',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'workspace_task',
    policyResource: `tool:${taskLabelGetToolName}`,
    description: "Get the business, type and size labels a new task should carry, decided by the company's model from the task's title and note against the company's registered definitions. A label nothing fits comes back as an empty string.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: taskLabelGetInputSchema,
    result: { schema: taskLabelGetResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Computation,
  },
];

export enum WorkspaceLeaveStatus {
  Requested = 'requested',
  Approved = 'approved',
  Rejected = 'rejected',
}

export enum WorkspaceDecision {
  Approved = 'approved',
  Rejected = 'rejected',
}

const leaveHintSchema = z.string().min(1).max(256).describe(
  'Identifies the leave to decide: its exact leave ID, or enough of the line leave_list shows for that row — the person, the kind, and the start date — to name exactly one. Resolved server-side; if it does not uniquely resolve, the call fails with a candidates list to retry against.',
);

export const leaveListInputSchema = z.strictObject({
  personHints: z.array(z.string()).describe(personHintsDescription('leave')).optional(),
  scope: z.enum(WorkspaceTaskScope).describe(whoseScopeDescription('leave')).optional(),
  from: z.string().describe(`Earliest day to include. ${momentDescription} Omit for no lower bound.`).optional(),
  to: z.string().describe(`Latest day to include. ${momentDescription} Omit for no upper bound.`).optional(),
  status: z.enum(WorkspaceLeaveStatus).describe('Filter by status. Omit the field to return every status.').optional(),
  limit: z.number().describe('Maximum number of rows to return.').optional(),
});

export const leaveBalanceInputSchema = z.strictObject({
  personHints: z.array(z.string()).describe(personHintsDescription('leave balance')).optional(),
  scope: z.enum(WorkspaceTaskScope).describe(whoseScopeDescription('leave balances')).optional(),
  year: z.number().describe('The leave year to count against, named by the calendar year it opens in: with a March start, February 2026 is leave year 2025. Omit for the year the requester is in now.').optional(),
});

export const leaveRequestInputSchema = z.strictObject({
  personHint: z.string().describe('Name or email of the person the leave belongs to. Only an administrator files leave for somebody else. Omit for the requester.').optional(),
  kind: z.string().describe('The kind of leave, named as this company registers it. leave_list returns registeredKinds; a kind outside that list fails with the registered ones.'),
  startsAt: z.string().describe(`First day of the leave. ${momentDescription}`),
  endsAt: z.string().describe(`Last day of the leave. ${momentDescription}`),
  days: z.number().describe('How many days of entitlement this consumes, which is not the same as the span it covers: a Friday-to-Monday leave spans four days and consumes two, and a half day is one day costing 0.5.'),
  note: z.string().describe('The reason, in the requester\'s own words. Omit when they gave none.').optional(),
});

export const leaveRequestInputIntentSchema = leaveRequestInputSchema.partial();

export const leaveDecideInputSchema = z.strictObject({
  leaveHint: leaveHintSchema,
  decision: z.enum(WorkspaceDecision).describe('approved or rejected.'),
});

export const leaveDecideInputIntentSchema = z.strictObject({
  decision: z.enum(WorkspaceDecision).describe('approved or rejected.').optional(),
});

export const leaveUpdateInputSchema = z.strictObject({
  leaveHint: leaveHintSchema,
  kind: z.string().describe('The kind of leave, named as this company registers it. Omit to keep the kind it has.').optional(),
  startsAt: z.string().describe(`First day of the leave. ${momentDescription} Omit to keep the day it starts on.`).optional(),
  endsAt: z.string().describe(`Last day of the leave. ${momentDescription} Omit to keep the day it ends on.`).optional(),
  days: z.number().describe('How many days of entitlement this consumes. Omit to keep the number it consumes.').optional(),
  note: z.string().describe('The reason, in the requester\'s own words. Omit to keep the note it has.').optional(),
});

export const leaveUpdateInputIntentSchema = leaveUpdateInputSchema.partial();

export const leaveDeleteInputSchema = z.strictObject({
  leaveHint: leaveHintSchema,
});

export const leaveDeleteInputIntentSchema = z.strictObject({});

export const leaveResultSchema = z.strictObject({
  leaveID: resourceIDSchema,
  personID: z.string(),
  person: z.string(),
  kindID: z.string(),
  kind: z.string(),
  days: z.number(),
  status: z.string(),
  isPaid: z.boolean(),
  isDeducted: z.boolean(),
  startDate: z.string(),
  endDate: z.string(),
  startsAt: z.string(),
  endsAt: z.string(),
  note: z.string().nullable(),
});

export const leaveListResultSchema = z.strictObject({
  leave: z.array(leaveResultSchema),
  count: z.number().int(),
  scope: z.string(),
  personID: z.string().nullable(),
  personName: z.string(),
  statusFilter: z.string().nullable(),
  registeredKinds: z.array(z.string()),
});

const leaveBalanceEntrySchema = z.strictObject({
  personID: z.string(),
  personName: z.string(),
  grantedDays: z.number().nullable(),
  remainingDays: z.number().nullable(),
  usedDays: z.number().nullable(),
  tracking: z.string(),
});

export const leaveBalanceResultSchema = z.strictObject({
  scope: z.string(),
  year: z.number().int(),
  count: z.number().int(),
  balances: z.array(leaveBalanceEntrySchema),
});

export const leaveGrantSetInputSchema = z.strictObject({
  personHint: z.string().describe('Name or email of the person whose entitlement to set.'),
  days: z.number().describe('The leave they are entitled to in a year, in days. It replaces the number they had rather than adding to it, so a request for so many days more is read with leave_balance first and written as the total.'),
});

export const leaveGrantSetInputIntentSchema = leaveGrantSetInputSchema.partial();

export const leaveGrantSetResultSchema = leaveBalanceEntrySchema;

export const leaveReturnEarlyInputSchema = z.strictObject({
  location: z.string().describe('The registered work location they came back to, named as company_settings_get lists them. Omit when they did not say where.').optional(),
});

export const leaveReturnEarlyInputIntentSchema = leaveReturnEarlyInputSchema.partial();

export const leaveReturnEarlyResultSchema = z.strictObject({
  shortened: z.boolean(),
  leaveID: z.string().nullable(),
  endsAt: z.string().nullable(),
  days: z.number().nullable(),
});

export enum WorkspaceAttendanceKind {
  ClockIn = 'clock_in',
  ClockOut = 'clock_out',
}

const attendanceDayDescription = 'The day it happened, as yyyy-mm-dd in the company time zone.';
const attendanceTimeDescription = 'The time of day it happened, as 24-hour HH:MM in the company time zone.';

const attendanceHintSchema = z.string().min(1).max(256).describe(
  'Identifies the attendance record: its exact event ID, or the line attendance_list shows for that row, written as it appeared there (name · kind · yyyy-mm-dd HH:MM). Resolved server-side; if it does not uniquely resolve, the call fails with a candidates list to retry against.',
);

const attendanceReasonSchema = z.string();

export const attendanceListInputSchema = z.strictObject({
  personHints: z.array(z.string()).describe(personHintsDescription('attendance')).optional(),
  scope: z.enum(WorkspaceTaskScope).describe(whoseScopeDescription('attendance')).optional(),
  from: z.string().describe(`Earliest day to include. ${attendanceDayDescription} Omit to start thirty days ago.`).optional(),
  to: z.string().describe(`Latest day to include. ${attendanceDayDescription} Omit to end today.`).optional(),
  pageOffset: z.number().int().min(0).optional(),
  pageLimit: z.number().int().min(1).max(100).optional(),
  teamSearch: z.string().max(128).optional(),
  changedBySearch: z.string().max(128).optional(),
  selectedTeamKey: z.string().max(100).optional(),
  selectedChangedByID: z.string().uuid().optional(),
  handWrittenOnly: z.boolean().describe('Keep only the records somebody wrote or changed by hand, and drop the ones clocked live. A hand-written record carries the reason it was written, or the moment it was moved from, or both; a live clock carries neither. Use it for "what has been written by hand this month" or to review what an administrator entered for somebody. Omit for every record in the window.').optional(),
  limit: z.number().describe('Maximum number of rows to return.').optional(),
});

export const attendanceAddInputSchema = z.strictObject({
  personHint: z.string().describe('Name or email of the person the record belongs to. Omit for the requester.').optional(),
  kind: z.enum(WorkspaceAttendanceKind).describe('clock_in for arriving, clock_out for leaving.'),
  date: z.string().describe(`The day the person actually arrived or left. ${attendanceDayDescription} Omit it when only a time is known: the record takes the latest day that time has already come, today or else yesterday.`).optional(),
  time: z.string().describe(`The time they actually arrived or left. ${attendanceTimeDescription} Omit it, with date, for the moment this call is made; a date needs a time.`).optional(),
  location: z.string().describe('The registered work location they were at. clock_out does not use it, so omit it there.').optional(),
  reason: attendanceReasonSchema.describe('Why the record is being written by hand, in the requester\'s own words. It is kept when given and never demanded; somebody clocking in or out right now needs none.').optional(),
  reasonCode: z.enum(attendanceChangeReasons).describe('Structured change reason. New UI changes use this instead of a free-text note; legacy reason text remains supported.').optional(),
});

export const attendanceAddInputIntentSchema = attendanceAddInputSchema.partial();

export const attendanceCorrectionSchema = z.strictObject({
  eventHint: attendanceHintSchema,
  date: z.string().describe(`The day it actually happened. ${attendanceDayDescription} Omit to keep the day it has.`).optional(),
  time: z.string().describe(`The time it actually happened. ${attendanceTimeDescription} Omit to keep the time it has.`).optional(),
  location: z.string().describe('The registered work location it happened at. Omit to keep the one it has.').optional(),
});

export const attendanceUpdateInputSchema = z.strictObject({
  corrections: z.array(attendanceCorrectionSchema).describe('The records to correct, together. Correcting one record is an array of one. A day whose clock-in and clock-out both move goes in one call, because the record refuses a correction that would leave the day out of order partway through.'),
  reason: attendanceReasonSchema.describe('Why the records were wrong, in the requester\'s own words.').optional(),
  reasonCode: z.enum(attendanceChangeReasons).describe('Structured change reason. New UI changes use this instead of a free-text note; legacy reason text remains supported.').optional(),
  undoOnly: z.boolean().describe('Undo the last observed attendance change atomically. Unknown historical previous state is refused, never inferred as deletion.').optional(),
});

export const attendanceUpdateInputIntentSchema = z.strictObject({
  corrections: z.array(attendanceCorrectionSchema).describe('The records to correct, together.').optional(),
  reason: attendanceReasonSchema.describe('Why the records were wrong.').optional(),
  reasonCode: z.enum(attendanceChangeReasons).describe('Structured change reason. New UI changes use this instead of a free-text note; legacy reason text remains supported.').optional(),
  undoOnly: z.boolean().describe('Undo the last observed attendance change atomically. Unknown historical previous state is refused, never inferred as deletion.').optional(),
});

export const attendanceDeleteInputSchema = z.strictObject({
  eventHint: attendanceHintSchema,
  reason: attendanceReasonSchema.describe('Why the record should not be there, in the requester\'s own words.').optional(),
  reasonCode: z.enum(attendanceChangeReasons).describe('Structured change reason. New UI changes use this instead of a free-text note; legacy reason text remains supported.').optional(),
  undoOnly: z.boolean().describe('Undo the last observed attendance change atomically. Unknown historical previous state is refused, never inferred as deletion.').optional(),
});

export const attendanceDeleteInputIntentSchema = z.strictObject({
  reason: attendanceReasonSchema.describe('Why the record should not be there.').optional(),
  reasonCode: z.enum(attendanceChangeReasons).describe('Structured change reason. New UI changes use this instead of a free-text note; legacy reason text remains supported.').optional(),
  undoOnly: z.boolean().describe('Undo the last observed attendance change atomically. Unknown historical previous state is refused, never inferred as deletion.').optional(),
});

export const attendanceResultSchema = z.strictObject({
  eventID: resourceIDSchema,
  personID: z.string(),
  person: z.string(),
  kind: z.string(),
  date: z.string(),
  time: z.string(),
  occurredAt: z.string(),
  location: z.string().nullable(),
  wasCorrected: z.boolean(),
  originalDate: z.string().nullable(),
  originalTime: z.string().nullable(),
  originalOccurredAt: z.string().nullable(),
  originalLocation: z.string().nullable().optional(),
  previousRecorded: z.boolean().optional(),
  personEmail: z.string().nullable().optional(),
  changedByID: z.string().nullable().optional(),
  reason: z.string().nullable(),
  teamName: z.string().optional(),
  changedByName: z.string().nullable().optional(),
  changedByEmail: z.string().nullable().optional(),
  changedBySource: z.enum(['observed','legacy_subject']).optional(),
  changedAt: z.string().nullable().optional(),
});

export const attendanceListResultSchema = z.strictObject({
  scope: z.string(),
  personID: z.string().nullable(),
  personName: z.string(),
  from: z.string(),
  to: z.string(),
  serverTime: z.string(),
  backdatedAfterMinutes: z.number().int(),
  count: z.number().int(),
  attendance: z.array(attendanceResultSchema),
  totalCount: z.number().int().optional(),
  pageOffset: z.number().int().optional(),
  pageLimit: z.number().int().optional(),
});

export const legacyAttendanceResultSchema = z.object({
  eventID: resourceIDSchema,
  personID: z.string(),
  person: z.string(),
  kind: z.string(),
  date: z.string(),
  time: z.string(),
  occurredAt: z.string(),
  location: z.string().nullable(),
  wasCorrected: z.boolean(),
  originalDate: z.string().nullable(),
  originalTime: z.string().nullable(),
  originalOccurredAt: z.string().nullable(),
  reason: z.string().nullable(),
});

export const legacyAttendanceListResultSchema = z.object({
  scope: z.string(),
  personID: z.string().nullable(),
  personName: z.string(),
  from: z.string(),
  to: z.string(),
  serverTime: z.string(),
  backdatedAfterMinutes: z.number().int(),
  count: z.number().int(),
  attendance: z.array(legacyAttendanceResultSchema),
});

export const attendanceWriteResultSchema = z.strictObject({
  status: z.string(),
  eventID: z.string().nullable(),
  backdated: z.boolean(),
});

export const attendanceAddResultSchema = attendanceWriteResultSchema.extend({
  event: savedAttendanceEventSchema.optional(),
});

const calendarToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: CalendarToolName.Add,
    namespace: 'calendar',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_calendar',
    policyResource: 'tool:event_add',
    description: 'Create a calendar event with a concrete time range. Resolve natural-language dates and times before calling. Use event_update for an existing event.',
    version: '2',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: calendarAddInputSchema,
    inputIntentSchema: calendarAddInputIntentSchema,
    result: {
      schema: calendarEventResultSchema,
      effects: [{
        objectType: 'calendar',
        effect: ResourceMutationEffect.Created,
        resultField: 'eventID',
        effectIdentity: ResourceEffectIdentity.ID,
      }],
    },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: 'success', action: 'write_calendar', targetKind: 'calendar' },
  },
  {
    name: CalendarToolName.List,
    namespace: 'calendar',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_calendar',
    policyResource: 'tool:event_list',
    description: 'List everything on the company calendar in a concrete time window: scheduled events and the approved leave of everyone who is off, optionally filtered by title, description, or location. Each entry names its source; a leave entry is read-only and cannot be updated or deleted. Resolve natural-language dates to startISO and endISO before calling.',
    version: '3',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: calendarListInputSchema,
    result: { schema: calendarListResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: CalendarToolName.Update,
    namespace: 'calendar',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_calendar',
    policyResource: 'tool:event_update',
    description: 'Update explicit fields on a calendar event. eventHint is the exact event ID or exact event title from a event_list result, resolved server-side to the canonical event; use event_list first when neither is known. At least one mutable field is required.',
    version: '3',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: calendarUpdateInputSchema,
    inputIntentSchema: calendarUpdateInputIntentSchema,
    result: {
      schema: calendarEventResultSchema,
      effects: [{
        objectType: 'calendar',
        effect: ResourceMutationEffect.Updated,
        resultField: 'eventID',
        effectIdentity: ResourceEffectIdentity.ID,
      }],
    },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: 'success', action: 'write_calendar', targetKind: 'calendar' },
  },
  {
    name: CalendarToolName.Delete,
    namespace: 'calendar',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_calendar',
    policyResource: 'tool:event_delete',
    description: 'Permanently delete a calendar event. eventHint is the exact event ID or exact event title from a event_list result, resolved server-side to the canonical event; use event_list first when neither is known. Requires approval; this action is irreversible.',
    version: '2',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: calendarDeleteInputSchema,
    inputIntentSchema: calendarDeleteInputIntentSchema,
    result: {
      schema: calendarDeleteResultSchema,
      effects: [{
        objectType: 'calendar',
        effect: ResourceMutationEffect.Deleted,
        resultField: 'eventID',
        effectIdentity: ResourceEffectIdentity.ID,
      }],
    },
    sideEffect: CapabilitySideEffect.Destructive,
    requiresApproval: true,
    completionEvidence: { mode: 'success', action: 'write_calendar', targetKind: 'calendar' },
  },
];

const messageToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: MessageToolName.Context,
    namespace: 'message',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'platform_message',
    policyResource: 'tool:message_context',
    description: 'Return the conversation the message arrived in, exactly as the messenger holds it, with its thread, requester, and bot identities.',
    version: '2',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: messageContextInputSchema,
    result: { schema: messageContextResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: MessageToolName.Search,
    namespace: 'message',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'platform_message',
    policyResource: 'tool:message_search',
    description: 'Find messages in an exact conversation scope, or read known ones in full. Searching by queries returns message IDs with a short preview around the match. Passing messageIDs instead returns those messages complete, which is what you need before rewriting or summarising a long message rather than a phrase inside it.',
    version: '2',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: messageSearchInputSchema,
    result: { schema: messageSearchResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: MessageToolName.Send,
    namespace: 'message',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'platform_message',
    policyResource: 'tool:message_send',
    description: 'Send a message to a direct message, channel, or the current conversation after approval. attachments uploads workspace files with the message. For targetType=channel, name the channel with channelName; only pass a channelID a tool result gave you. Never answer the person you are replying to with this tool — the final reply is delivered for you. To correct a sent message, use message_update.',
    version: '3',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: messageSendInputSchema,
    inputIntentSchema: messageSendInputIntentSchema,
    result: {
      schema: messageSendResultSchema,
      effects: [{
        objectType: 'message',
        effect: ResourceMutationEffect.Sent,
        resultField: 'messageIDs',
        effectIdentity: ResourceEffectIdentity.ID,
      }],
    },
    sideEffect: CapabilitySideEffect.ExternalSend,
    idempotency: { supported: true, required: false, scope: 'operation' },
    requiresApproval: true,
    completionEvidence: { mode: 'success', action: 'send_message', targetKind: 'message' },
  },
  {
    name: MessageToolName.Update,
    namespace: 'message',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'platform_message',
    policyResource: 'tool:message_update',
    description: 'Replace one exact span of text inside the assistant\'s messages or your own, leaving the rest of it untouched. oldText must appear exactly once in that message; copy it verbatim from a message_search preview or from the message you sent. This is the tool for every correction to something you already posted — when the user points out a mistake, fix that message instead of posting a correction as a new one. It runs immediately without asking the user to confirm, because it can only change text you quoted. If oldText does not match, the call fails without changing anything and returns the message as it currently reads, so retry with a span copied from that. To rewrite a long message wholesale, read it in full first with message_search messageIDs and quote what you read. attachments uploads workspace files into the edited message; pass it alone to add a file, such as an inbound image original, without changing the text.',
    version: '2',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: messageUpdateInputSchema,
    inputIntentSchema: messageUpdateInputIntentSchema,
    result: {
      schema: messageUpdateResultSchema,
      effects: [{
        objectType: 'message',
        effect: ResourceMutationEffect.Updated,
        resultField: 'messageID',
        effectIdentity: ResourceEffectIdentity.ID,
      }],
    },
    sideEffect: CapabilitySideEffect.ExternalWrite,
    requiresApproval: false,
    completionEvidence: { mode: 'success', action: 'update_message', targetKind: 'message' },
  },
  {
    name: MessageToolName.Delete,
    namespace: 'message',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'platform_message',
    policyResource: 'tool:message_delete',
    description: 'Permanently delete exact message IDs from message_search after approval. You may delete yours, the assistant\'s, or anyone\'s if you hold the channel admin role. List only the exact messages the user asked to remove; when several search matches quote or mention the same text, pick the one message that is the target itself, never the whole match list.',
    version: '2',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: messageDeleteInputSchema,
    inputIntentSchema: messageDeleteInputIntentSchema,
    result: {
      schema: messageDeleteResultSchema,
      effects: [{
        objectType: 'message',
        effect: ResourceMutationEffect.Deleted,
        resultField: 'messageIDs',
        effectIdentity: ResourceEffectIdentity.ID,
      }],
    },
    sideEffect: CapabilitySideEffect.Destructive,
    requiresApproval: true,
    completionEvidence: { mode: 'success', action: 'delete_message', targetKind: 'message' },
  },
];

const imageGenerateInputSchema = z.strictObject({
  aspectRatio: z.enum(["1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3"]).describe("Output aspect ratio. Defaults to 1:1 if omitted.").optional(),
  path: z.string().describe("Absolute workspace path to save the generated PNG, e.g. /workspace/shared/logo.png. Must be under /workspace and end in .png."),
  prompt: z.string().describe("Detailed description of the image to generate. Write it like describing a scene to an artist, not a keyword list."),
});

const imageGenerateInputIntentSchema = imageGenerateInputSchema.partial();

const imageGenerateResultSchema = z.strictObject({
  status: z.literal('ok'),
  path: resourceIDSchema.describe('Workspace path the generated PNG was saved to.'),
  attachments: z.array(imageReadAttachmentSchema).min(1),
});

const fileToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: DocumentToolName.Read,
    namespace: 'document',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'workspace_document',
    policyResource: 'tool:document_read',
    description: 'Read a workspace document and return Markdown content. path takes an exact /workspace path, or an attachment\'s exact url copied verbatim from the conversation — a document attached to an earlier message is fetched by that url. Never invent a filesystem path from a url. Use image_read for image files.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.High,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: documentReadInputSchema,
    result: { schema: documentReadResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: ImageToolName.Read,
    namespace: 'image',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'workspace_document',
    policyResource: 'tool:image_read',
    description: 'Read a workspace image and return a base64 attachment. path takes an exact /workspace path, or an attachment\'s exact url copied verbatim from the conversation — an image attached to an earlier message is fetched by that url. Never invent a filesystem path from a url. Use document_read for document files.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: imageReadInputSchema,
    result: { schema: imageReadResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: "image_generate",
    namespace: "image",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "workspace_document",
    policyResource: "tool:image_generate",
    description: "Generate a new image from a text prompt and save it to a workspace path. Provide an absolute /workspace output path ending in .png. Optionally set aspectRatio. Returns the saved image as an attachment. Use image_read instead if you need to read an existing image file.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.High,
    inputSchema: imageGenerateInputSchema,
    inputIntentSchema: imageGenerateInputIntentSchema,
    result: { schema: imageGenerateResultSchema, effects: [{ objectType: "image", effect: ResourceMutationEffect.Created, resultField: "path", effectIdentity: ResourceEffectIdentity.Path }] },
    sideEffect: CapabilitySideEffect.ExternalWrite,
  },
];

const browserToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: BrowserToolName.Open,
    approvalScope: 'browser',
    namespace: 'browser',
    answeredBy: CapabilityAnsweredBy.Local,
    privacyClass: 'device_browser',
    policyResource: 'tool:browser_open',
    description: 'Open an exact HTTP or HTTPS URL in the browser and return the resulting page identity and initial structure.',
    version: '2',
    estimatedLatency: CapabilityEstimatedLatency.Interactive,
    inputSchema: browserOpenInputSchema,
    inputIntentSchema: browserOpenInputIntentSchema,
    result: { schema: browserOpenResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Connect,
  },
  {
    name: BrowserToolName.Snapshot,
    approvalScope: 'browser',
    namespace: 'browser',
    answeredBy: CapabilityAnsweredBy.Local,
    privacyClass: 'device_browser',
    policyResource: 'tool:browser_snapshot',
    description: 'Read the current browser page structure and return stable interactive references for inspection and control.',
    version: '2',
    estimatedLatency: CapabilityEstimatedLatency.Interactive,
    inputSchema: browserSnapshotInputSchema,
    result: { schema: browserSnapshotResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: BrowserToolName.Screenshot,
    approvalScope: 'browser',
    namespace: 'browser',
    answeredBy: CapabilityAnsweredBy.Local,
    privacyClass: 'device_browser',
    policyResource: 'tool:browser_screenshot',
    description: 'Capture the visible browser page as a PNG attachment for visual inspection.',
    version: '4',
    estimatedLatency: CapabilityEstimatedLatency.Interactive,
    inputSchema: browserScreenshotInputSchema,
    result: { schema: browserScreenshotResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: BrowserToolName.Click,
    approvalScope: 'browser',
    namespace: 'browser',
    answeredBy: CapabilityAnsweredBy.Local,
    privacyClass: 'device_browser',
    policyResource: 'tool:browser_click',
    description: 'Click one exact target from the current browser snapshot and return the completed action.',
    version: '2',
    estimatedLatency: CapabilityEstimatedLatency.Interactive,
    inputSchema: browserClickInputSchema,
    inputIntentSchema: browserClickInputIntentSchema,
    result: { schema: browserClickResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.ExternalWrite,
  },
];

const artifactToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: ArtifactToolName.Review,
    namespace: 'artifact',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'workspace_document',
    policyResource: 'tool:artifact_review',
    description: 'Review rendered artifact screenshots against a concrete intent and rubric, returning typed visual issues and suggested fixes.',
    version: '2',
    estimatedLatency: CapabilityEstimatedLatency.High,
    inputSchema: artifactReviewInputSchema,
    result: {
      schema: artifactReviewResultSchema,
      effects: [],
      evidenceCondition: { resultField: 'passed', equals: true },
    },
    sideEffect: CapabilitySideEffect.Read,
  },
];

const webFetchInputSchema = z.strictObject({
  allowedDomains: z.array(z.string()).describe("If set, only URLs from these domains are fetched; others are skipped with an error. Useful for safety when the URL list is dynamic.").optional(),
  blockedDomains: z.array(z.string()).describe("Domains to refuse fetching even if present in urls, e.g. [\"malicious.example\"]. Supplements the built-in block list.").optional(),
  maxContentTokens: z.int().describe("Soft cap on tokens returned per URL. Defaults to 50000; maximum is 100000. Reduce when fetching many URLs.").optional(),
  urls: z.array(z.string()).describe("List of fully-qualified public URLs to fetch, e.g. [\"https://example.com/article\"]. Maximum 10 URLs per call. Localhost and private IPs are blocked."),
});

export const webFetchResultSchema = z.strictObject({
  provider: z.string(),
  remoteLLMInvolved: z.boolean(),
  compatibility: z.string(),
  results: z.array(z.strictObject({
    url: z.string().describe('URL that was asked for. Empty when the provider answered several URLs as one combined text.'),
    finalURL: z.string(),
    title: z.string(),
    content: z.string(),
  })),
  errors: z.array(z.strictObject({
    url: z.string(),
    error: z.string(),
  })),
});

const webToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: WebToolName.Search,
    namespace: 'web',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'public_web',
    policyResource: 'tool:web_search',
    description: 'Search the public web and return ranked result snippets. Use this when you need current information, facts, or links that are not already in context. Do not use for workspace data, calendar, mail, or tasks — those have dedicated tools.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: webSearchInputSchema,
    result: { schema: webSearchResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: WebToolName.Fetch,
    namespace: "web",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "public_web",
    policyResource: "tool:web_fetch",
    description: "Fetch and return the text content of one or more public URLs. Use after web_search when you need the full page content, not just a snippet. Do not fetch localhost or private network addresses — those are blocked.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: webFetchInputSchema,
    result: { schema: webFetchResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
];

const leaveToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: 'leave_list',
    namespace: 'leave',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_leave',
    policyResource: 'tool:leave_list',
    description: "List leave in the record. Use this to answer 'when am I off', 'who is away next week', or 'what have I not had decided yet'. It reads the requester's own leave unless personHints names other people or scope is all. registeredKinds in the result names the leave types this company offers, which is what leave_request takes.",
    version: '2',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: leaveListInputSchema,
    result: { schema: leaveListResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'leave_balance',
    namespace: 'leave',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_leave',
    policyResource: 'tool:leave_balance',
    description: "Read how much leave is left for a year. Use this to answer 'how many days do I have left'. The default scope is the requester; scope all is everybody who works here, which is what reviewing entitlements wants. tracking is unlimited when the company grants no fixed entitlement, and then the day counts are null rather than zero. Somebody whose balance the requester may not read comes back with null day counts rather than failing.",
    version: '2',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: leaveBalanceInputSchema,
    result: { schema: leaveBalanceResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'leave_request',
    namespace: 'leave',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_leave',
    policyResource: 'tool:leave_request',
    description: 'File a leave request. It is filed as requested and grants nothing until somebody decides it. Whether the leave is paid and whether it consumes the entitlement follow from the kind, so do not ask the requester for either. It is the requester\'s own leave unless personHint names somebody else, which only an administrator may do.',
    version: '2',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: leaveRequestInputSchema,
    inputIntentSchema: leaveRequestInputIntentSchema,
    result: {
      schema: leaveResultSchema,
      effects: [{
        objectType: 'leave',
        effect: ResourceMutationEffect.Created,
        resultField: 'leaveID',
        effectIdentity: ResourceEffectIdentity.ID,
      }],
    },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: 'success', action: 'write_leave', targetKind: 'leave' },
  },
  {
    name: 'leave_update',
    namespace: 'leave',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_leave',
    policyResource: 'tool:leave_update',
    description: 'Correct a leave that was filed wrong: its days, its kind, how much entitlement it consumes, or the note. A person corrects their own while it is still waiting on a decision; an administrator corrects anybody\'s at any time, which is how a leave filed for the wrong year is moved to the right one. Only the fields you pass change.',
    version: '2',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: leaveUpdateInputSchema,
    inputIntentSchema: leaveUpdateInputIntentSchema,
    result: {
      schema: leaveResultSchema,
      effects: [{
        objectType: 'leave',
        effect: ResourceMutationEffect.Updated,
        resultField: 'leaveID',
        effectIdentity: ResourceEffectIdentity.ID,
      }],
    },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    requiresApproval: true,
    completionEvidence: { mode: 'success', action: 'write_leave', targetKind: 'leave' },
  },
  {
    name: 'leave_delete',
    namespace: 'leave',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_leave',
    policyResource: 'tool:leave_delete',
    description: 'Take a leave back out of the record entirely, as though it was never filed. A person takes back their own while it is still waiting on a decision; an administrator takes back anybody\'s. Entitlement an approved leave spent comes back with it. To refuse a leave rather than erase it, decide it rejected.',
    version: '2',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: leaveDeleteInputSchema,
    inputIntentSchema: leaveDeleteInputIntentSchema,
    result: {
      schema: leaveResultSchema,
      effects: [{
        objectType: 'leave',
        effect: ResourceMutationEffect.Deleted,
        resultField: 'leaveID',
        effectIdentity: ResourceEffectIdentity.ID,
      }],
    },
    sideEffect: CapabilitySideEffect.Destructive,
    requiresApproval: true,
    completionEvidence: { mode: 'success', action: 'write_leave', targetKind: 'leave' },
  },
  {
    name: 'leave_decide',
    namespace: 'leave',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_leave',
    policyResource: 'tool:leave_decide',
    description: 'Approve or reject a leave request. Only an administrator may, and the record refuses anybody else. An approval is what spends the entitlement, so it requires approval from the person asking for it.',
    version: '2',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: leaveDecideInputSchema,
    inputIntentSchema: leaveDecideInputIntentSchema,
    result: {
      schema: leaveResultSchema,
      effects: [{
        objectType: 'leave',
        effect: ResourceMutationEffect.Updated,
        resultField: 'leaveID',
        effectIdentity: ResourceEffectIdentity.ID,
      }],
    },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    requiresApproval: true,
    completionEvidence: { mode: 'success', action: 'write_leave', targetKind: 'leave' },
  },
  {
    name: 'leave_grant_set',
    namespace: 'leave',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_leave',
    policyResource: 'tool:leave_grant_set',
    description: "Set how much leave one person is entitled to in a year. Only an administrator may, and the record refuses anybody else. The number replaces every grant they hold, so 'give them three more days' is read with leave_balance first and written as the total. Somebody who has never been given a number of their own holds the company's annual grant, which company_settings_get answers as leaveDays. Requires approval; an entitlement is what every later leave is spent against.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: leaveGrantSetInputSchema,
    inputIntentSchema: leaveGrantSetInputIntentSchema,
    result: { schema: leaveGrantSetResultSchema, effects: [{ objectType: 'leave_grant', effect: ResourceMutationEffect.Updated, resultField: 'personID', effectIdentity: ResourceEffectIdentity.ID }] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    requiresApproval: true,
  },
  {
    name: 'leave_return_early',
    namespace: 'leave',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_leave',
    policyResource: 'tool:leave_return_early',
    description: 'Return to work early, before the leave was due to end. The leave the requester is on stops at this moment, the entitlement it no longer spends comes back to them, and they are clocked in. Use it when somebody who is away says they are back; clocking in on an ordinary day is attendance_add. It is the requester\'s own leave and nobody else\'s. shortened is false when no leave was covering the moment, and they are clocked in either way.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: leaveReturnEarlyInputSchema,
    inputIntentSchema: leaveReturnEarlyInputIntentSchema,
    result: { schema: leaveReturnEarlyResultSchema, effects: [{ objectType: 'leave', effect: ResourceMutationEffect.Updated, resultField: 'leaveID', effectIdentity: ResourceEffectIdentity.ID, when: { resultField: 'shortened', equals: true } }] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: 'success', action: 'write_leave', targetKind: 'leave' },
  },
];

const attendanceToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: 'attendance_team_page_get',
    namespace: 'attendance',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_attendance',
    policyResource: 'tool:attendance_team_page_get',
    description: 'Read one authorized page of team attendance counts or employees for the current company day. Team cards contain true current-state counts, recent recorded clock events and recorded work locations. Employee search and location filtering happen before paging. Historical records are read separately.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: attendanceTeamPageInputSchema,
    result: { schema: attendanceTeamPageSchema.omit({companyName:true,companySummary:true}), effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'attendance_team_dashboard_get',
    namespace: 'attendance',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_attendance',
    policyResource: 'tool:attendance_team_dashboard_get',
    description: 'Read one authorized page of team attendance counts or employees for the current company day. Team cards contain true current-state counts, recent recorded clock events and recorded work locations. Employee search and location filtering happen before paging. Historical records are read separately.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: attendanceTeamPageInputSchema,
    result: { schema: attendanceTeamPageSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'attendance_current_get',
    namespace: 'attendance',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_attendance',
    policyResource: 'tool:attendance_current_get',
    description: 'Read the authenticated requester\'s actionable attendance state in one record snapshot: company time zone and clock, registered work locations, today\'s events, the latest event across all dates, and currently active approved leave. Accepts no member or company selection. This snapshot contains no colleague rows or period totals; use attendance_list for history.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: z.strictObject({}),
    result: { schema: currentAttendanceSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'attendance_list',
    namespace: 'attendance',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_attendance',
    policyResource: 'tool:attendance_list',
    description: "List clock-ins and clock-outs as the record holds them. Use this to answer 'when did I come in', 'was anybody late this week', or to find the record another attendance tool is about to correct. Dates are yyyy-mm-dd and times are 24-hour HH:MM, both in the company time zone. Without from and to it covers the last thirty days. It reads the requester's own attendance unless personHints names other people or scope is all. Each row says whether it was written by hand: reason is what the writer gave, and originalDate with originalTime are the moment it was moved from, both null when it was never moved. handWrittenOnly keeps those rows alone. serverTime is the record's own clock, which is the one to compare a moment against rather than the caller's; backdatedAfterMinutes is how far into the past a moment has to be before writing it counts as writing after the fact.",
    version: '3',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: attendanceListInputSchema,
    result: { schema: legacyAttendanceListResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'attendance_changes_page_get',
    namespace: 'attendance',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_attendance',
    policyResource: 'tool:attendance_changes_page_get',
    description: "List clock-ins and clock-outs as the record holds them. Use this to answer 'when did I come in', 'was anybody late this week', or to find the record another attendance tool is about to correct. Dates are yyyy-mm-dd and times are 24-hour HH:MM, both in the company time zone. Without from and to it covers the last thirty days. It reads the requester's own attendance unless personHints names other people or scope is all. Each row says whether it was written by hand: reason is what the writer gave, and originalDate with originalTime are the moment it was moved from, both null when it was never moved. handWrittenOnly keeps those rows alone. serverTime is the record's own clock, which is the one to compare a moment against rather than the caller's; backdatedAfterMinutes is how far into the past a moment has to be before writing it counts as writing after the fact.",
    version: '1',
    modelVisibility: CapabilityModelVisibility.Hidden,
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: attendanceListInputSchema,
    result: { schema: attendanceListResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'attendance_add',
    namespace: 'attendance',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_attendance',
    policyResource: 'tool:attendance_add',
    description: "Write one attendance record, a clock-in or clock-out, at the moment it actually happened. Omit date and time for right now, which is what somebody clocking in as they arrive means. A person writes their own record from the last three days and it goes in at once, with status added and its eventID. A press right now that reverses the one before it within a minute, clocking back in at the same place or clocking straight out again, takes that earlier record back instead of adding one: status removed, eventID names the record taken back, and that is a finished answer too. Reaching further back is an administrator's to write, for anybody. Asked by anybody else it comes back with status asked and no eventID, and the administrators have already been told what was asked for: that is a finished answer and the task is done. Say an administrator was asked and leave it there; do not call this again. A moment in the future is refused. location names a work location this company has registered and clock_out does not use it. reason says why a record is being written by hand, and is kept when given, never demanded.",
    version: '4',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: attendanceAddInputSchema,
    inputIntentSchema: attendanceAddInputIntentSchema,
    result: { schema: attendanceAddResultSchema, effects: [{ objectType: 'attendance', effect: ResourceMutationEffect.Created, resultField: 'eventID', effectIdentity: ResourceEffectIdentity.ID, when: { resultField: 'status', equals: 'added' }  }, { objectType: 'attendance', effect: ResourceMutationEffect.Deleted, resultField: 'eventID', effectIdentity: ResourceEffectIdentity.ID, when: { resultField: 'status', equals: 'removed' } }] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
  },
  {
    name: 'attendance_update',
    namespace: 'attendance',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_attendance',
    policyResource: 'tool:attendance_update',
    description: 'Correct the day, the time, or the work location of attendance records that were written wrong, all in one call. What the record held before the correction is kept alongside it, with the reason. A person corrects their own records from the last three days. Correcting an older one, or anybody else\'s, is an administrator\'s: asked by anybody else it comes back with status asked, the administrators have been told, and the task is done. Say an administrator was asked; do not call this again.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: attendanceUpdateInputSchema,
    inputIntentSchema: attendanceUpdateInputIntentSchema,
    result: { schema: attendanceWriteResultSchema, effects: [{ objectType: 'attendance', effect: ResourceMutationEffect.Updated, resultField: 'eventID', effectIdentity: ResourceEffectIdentity.ID, when: { resultField: 'status', equals: 'corrected' } }] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    requiresApproval: true,
  },
  {
    name: 'attendance_delete',
    namespace: 'attendance',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_attendance',
    policyResource: 'tool:attendance_delete',
    description: 'Remove an attendance record that should never have been there. The record stops counting, and who removed it and why stays in the record. A person removes their own records from the last three days. Removing an older one, or anybody else\'s, is an administrator\'s: asked by anybody else it comes back with status asked, the administrators have been told, and the task is done. Say an administrator was asked; do not call this again.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: attendanceDeleteInputSchema,
    inputIntentSchema: attendanceDeleteInputIntentSchema,
    result: { schema: attendanceWriteResultSchema, effects: [{ objectType: 'attendance', effect: ResourceMutationEffect.Deleted, resultField: 'eventID', effectIdentity: ResourceEffectIdentity.ID, when: { resultField: 'status', equals: 'removed' } }] },
    sideEffect: CapabilitySideEffect.Destructive,
    requiresApproval: true,
  },
];

const capabilityToolDefinitions: CapabilityToolDefinition[] = [
  ...scheduleToolDefinitions,
  ...taskToolDefinitions,
  ...peopleToolDefinitions,
  ...calendarToolDefinitions,
  ...leaveToolDefinitions,
  ...attendanceToolDefinitions,
  ...messageToolDefinitions,
  ...webToolDefinitions,
  ...fileToolDefinitions,
  ...browserToolDefinitions,
  ...browserControlToolDefinitions,
  ...artifactToolDefinitions,
  ...companyToolDefinitions,
  ...dataRoomToolDefinitions,
  ...circleToolDefinitions,
  ...crmToolDefinitions,
  ...settingsToolDefinitions,
  ...notificationToolDefinitions,
  ...mailToolDefinitions,
  ...hostToolDefinitions,
];

export type TaskAddInput = z.infer<typeof taskAddInputSchema>;
export type TaskListInput = z.infer<typeof taskListInputSchema>;
export type TaskUpdateInput = z.infer<typeof taskUpdateInputSchema>;
export type TaskDeleteInput = z.infer<typeof taskDeleteInputSchema>;
export type TaskResult = z.infer<typeof taskResultSchema>;
export type TaskVocabularySetInput = z.infer<typeof taskVocabularySetInputSchema>;
export type TaskLabelVocabulary = z.infer<typeof taskLabelVocabularySchema>;
export type LeaveListInput = z.infer<typeof leaveListInputSchema>;
export type LeaveBalanceInput = z.infer<typeof leaveBalanceInputSchema>;
export type LeaveRequestInput = z.infer<typeof leaveRequestInputSchema>;
export type LeaveDecideInput = z.infer<typeof leaveDecideInputSchema>;
export type LeaveResult = z.infer<typeof leaveResultSchema>;
export type LeaveListResult = z.infer<typeof leaveListResultSchema>;
export type LeaveBalanceResult = z.infer<typeof leaveBalanceResultSchema>;
export type AttendanceListInput = z.infer<typeof attendanceListInputSchema>;
export type AttendanceAddInput = z.infer<typeof attendanceAddInputSchema>;
export type AttendanceUpdateInput = z.infer<typeof attendanceUpdateInputSchema>;
export type AttendanceDeleteInput = z.infer<typeof attendanceDeleteInputSchema>;
export type AttendanceResult = z.infer<typeof attendanceResultSchema>;
export type AttendanceListResult = z.infer<typeof attendanceListResultSchema>;
export type AttendanceWriteResult = z.infer<typeof attendanceWriteResultSchema>;
export type CalendarAddInput = z.infer<typeof calendarAddInputSchema>;
export type CalendarListInput = z.infer<typeof calendarListInputSchema>;
export type CalendarUpdateInput = z.infer<typeof calendarUpdateInputSchema>;
export type CalendarDeleteInput = z.infer<typeof calendarDeleteInputSchema>;
export type CalendarEventResult = z.infer<typeof calendarEventResultSchema>;
export type CalendarEntryResult = z.infer<typeof calendarEntryResultSchema>;
export type MessageContextInput = z.infer<typeof messageContextInputSchema>;
export type MessageSearchInput = z.infer<typeof messageSearchInputSchema>;
export type MessageSendInput = z.infer<typeof messageSendInputSchema>;
export type MessageUpdateInput = z.infer<typeof messageUpdateInputSchema>;
export type MessageDeleteInput = z.infer<typeof messageDeleteInputSchema>;
export type MessageContextResult = z.infer<typeof messageContextResultSchema>;
export type MessageSearchResult = z.infer<typeof messageSearchResultSchema>;
export type MessageSendResult = z.infer<typeof messageSendResultSchema>;
export type MessageUpdateResult = z.infer<typeof messageUpdateResultSchema>;
export type MessageDeleteResult = z.infer<typeof messageDeleteResultSchema>;
export type DocumentReadInput = z.infer<typeof documentReadInputSchema>;
export type DocumentReadResult = z.infer<typeof documentReadResultSchema>;
export type ImageReadInput = z.infer<typeof imageReadInputSchema>;
export type ImageReadResult = z.infer<typeof imageReadResultSchema>;
export type BrowserOpenInput = z.infer<typeof browserOpenInputSchema>;
export type BrowserOpenResult = z.infer<typeof browserOpenResultSchema>;
export type BrowserSnapshotInput = z.infer<typeof browserSnapshotInputSchema>;
export type BrowserSnapshotResult = z.infer<typeof browserSnapshotResultSchema>;
export type BrowserScreenshotInput = z.infer<typeof browserScreenshotInputSchema>;
export type BrowserScreenshotResult = z.infer<typeof browserScreenshotResultSchema>;
export type BrowserClickInput = z.infer<typeof browserClickInputSchema>;
export type BrowserClickResult = z.infer<typeof browserClickResultSchema>;
export type ArtifactReviewInput = z.infer<typeof artifactReviewInputSchema>;
export type ArtifactReviewResult = z.infer<typeof artifactReviewResultSchema>;
export type WebSearchInput = z.infer<typeof webSearchInputSchema>;
export type WebSearchResult = z.infer<typeof webSearchResultSchema>;

export function buildCapabilityToolCatalog(protocolVersion: string): CapabilityToolCatalog {
  return buildCapabilityCatalog(protocolVersion, capabilityToolDefinitions);
}

export function capabilityToolInputSchema(name: string): z.ZodType | undefined {
  return capabilityToolDefinitions.find((definition) => definition.name === name)?.inputSchema;
}

export function capabilityToolResultSchema(name: string): z.ZodType | undefined {
  return capabilityToolDefinitions.find((definition) => definition.name === name)?.result?.schema;
}
