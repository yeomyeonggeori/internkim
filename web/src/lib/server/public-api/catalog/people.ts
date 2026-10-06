import { z } from 'zod';

import {
  CapabilityAnsweredBy,
  CapabilityEstimatedLatency,
  CapabilitySideEffect,
  ResourceEffectIdentity,
} from './protocol';

import { ResourceMutationEffect, type CapabilityToolDefinition } from './definition';

const resourceIDSchema = z.string()
  .min(1)
  .regex(/^\S(?:.*\S)?$/, 'Resource identity must not have leading or trailing whitespace.');

export enum WorkspaceEmploymentStatus {
  Active = 'active',
  Departed = 'departed',
}

const personHintSchema = z.string().min(1).max(256).describe(
  'Identifies the person to act on: their exact person ID, their exact email address, their @handle, or their name as a person_list result shows it. A name only one colleague carries resolves; a name several carry fails with the candidates to choose between.',
);

const teamHintSchema = z.string().min(1).max(256).describe(
  'Identifies the organization to act on: its exact team ID or its exact CURRENT name as a team_list result shows it. Never a new or intended name. A name only one organization carries resolves; a name several carry fails with the candidates to choose between.',
);

const teamMembershipHintSchema = z.string().max(256).describe(
  'The organization this person belongs to, named the way teamHint is. An empty string takes them out of every organization.',
);

const supervisorHintSchema = z.string().max(256).describe(
  'The person this one reports to, named the way personHint is. An empty string leaves them reporting to nobody.',
);

const parentHintSchema = z.string().max(256).describe(
  'The organization this one sits under, named the way teamHint is. An empty string makes it a top-level organization.',
);

export const personListInputSchema = z.strictObject({searchText: z.string().max(100).optional(),limit: z.number().int().min(1).max(48).optional()});

export const personListInputIntentSchema = personListInputSchema;

const directoryPersonSchema = z.strictObject({
  personID: z.string(),
  name: z.string(),
  email: z.string(),
  mention: z.string().optional(),
  handle: z.string().optional(),
  isAdmin: z.boolean().optional(),
  employmentStatus: z.string().optional(),
  jobTitle: z.string().optional(),
  teamID: z.string().optional(),
  teamName: z.string().optional(),
  supervisorID: z.string().optional(),
  supervisorName: z.string().optional(),
  phoneNumber: z.string().optional(),
  hireDate: z.string().optional(),
  timeZone: z.string().optional(),
});

export const personListResultSchema = z.strictObject({
  requesterID: z.string(),
  count: z.number(),
  people: z.array(directoryPersonSchema),
});

const personUpdateObjectSchema = z.strictObject({
  personHint: personHintSchema,
  name: z.string().max(256).describe('The name every screen and every mention shows for them.').optional(),
  isAdmin: z.boolean().describe('Whether they administer the company. Only an administrator may raise or lower this.').optional(),
  jobTitle: z.string().max(256).describe("Their job title, e.g. 'editor in chief'. An empty string clears it.").optional(),
  teamHint: teamMembershipHintSchema.optional(),
  supervisorHint: supervisorHintSchema.optional(),
  phoneNumber: z.string().max(64).describe('Their phone number as they write it. An empty string clears it.').optional(),
  hireDate: z.string().max(32).describe('The day they started, in YYYY-MM-DD format. An empty string clears it.').optional(),
  employmentStatus: z.enum(WorkspaceEmploymentStatus)
    .describe('Whether they still work here. Only an administrator may mark somebody departed or bring them back.')
    .optional(),
});

export const personUpdateInputSchema = personUpdateObjectSchema
  .refine((document) => Object.keys(document).length > 1, 'At least one person field must be updated.')
  .meta({ minProperties: 2 });

export const personUpdateInputIntentSchema = personUpdateObjectSchema.omit({ personHint: true });

