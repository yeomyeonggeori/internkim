import { z } from 'zod';

import { crmStageKeys } from '$lib/crm/crm-stage';

import {
  CapabilityAnsweredBy,
  CapabilityEstimatedLatency,
  CapabilitySideEffect,
} from './protocol';

import { taskLabelVocabularySchema } from './labels';
import { WorkspaceTaskStatus } from './workspace-task';

import type { CapabilityToolDefinition } from './definition';

const resourceIDSchema = z.string()
  .min(1)
  .regex(/^\S(?:.*\S)?$/, 'Resource identity must not have leading or trailing whitespace.');

const dayDescription = 'A day as yyyy-mm-dd.';
const momentDescription = 'ISO 8601 with a time zone for a moment, or yyyy-mm-dd for a whole day.';
const amountDescription = "What it is worth, in the currency's smallest unit: 15000000 is ₩15,000,000.";

const organizationHintSchema = z.string().min(1).max(256).describe(
  'The organization: its exact ID, or its name as crm_organization_list shows it.',
);

const contactHintSchema = z.string().min(1).max(256).describe(
  'The person: their exact contact ID, their email address, or their name as crm_contact_list shows it.',
);

const opportunityHintSchema = z.string().min(1).max(256).describe(
  'The deal: its exact opportunity ID, or its exact CURRENT title from crm_opportunity_list. Never a new title.',
);

export const crmStageSchema = z.enum(crmStageKeys);

const ownerHintDescription = 'The colleague who owns it, by name or email.';
const importanceDescription = 'high, medium or low.';

const organizationFields = {
  name: z.string().min(1).max(256).describe("The organization's name."),
  types: z.array(z.string()).describe('Types from crm_vocabulary_get, e.g. customer. Replaces the whole list.').optional(),
  status: z.string().max(32).describe('prospect, active or paused.').optional(),
  importance: z.string().max(32).describe(importanceDescription).optional(),
  tags: z.array(z.string()).describe('Replaces the whole list.').optional(),
  ownerPersonHint: z.string().max(256).describe(ownerHintDescription).optional(),
  address: z.string().max(512).optional(),
  description: z.string().max(4096).describe('What to remember about them.').optional(),
};

const contactFields = {
  name: z.string().min(1).max(256).describe("The person's name."),
  email: z.string().max(256).optional(),
  phoneNumber: z.string().max(64).optional(),
  role: z.string().max(128).describe("Their title, e.g. 'purchasing manager'.").optional(),
  department: z.string().max(128).optional(),
  description: z.string().max(4096).describe('What to remember about them.').optional(),
};

const opportunityFields = {
  title: z.string().min(1).max(256).describe('What the deal is called.'),
  pipeline: z.string().max(128).describe('The pipeline from crm_vocabulary_get. Omit when the company runs one.').optional(),
  business: z.string().max(128).describe('Business label from a task_list result.').optional(),
  amountMinor: z.number().int().describe(amountDescription).optional(),
  currencyCode: z.string().length(3).describe("ISO 4217, e.g. 'KRW'.").optional(),
  contactHint: contactHintSchema.optional(),
  ownerPersonHint: z.string().max(256).describe(ownerHintDescription).optional(),
  importance: z.string().max(32).describe(importanceDescription).optional(),
  expectedCloseDate: z.string().describe(`When it should close, in the company's time zone. ${dayDescription}`).optional(),
  description: z.string().max(4096).describe('What to remember about the deal.').optional(),
};

export const crmOrganizationListInputSchema = z.strictObject({
  query: z.string().max(256).describe('Filter on the name, the address and the description.').optional(),
  includeArchived: z.boolean().describe('Set true to include archived ones.').optional(),
});

export const crmOrganizationAddInputSchema = z.strictObject(organizationFields);

export const crmOrganizationAddInputIntentSchema = crmOrganizationAddInputSchema.partial();

const crmOrganizationUpdateObjectSchema = z.strictObject({
  organizationHint: organizationHintSchema,
  ...organizationFields,
  name: organizationFields.name.optional(),
});

export const crmOrganizationUpdateInputSchema = crmOrganizationUpdateObjectSchema
  .refine(hasSomethingToWrite(['organizationHint']), 'At least one organization field must be updated.')
  .meta({ minProperties: 2 });

export const crmOrganizationUpdateInputIntentSchema = crmOrganizationUpdateObjectSchema.omit({
  organizationHint: true,
});

