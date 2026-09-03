import { z } from 'zod';

import { crmStageKeys } from '$lib/crm/crm-stage';

import {
  CapabilityAnsweredBy,
  CapabilityEstimatedLatency,
  CapabilityModelVisibility,
  CapabilitySideEffect,
} from './protocol';

import type { CapabilityToolDefinition } from './definition';

const resourceIDSchema = z.string()
  .min(1)
  .regex(/^\S(?:.*\S)?$/, 'Resource identity must not have leading or trailing whitespace.');

const dayDescription = 'A day as yyyy-mm-dd.';
const momentDescription = 'ISO 8601 with a time zone for a moment, or yyyy-mm-dd for a whole day.';
const amountDescription = "The amount in the currency's smallest unit: 15000000 is ₩15,000,000 and 120000 is $1,200.00.";

const organizationHintSchema = z.string().min(1).max(256).describe(
  'The organization: its exact ID, or its name as crm_organization_list shows it. A name several carry fails with the candidates.',
);

const contactHintSchema = z.string().min(1).max(256).describe(
  'The person: their exact contact ID, their email address, or their name as crm_contact_list shows it. A name several carry fails with the candidates.',
);

const opportunityHintSchema = z.string().min(1).max(256).describe(
  'The deal: its exact opportunity ID, or its exact CURRENT title as crm_opportunity_list shows it. Never a new or intended title.',
);

export const crmStageSchema = z.enum(crmStageKeys);

const ownerHintDescription = 'Name, @handle or email of the colleague who owns it.';
const importanceDescription = 'high, medium or low.';

const organizationFields = {
  name: z.string().min(1).max(256).describe("The organization's name."),
  types: z.array(z.string()).describe('Organization types from crm_vocabulary_get, replacing the whole list.').optional(),
  status: z.string().max(32).describe('prospect, active or paused.').optional(),
  importance: z.string().max(32).describe(importanceDescription).optional(),
  tags: z.array(z.string()).describe('Free-text tags, replacing the whole list.').optional(),
  ownerPersonHint: z.string().max(256).describe(ownerHintDescription).optional(),
  address: z.string().max(512).optional(),
  description: z.string().max(4096).describe('What this company should remember about them.').optional(),
};

const contactFields = {
  name: z.string().min(1).max(256).describe("The person's name."),
  email: z.string().max(256).optional(),
  phoneNumber: z.string().max(64).optional(),
  role: z.string().max(128).describe("Their title where they work, e.g. 'purchasing manager'.").optional(),
  department: z.string().max(128).describe('The team they work in.').optional(),
  description: z.string().max(4096).describe('What this company should remember about them.').optional(),
};

const opportunityFields = {
  title: z.string().min(1).max(256).describe('What the deal is called.'),
  pipeline: z.string().max(128).describe('The pipeline it runs in, from crm_vocabulary_get. Omit to use the only one registered.').optional(),
  business: z.string().max(128).describe('Business label, from registeredLabels.businesses in a task_list result.').optional(),
  amountMinor: z.number().int().describe(`What it is worth. ${amountDescription}`).optional(),
  currencyCode: z.string().length(3).describe("ISO 4217 code, e.g. 'KRW'. Defaults to the company's.").optional(),
  contactHint: contactHintSchema.optional(),
  ownerPersonHint: z.string().max(256).describe(ownerHintDescription).optional(),
  importance: z.string().max(32).describe(importanceDescription).optional(),
  expectedCloseDate: z.string().describe(`When it is expected to close, in the company's time zone. ${dayDescription}`).optional(),
  description: z.string().max(4096).describe('What this company should remember about the deal.').optional(),
};

export const crmOrganizationListInputSchema = z.strictObject({
  query: z.string().max(256).describe('Free-text filter matched against the name, the address and the description.').optional(),
  includeArchived: z.boolean().describe('Set true to include organizations somebody archived.').optional(),
});

export const crmOrganizationAddInputSchema = z.strictObject(organizationFields);

export const crmOrganizationAddInputIntentSchema = crmOrganizationAddInputSchema.partial();

const crmOrganizationUpdateObjectSchema = z.strictObject({
  organizationHint: organizationHintSchema,
  ...organizationFields,
  name: organizationFields.name.describe('The name it should carry now.').optional(),
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
  organizationHint: organizationHintSchema.describe('Only the people at this organization, named the way crm_organization_list shows it.').optional(),
  query: z.string().max(256).describe('Free-text filter matched against the name, the email address and the title.').optional(),
  includeArchived: z.boolean().describe('Set true to include people somebody archived.').optional(),
});

export const crmContactAddInputSchema = z.strictObject({
  organizationHint: organizationHintSchema,
  ...contactFields,
});

export const crmContactAddInputIntentSchema = crmContactAddInputSchema.partial();

const crmContactUpdateObjectSchema = z.strictObject({
  contactHint: contactHintSchema,
  organizationHint: organizationHintSchema.describe('Where they work now, named the way crm_organization_list shows it.').optional(),
  ...contactFields,
  name: contactFields.name.describe('The name they should carry now.').optional(),
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
  organizationHint: organizationHintSchema.describe('Only the deals with this organization, named the way crm_organization_list shows it.').optional(),
  stage: crmStageSchema.describe('Only the deals at this stage.').optional(),
  query: z.string().max(256).describe('Free-text filter matched against the title and the description.').optional(),
  includeArchived: z.boolean().describe('Set true to include deals somebody archived.').optional(),
});