export const personInviteInputSchema = z.strictObject({
  email: z.string().max(320).describe("The address they sign in with, e.g. 'newcomer@example.com'. One address belongs to one company."),
  name: z.string().min(1).max(256).describe('Their name as colleagues will see it.'),
  jobTitle: z.string().max(256).describe('Their job title, when it is known already.').optional(),
  teamHint: teamMembershipHintSchema.optional(),
});

export const personInviteInputIntentSchema = personInviteInputSchema.partial();

export const personInviteResultSchema = z.strictObject({
  personID: resourceIDSchema,
  email: z.string(),
  name: z.string(),
  employmentStatus: z.string(),
  temporaryPassword: z.string(),
});

const teamResultSchema = z.strictObject({
  teamID: resourceIDSchema,
  name: z.string(),
  parentTeamID: z.string(),
  parentTeamName: z.string(),
  position: z.number().int(),
  peopleCount: z.number().int(),
});

export const teamListInputSchema = z.strictObject({});

export const teamListInputIntentSchema = z.strictObject({});

export const teamListResultSchema = z.strictObject({
  count: z.number().int(),
  teams: z.array(teamResultSchema),
});

export const teamAddInputSchema = z.strictObject({
  name: z.string().min(1).max(256).describe("The organization's name, e.g. 'Engineering'."),
  parentHint: parentHintSchema.optional(),
  position: z.number().int().describe('Where it sits among its siblings, counting from 0. Omit to put it last.').optional(),
});

export const teamAddInputIntentSchema = teamAddInputSchema.partial();

const teamUpdateObjectSchema = z.strictObject({
  teamHint: teamHintSchema,
  name: z.string().min(1).max(256).describe("The organization's new name.").optional(),
  parentHint: parentHintSchema.optional(),
  position: z.number().int().describe('Where it sits among its siblings, counting from 0.').optional(),
});

export const teamUpdateInputSchema = teamUpdateObjectSchema
  .refine((document) => Object.keys(document).length > 1, 'At least one organization field must be updated.')
  .meta({ minProperties: 2 });

export const teamUpdateInputIntentSchema = teamUpdateObjectSchema.omit({ teamHint: true });

export const teamDeleteInputSchema = z.strictObject({
  teamHint: teamHintSchema,
});

export const teamDeleteInputIntentSchema = z.strictObject({});

export const teamDeleteResultSchema = z.strictObject({
  teamID: resourceIDSchema,
  name: z.string(),
  deleted: z.literal(true),
  peopleLeftWithNoOrganization: z.number().int(),
});