export const crmOrganizationArchiveInputSchema = z.strictObject({
  organizationHint: organizationHintSchema,
});

export const crmOrganizationArchiveInputIntentSchema = z.strictObject({});

export const crmContactListInputSchema = z.strictObject({
  organizationHint: organizationHintSchema.describe('Only the people at this organization.').optional(),
  query: z.string().max(256).describe('Filter on the name, the email address and the title.').optional(),
  includeArchived: z.boolean().describe('Set true to include archived ones.').optional(),
});

export const crmContactAddInputSchema = z.strictObject({
  organizationHint: organizationHintSchema,
  ...contactFields,
});

export const crmContactAddInputIntentSchema = crmContactAddInputSchema.partial();

const crmContactUpdateObjectSchema = z.strictObject({
  contactHint: contactHintSchema,
  organizationHint: organizationHintSchema.describe('Where they work now.').optional(),
  ...contactFields,
  name: contactFields.name.optional(),
});

export const crmContactUpdateInputSchema = crmContactUpdateObjectSchema
  .refine(hasSomethingToWrite(['contactHint']), 'At least one contact field must be updated.')
  .meta({ minProperties: 2 });

export const crmContactUpdateInputIntentSchema = crmContactUpdateObjectSchema.omit({ contactHint: true });

export const crmContactArchiveInputSchema = z.strictObject({
  contactHint: contactHintSchema,
});

export const crmContactArchiveInputIntentSchema = z.strictObject({});

export const crmOpportunityListInputSchema = z.strictObject({
  organizationHint: organizationHintSchema.describe('Only the deals with this organization.').optional(),
  stage: crmStageSchema.describe('Only the deals at this stage.').optional(),
  query: z.string().max(256).describe('Filter on the title and the description.').optional(),
  includeArchived: z.boolean().describe('Set true to include archived ones.').optional(),
});

export const crmOpportunityAddInputSchema = z.strictObject({
  organizationHint: organizationHintSchema,
  ...opportunityFields,
  stage: crmStageSchema.describe('Where it starts. Defaults to waiting; a closing stage is refused here.').optional(),
});

export const crmOpportunityAddInputIntentSchema = crmOpportunityAddInputSchema.partial();

const crmOpportunityUpdateObjectSchema = z.strictObject({
  opportunityHint: opportunityHintSchema,
  organizationHint: organizationHintSchema.describe('The organization it is with now.').optional(),
  ...opportunityFields,
  title: opportunityFields.title.optional(),
});

export const crmOpportunityUpdateInputSchema = crmOpportunityUpdateObjectSchema
  .refine(hasSomethingToWrite(['opportunityHint']), 'At least one opportunity field must be updated.')
  .meta({ minProperties: 2 });

export const crmOpportunityUpdateInputIntentSchema = crmOpportunityUpdateObjectSchema.omit({
  opportunityHint: true,
});

const crmOpportunityMoveObjectSchema = z.strictObject({
  opportunityHint: opportunityHintSchema,
  stage: crmStageSchema.describe('Where it moves to. Omit to only change its position.').optional(),
  position: z.number().int().min(0).describe('Where it sits at that stage, counting from 0.').optional(),
  closedAt: z.string().describe(`When the move happened. ${momentDescription} Defaults to now.`).optional(),
  finalAmountMinor: z.number().int().describe("What it was worth in the company's own currency, which the record converts when the call does not.").optional(),
  reason: z.string().max(1024).describe('Why it was lost. A move into lost is refused without one.').optional(),
});

export const crmOpportunityMoveInputSchema = crmOpportunityMoveObjectSchema
  .refine(hasSomethingToWrite(['opportunityHint']), 'A move names the stage or the position it moves the deal to.')
  .meta({ minProperties: 2 });

export const crmOpportunityMoveInputIntentSchema = crmOpportunityMoveObjectSchema.omit({
  opportunityHint: true,
});

export const crmOpportunityArchiveInputSchema = z.strictObject({
  opportunityHint: opportunityHintSchema,
});

export const crmOpportunityArchiveInputIntentSchema = z.strictObject({});

export const crmVocabularyGetInputSchema = z.strictObject({});

const crmDefinitionSchema = z.strictObject({
  id: z.string().min(1).max(64).describe('The stable key the records carry.'),
  name: z.string().min(1).max(120).describe('What people call it.'),
  color: z.string().max(32).describe('The colour the CRM screens draw it in, as a CSS colour.').optional(),
});