export const crmOpportunityAddInputSchema = z.strictObject({
  organizationHint: organizationHintSchema,
  ...opportunityFields,
  stage: crmStageSchema.describe('Where the deal starts. Defaults to waiting, and a stage that closes it is refused here.').optional(),
});

export const crmOpportunityAddInputIntentSchema = crmOpportunityAddInputSchema.partial();

const crmOpportunityUpdateObjectSchema = z.strictObject({
  opportunityHint: opportunityHintSchema,
  organizationHint: organizationHintSchema.describe('The organization it is with now, named the way crm_organization_list shows it.').optional(),
  ...opportunityFields,
  title: opportunityFields.title.describe('The title it should carry now.').optional(),
});

export const crmOpportunityUpdateInputSchema = crmOpportunityUpdateObjectSchema
  .refine(hasSomethingToWrite(['opportunityHint']), 'At least one opportunity field must be updated.')
  .meta({ minProperties: 2 });

export const crmOpportunityUpdateInputIntentSchema = crmOpportunityUpdateObjectSchema.omit({
  opportunityHint: true,
});

const crmOpportunityMoveObjectSchema = z.strictObject({
  opportunityHint: opportunityHintSchema,
  stage: crmStageSchema.describe('Where the deal moves to. Omit to leave it where it stands and only change its position.').optional(),
  position: z.number().int().min(0).describe('Where it sits among the deals at that stage, counting from 0.').optional(),
  closedAt: z.string().describe(`When the move happened. ${momentDescription} Defaults to now.`).optional(),
  finalAmountMinor: z.number().int().describe(`What it was worth in the company's own currency when it closed. ${amountDescription} Only a deal held in another currency needs one, and the record converts it when the call does not.`).optional(),
  reason: z.string().max(1024).describe('Why the deal was lost. A move into lost is refused without one.').optional(),
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
    description: "The organizations this company sells to, partners with, or buys from, by name. Use this to answer 'who are our customers', to find the organization another crm tool is about to change, or to read who owns a relationship.",
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
    description: 'Add an organization this company deals with. Read crm_organization_list first when the requester may be naming one that already exists; a second row for the same company is what makes a CRM useless.',
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
    description: 'Change what this company holds about an organization. Only the fields the call names change.',
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
    description: 'Put an organization away, so it leaves the CRM screens while everything recorded against it stays. Its people and its deals stay where they are, so put it to the requester before calling.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: crmOrganizationArchiveInputSchema,
    inputIntentSchema: crmOrganizationArchiveInputIntentSchema,
    result: { schema: crmArchivedResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    requiresApproval: true,
    completionEvidence: { mode: 'success', action: 'write_crm', targetKind: 'crm_organization' },
  },
  {
    name: 'crm_contact_list',
    namespace: 'crm',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_crm',
    policyResource: 'tool:crm_contact_list',
    description: "The people this company deals with at other organizations, with their email address, phone number and title. Use this to answer 'who do we talk to at ABC' or to find the contact another crm tool is about to change. These are counterparts, not colleagues; person_list answers for colleagues.",
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
    description: 'Add a person at an organization this company deals with. The organization has to exist already, so add it with crm_organization_add first when it does not.',
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
    description: 'Change what this company holds about a person at another organization, including moving them to a different one. Only the fields the call names change.',
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
    description: 'Put a person away, so they leave the CRM screens while everything recorded against them stays. Use this when somebody has left the organization rather than when a name is wrong.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: crmContactArchiveInputSchema,
    inputIntentSchema: crmContactArchiveInputIntentSchema,
    result: { schema: crmArchivedResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    requiresApproval: true,
    completionEvidence: { mode: 'success', action: 'write_crm', targetKind: 'crm_contact' },
  },
  {
    name: 'crm_opportunity_list',
    namespace: 'crm',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_crm',
    policyResource: 'tool:crm_opportunity_list',
    description: "The deals this company is working on, each with the stage it stands at, where it sits in that stage, what it is worth and how much has been recorded against it. Use this to answer 'what are we closing this month', 'how is the ABC deal going', or to find the deal another crm tool is about to change.",
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
    description: 'Open a deal with an organization this company already holds. It starts at waiting unless the call names another stage, and a stage that closes it is refused here: open it, then move it with crm_opportunity_move.',
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
    description: 'Change what this company holds about a deal: its title, what it is worth, who it is with, when it is expected to close. Only the fields the call names change. The stage is crm_opportunity_move’s.',
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
    description: "Move a deal to another stage, or reorder it within the one it stands at. A move into done or lost settles what the deal was worth in the company's own currency and cannot be undone by moving it back, and losing one needs a written reason, so put the move to the requester before calling.",
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
    description: 'Put a deal away, so it leaves the pipeline while everything recorded against it stays. A deal that was lost is moved to lost with its reason rather than archived.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: crmOpportunityArchiveInputSchema,
    inputIntentSchema: crmOpportunityArchiveInputIntentSchema,
    result: { schema: crmArchivedResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    requiresApproval: true,
    completionEvidence: { mode: 'success', action: 'write_crm', targetKind: 'crm_opportunity' },
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
    modelVisibility: CapabilityModelVisibility.Hidden,
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
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: crmVocabularySetInputSchema,
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