export const peopleToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: 'person_list',
    namespace: 'person',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_task',
    policyResource: 'tool:person_list',
    description: 'Everyone who works here, with their exact name, email and mention, and what the company directory holds about them: their job title, their organization, who they report to, and when they started. Call this when somebody asks who a person is or who works here, and when a person hint was refused with a list of candidates to choose between. Do not call it to turn a fragment into a name before another call: person hints take the fragment as it was written and the server resolves it, so a name given partly or by a given name alone is already enough. requesterID names which of them is asking.',
    version: '4',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: personListInputSchema,
    inputIntentSchema: personListInputIntentSchema,
    result: { schema: personListResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'person_update',
    namespace: 'person',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_task',
    policyResource: 'tool:person_update',
    description: "Change what the company directory holds about one person: their name, their job title, their organization, who they report to, their phone number, the day they started, whether they administer the company, and whether they still work here. Only the fields the call names change. A person may correct their own phone number and start date; every other field, and anybody else's, is an administrator's. Ask the requester first before raising or lowering administrator rights or marking somebody departed; both are wide changes somebody has to mean.",
    version: '2',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: personUpdateInputSchema,
    inputIntentSchema: personUpdateInputIntentSchema,
    result: {
      schema: directoryPersonSchema,
      effects: [{
        objectType: 'person',
        effect: ResourceMutationEffect.Updated,
        resultField: 'personID',
        effectIdentity: ResourceEffectIdentity.ID,
      }],
    },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
  },
  {
    name: 'person_invite',
    namespace: 'person',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_task',
    policyResource: 'tool:person_invite',
    description: 'Invite somebody to the company: it creates their account, gives them a temporary password to sign in with once, and puts them in the directory. Only an administrator invites people. The answer carries the temporary password, which is the only time it is readable, so pass it to whoever is bringing the person in. Requires approval; an account and an address are hard to take back.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: personInviteInputSchema,
    inputIntentSchema: personInviteInputIntentSchema,
    result: {
      schema: personInviteResultSchema,
      effects: [{
        objectType: 'person',
        effect: ResourceMutationEffect.Created,
        resultField: 'personID',
        effectIdentity: ResourceEffectIdentity.ID,
      }],
    },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    requiresApproval: true,
  },
  {
    name: 'team_list',
    namespace: 'team',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_task',
    policyResource: 'tool:team_list',
    description: "Every team in the company — the organizations the organization chart holds — in the order it shows them, each with the organization it sits under and how many people belong to it. Call this before naming an organization in another call, and when an organization hint was refused with candidates to choose between.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: teamListInputSchema,
    inputIntentSchema: teamListInputIntentSchema,
    result: { schema: teamListResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'team_add',
    namespace: 'team',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_task',
    policyResource: 'tool:team_add',
    description: 'Create one team, an organization on the chart, optionally under another one. Only an administrator edits the organization chart. Creating an organization puts nobody in it; person_update moves people.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: teamAddInputSchema,
    inputIntentSchema: teamAddInputIntentSchema,
    result: {
      schema: teamResultSchema,
      effects: [{
        objectType: 'team',
        effect: ResourceMutationEffect.Created,
        resultField: 'teamID',
        effectIdentity: ResourceEffectIdentity.ID,
      }],
    },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
  },
  {
    name: 'team_update',
    namespace: 'team',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_task',
    policyResource: 'tool:team_update',
    description: 'Rename one team, move that organization under another one, or change where it sits among its siblings. Only the fields the call names change, and only an administrator edits the organization chart. Moving an organization under one of its own descendants is refused.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: teamUpdateInputSchema,
    inputIntentSchema: teamUpdateInputIntentSchema,
    result: {
      schema: teamResultSchema,
      effects: [{
        objectType: 'team',
        effect: ResourceMutationEffect.Updated,
        resultField: 'teamID',
        effectIdentity: ResourceEffectIdentity.ID,
      }],
    },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
  },
  {
    name: 'team_delete',
    namespace: 'team',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_task',
    policyResource: 'tool:team_delete',
    description: 'Remove one team from the organization chart, so that organization is gone. Everybody who belonged to it keeps their directory entry and belongs to no organization until somebody moves them. An organization with organizations under it is refused; remove or move those first. Requires approval; this action is irreversible.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: teamDeleteInputSchema,
    inputIntentSchema: teamDeleteInputIntentSchema,
    result: {
      schema: teamDeleteResultSchema,
      effects: [{
        objectType: 'team',
        effect: ResourceMutationEffect.Deleted,
        resultField: 'teamID',
        effectIdentity: ResourceEffectIdentity.ID,
      }],
    },
    sideEffect: CapabilitySideEffect.Destructive,
    requiresApproval: true,
  },
];

export type DirectoryPerson = z.infer<typeof directoryPersonSchema>;
export type PersonUpdateInput = z.infer<typeof personUpdateObjectSchema>;
export type PersonInviteInput = z.infer<typeof personInviteInputSchema>;
export type PersonInviteResult = z.infer<typeof personInviteResultSchema>;
export type TeamResult = z.infer<typeof teamResultSchema>;
export type TeamListResult = z.infer<typeof teamListResultSchema>;
export type TeamAddInput = z.infer<typeof teamAddInputSchema>;
export type TeamUpdateInput = z.infer<typeof teamUpdateObjectSchema>;
export type TeamDeleteInput = z.infer<typeof teamDeleteInputSchema>;
export type TeamDeleteResult = z.infer<typeof teamDeleteResultSchema>;