const crmPipelineDefinitionSchema = z.strictObject({
  ...crmDefinitionSchema.shape,
  direction: z.string().max(64).describe('Which way the pipeline runs, e.g. inbound or outbound.').optional(),
});

export const crmVocabularySetInputSchema = z.strictObject({
  organizationTypes: z.array(crmDefinitionSchema).describe('Every organization type this company names, replacing the whole list. A type still carried by an organization cannot be dropped.'),
  pipelines: z.array(crmPipelineDefinitionSchema).describe('Every pipeline this company runs deals through, replacing the whole list. A pipeline still carried by a deal cannot be dropped.'),
});

export const crmVocabularySetInputIntentSchema = crmVocabularySetInputSchema.partial();

const activityHintSchema = z.string().min(1).max(256).describe(
  'The activity: its exact task ID, or its exact CURRENT title as crm_activity_list shows it.',
);

export const crmActivityListInputSchema = z.strictObject({
  organizationHint: organizationHintSchema.describe('Only the work with this organization.').optional(),
  opportunityHint: opportunityHintSchema.describe('Only the work on this deal.').optional(),
  query: z.string().max(256).describe('Free-text filter matched against the title and the note.').optional(),
});

export const crmActivitySaveInputSchema = z.strictObject({
  activityHint: activityHintSchema.describe('The activity to change. Omit to record a new one.').optional(),
  organizationHint: organizationHintSchema.describe('The organization the work is with. A new activity names one.').optional(),
  opportunityHint: opportunityHintSchema.describe('The deal the work belongs to.').optional(),
  contactHint: contactHintSchema.describe('The person the work was with.').optional(),
  title: z.string().min(1).max(256).describe('What happened, in a line.').optional(),
  kind: z.string().max(64).describe('The kind of work it was: a note, a call, a meeting, an email, a task.').optional(),
  business: z.string().max(128).describe('Business label, from registeredLabels.businesses.').optional(),
  note: z.string().max(8192).describe('What was said or decided.').optional(),
  status: z.enum(WorkspaceTaskStatus).describe('The status the task stands at.').optional(),
  occurredAt: z.string().describe(`When it happened. ${momentDescription}`).optional(),
  participantPersonHints: z
    .array(z.string().max(256))
    .describe('Names, @handles or emails of the colleagues taking part. A task has participants, not one assignee.')
    .optional(),
  ownerPersonHint: z
    .string()
    .max(256)
    .describe('Deprecated alias for a single entry in participantPersonHints.')
    .optional(),
  isEvent: z.boolean().describe('Set true for work that goes in the calendar, which then needs startsAt and endsAt.').optional(),
  isWholeDay: z.boolean().describe('Whether the calendar entry takes the whole day.').optional(),
  startsAt: z.string().describe(`When the calendar entry starts. ${momentDescription}`).optional(),
  endsAt: z.string().describe(`When the calendar entry ends. ${momentDescription}`).optional(),
  location: z.string().max(256).describe('Where it happens.').optional(),
  notifyMinutesBefore: z.number().int().min(0).describe('Minutes before the start to remind the people in it.').optional(),
});

export const crmActivitySaveInputIntentSchema = crmActivitySaveInputSchema.omit({ activityHint: true });

export const crmActivityResultSchema = z.strictObject({
  activityID: resourceIDSchema,
  organizationID: z.string(),
  opportunityID: z.string(),
  contactID: z.string(),
  business: z.string(),
  kind: z.string(),
  title: z.string(),
  occurredAt: z.string(),
  content: z.string(),
  taskStatus: z.string(),
  size: z.string(),
  participantIDs: z.array(z.string()).describe('Everyone taking part, in no particular order.'),
  ownerPersonID: z.string().describe('One of the participants, kept for callers written before participantIDs. Read participantIDs instead.'),
  requesterPersonID: z.string(),
  isEvent: z.boolean(),
  isWholeDay: z.boolean(),
  startsAt: z.string(),
  endsAt: z.string(),
  notifyMinutesBefore: z.number().int().nullable(),
  location: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export const crmActivityListResultSchema = z.strictObject({
  count: z.number().int(),
  activities: z.array(crmActivityResultSchema),
  registeredLabels: taskLabelVocabularySchema,
});

const crmAuditResultSchema = z.strictObject({
  createdAt: z.string(),
  createdByPersonID: z.string(),
  updatedAt: z.string(),
  updatedByPersonID: z.string(),
  archivedAt: z.string().nullable(),
  archivedByPersonID: z.string(),
});

export const crmOrganizationResultSchema = z.strictObject({
  organizationID: resourceIDSchema,
  name: z.string(),
  status: z.string(),
  types: z.array(z.string()),
  tags: z.array(z.string()),
  importance: z.string(),
  ownerPersonID: z.string(),
  address: z.string(),
  description: z.string(),
  audit: crmAuditResultSchema,
});

export const crmOrganizationListResultSchema = z.strictObject({
  count: z.number().int(),
  organizations: z.array(crmOrganizationResultSchema),
});

export const crmContactResultSchema = z.strictObject({
  contactID: resourceIDSchema,
  organizationID: z.string(),
  name: z.string(),
  email: z.string(),
  phoneNumber: z.string(),
  role: z.string(),
  department: z.string(),
  description: z.string(),
  audit: crmAuditResultSchema,
});

export const crmContactListResultSchema = z.strictObject({
  count: z.number().int(),
  contacts: z.array(crmContactResultSchema),
});

export const crmOpportunityResultSchema = z.strictObject({
  opportunityID: resourceIDSchema,
  organizationID: z.string(),
  contactID: z.string(),
  title: z.string(),
  business: z.string(),
  pipeline: z.string(),
  stage: z.string(),
  stagePosition: z.number().int(),
  stageChangedAt: z.string(),
  ownerPersonID: z.string(),
  amountMinor: z.number().int().nullable(),
  currencyCode: z.string(),
  baseAmountMinor: z.number().int().nullable(),
  baseCurrencyCode: z.string(),
  importance: z.string(),
  expectedCloseAt: z.string(),
  expectedCloseTimeZone: z.string(),
  lostReason: z.string(),
  description: z.string(),
  activityCount: z.number().int(),
  audit: crmAuditResultSchema,
});

export const crmOpportunityListResultSchema = z.strictObject({
  count: z.number().int(),
  opportunities: z.array(crmOpportunityResultSchema),
});

export const crmArchivedResultSchema = z.strictObject({
  recordID: resourceIDSchema,
  name: z.string(),
  archivedAt: z.string(),
});

export const crmVocabularyResultSchema = z.strictObject({
  organizationTypes: z.array(z.strictObject({
    id: z.string(),
    name: z.string(),
    color: z.string().optional(),
  })),
  pipelines: z.array(z.strictObject({
    id: z.string(),
    name: z.string(),
    color: z.string().optional(),
    direction: z.string().optional(),
  })),
  stages: z.array(z.strictObject({
    stage: z.string(),
    outcome: z.string(),
  })),
});

function hasSomethingToWrite(hintFields: string[]): (document: object) => boolean {
  return (document) => Object.keys(document).some((name) => !hintFields.includes(name));
}

export const crmToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: 'crm_organization_list',
    namespace: 'crm',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_crm',
    policyResource: 'tool:crm_organization_list',
    description: "The organizations this company sells to, partners with or buys from, with who owns each relationship — 'who are our customers'.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: crmOrganizationListInputSchema,
    result: { schema: crmOrganizationListResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'crm_organization_add',
    namespace: 'crm',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_crm',
    policyResource: 'tool:crm_organization_add',
    description: 'Add an organization this company deals with, after crm_organization_list shows it is not one already held.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: crmOrganizationAddInputSchema,
    inputIntentSchema: crmOrganizationAddInputIntentSchema,
    result: { schema: crmOrganizationResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: 'success', action: 'write_crm', targetKind: 'crm_organization' },
  },
  {
    name: 'crm_organization_update',
    namespace: 'crm',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_crm',
    policyResource: 'tool:crm_organization_update',
    description: 'Change what this company holds about an organization; only the fields the call names change.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: crmOrganizationUpdateInputSchema,
    inputIntentSchema: crmOrganizationUpdateInputIntentSchema,
    result: { schema: crmOrganizationResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: 'success', action: 'write_crm', targetKind: 'crm_organization' },
  },
  {
    name: 'crm_organization_archive',
    namespace: 'crm',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_crm',
    policyResource: 'tool:crm_organization_archive',
    description: 'Put an organization away, so it leaves the CRM screens while its people, its deals and everything recorded against it stay.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: crmOrganizationArchiveInputSchema,
    inputIntentSchema: crmOrganizationArchiveInputIntentSchema,
    result: { schema: crmArchivedResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Destructive,
    requiresApproval: true,
    completionEvidence: { mode: 'success', action: 'write_crm', targetKind: 'crm_organization' },
  },
  {
    name: 'crm_contact_list',
    namespace: 'crm',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_crm',
    policyResource: 'tool:crm_contact_list',
    description: "The people this company deals with at other organizations, with how to reach them — 'who do we talk to at ABC'. Colleagues are person_list's.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: crmContactListInputSchema,
    result: { schema: crmContactListResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'crm_contact_add',
    namespace: 'crm',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_crm',
    policyResource: 'tool:crm_contact_add',
    description: 'Add a person at an organization this company already holds, which crm_organization_add creates when it does not.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: crmContactAddInputSchema,
    inputIntentSchema: crmContactAddInputIntentSchema,
    result: { schema: crmContactResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: 'success', action: 'write_crm', targetKind: 'crm_contact' },
  },
  {
    name: 'crm_contact_update',
    namespace: 'crm',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_crm',
    policyResource: 'tool:crm_contact_update',
    description: 'Change what this company holds about a person, including moving them to another organization; only the fields the call names change.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: crmContactUpdateInputSchema,
    inputIntentSchema: crmContactUpdateInputIntentSchema,
    result: { schema: crmContactResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: 'success', action: 'write_crm', targetKind: 'crm_contact' },
  },
  {
    name: 'crm_contact_archive',
    namespace: 'crm',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_crm',
    policyResource: 'tool:crm_contact_archive',
    description: 'Put a person away, so they leave the CRM screens while everything recorded against them stays, as when somebody leaves the organization.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: crmContactArchiveInputSchema,
    inputIntentSchema: crmContactArchiveInputIntentSchema,
    result: { schema: crmArchivedResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Destructive,
    requiresApproval: true,
    completionEvidence: { mode: 'success', action: 'write_crm', targetKind: 'crm_contact' },
  },
  {
    name: 'crm_opportunity_list',
    namespace: 'crm',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_crm',
    policyResource: 'tool:crm_opportunity_list',
    description: "The deals this company is working on, each with its stage, what it is worth and how much work has been recorded against it — 'what are we closing this month'.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: crmOpportunityListInputSchema,
    result: { schema: crmOpportunityListResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'crm_opportunity_add',
    namespace: 'crm',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_crm',
    policyResource: 'tool:crm_opportunity_add',
    description: 'Open a deal with an organization this company already holds. A deal already won or lost is opened here and then closed with crm_opportunity_move.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: crmOpportunityAddInputSchema,
    inputIntentSchema: crmOpportunityAddInputIntentSchema,
    result: { schema: crmOpportunityResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: 'success', action: 'write_crm', targetKind: 'crm_opportunity' },
  },
  {
    name: 'crm_opportunity_update',
    namespace: 'crm',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_crm',
    policyResource: 'tool:crm_opportunity_update',
    description: 'Change what a deal is worth, who it is with, or when it should close; only the fields the call names change. Its stage is crm_opportunity_move\u2019s.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: crmOpportunityUpdateInputSchema,
    inputIntentSchema: crmOpportunityUpdateInputIntentSchema,
    result: { schema: crmOpportunityResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: 'success', action: 'write_crm', targetKind: 'crm_opportunity' },
  },
  {
    name: 'crm_opportunity_move',
    namespace: 'crm',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_crm',
    policyResource: 'tool:crm_opportunity_move',
    description: "Move a deal to another stage, or reorder it within the one it stands at. A move into done or lost settles what it was worth and cannot be undone, so put it to the requester first.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: crmOpportunityMoveInputSchema,
    inputIntentSchema: crmOpportunityMoveInputIntentSchema,
    result: { schema: crmOpportunityResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    requiresApproval: true,
    completionEvidence: { mode: 'success', action: 'write_crm', targetKind: 'crm_opportunity' },
  },
  {
    name: 'crm_opportunity_archive',
    namespace: 'crm',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_crm',
    policyResource: 'tool:crm_opportunity_archive',
    description: 'Put a deal away, so it leaves the pipeline while everything recorded against it stays. A deal that was lost is moved to lost instead.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: crmOpportunityArchiveInputSchema,
    inputIntentSchema: crmOpportunityArchiveInputIntentSchema,
    result: { schema: crmArchivedResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Destructive,
    requiresApproval: true,
    completionEvidence: { mode: 'success', action: 'write_crm', targetKind: 'crm_opportunity' },
  },
  {
    name: 'crm_activity_list',
    namespace: 'crm',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_crm',
    policyResource: 'tool:crm_activity_list',
    description: 'The work recorded against the organizations and deals this company holds, newest first, with the labels and colours the CRM screens draw it in. The same rows task_list answers, shaped for the CRM.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: crmActivityListInputSchema,
    result: { schema: crmActivityListResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'crm_activity_save',
    namespace: 'crm',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_crm',
    policyResource: 'tool:crm_activity_save',
    description: 'Record work against an organization or a deal, or change work already recorded. An activity is a task, so this writes what task_add and task_update write, plus the calendar entry when the work goes in the calendar.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: crmActivitySaveInputSchema,
    inputIntentSchema: crmActivitySaveInputIntentSchema,
    result: { schema: crmActivityResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: 'success', action: 'write_crm', targetKind: 'crm_activity' },
  },
  {
    name: 'crm_vocabulary_get',
    namespace: 'crm',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_crm',
    policyResource: 'tool:crm_vocabulary_get',
    description: 'The words this company runs its CRM in: the organization types it sorts relationships by, the pipelines it runs deals through, and the stages a deal can stand at with what each one means.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: crmVocabularyGetInputSchema,
    result: { schema: crmVocabularyResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'crm_vocabulary_set',
    namespace: 'crm',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_crm',
    policyResource: 'tool:crm_vocabulary_set',
    description: 'Set the organization types and pipelines this company runs its CRM in. Both lists are written at once, so read crm_vocabulary_get first and send it back with what changes. The stages are the product’s and are not set here. This is an administrator’s.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: crmVocabularySetInputSchema,
    inputIntentSchema: crmVocabularySetInputIntentSchema,
    result: { schema: crmVocabularyResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: 'success', action: 'write_crm', targetKind: 'crm_vocabulary' },
  },
];

export type CRMOrganizationListInput = z.infer<typeof crmOrganizationListInputSchema>;
export type CRMOrganizationAddInput = z.infer<typeof crmOrganizationAddInputSchema>;
export type CRMOrganizationUpdateInput = z.infer<typeof crmOrganizationUpdateInputSchema>;
export type CRMOrganizationResult = z.infer<typeof crmOrganizationResultSchema>;
export type CRMOrganizationListResult = z.infer<typeof crmOrganizationListResultSchema>;
export type CRMContactListInput = z.infer<typeof crmContactListInputSchema>;
export type CRMContactAddInput = z.infer<typeof crmContactAddInputSchema>;
export type CRMContactUpdateInput = z.infer<typeof crmContactUpdateInputSchema>;
export type CRMContactResult = z.infer<typeof crmContactResultSchema>;
export type CRMContactListResult = z.infer<typeof crmContactListResultSchema>;
export type CRMOpportunityListInput = z.infer<typeof crmOpportunityListInputSchema>;
export type CRMOpportunityAddInput = z.infer<typeof crmOpportunityAddInputSchema>;
export type CRMOpportunityUpdateInput = z.infer<typeof crmOpportunityUpdateInputSchema>;
export type CRMOpportunityMoveInput = z.infer<typeof crmOpportunityMoveInputSchema>;
export type CRMOpportunityResult = z.infer<typeof crmOpportunityResultSchema>;
export type CRMOpportunityListResult = z.infer<typeof crmOpportunityListResultSchema>;
export type CRMArchivedResult = z.infer<typeof crmArchivedResultSchema>;
export type CRMVocabularySetInput = z.infer<typeof crmVocabularySetInputSchema>;
export type CRMVocabularyResult = z.infer<typeof crmVocabularyResultSchema>;
export type CRMActivityListInput = z.infer<typeof crmActivityListInputSchema>;
export type CRMActivitySaveInput = z.infer<typeof crmActivitySaveInputSchema>;
export type CRMActivityResult = z.infer<typeof crmActivityResultSchema>;
export type CRMActivityListResult = z.infer<typeof crmActivityListResultSchema>;
