import { z } from 'zod';

import {
  CapabilityAnsweredBy,
  CapabilityEstimatedLatency,
  CapabilitySideEffect,
  ResourceEffectIdentity,
} from './protocol';

import { ResourceMutationEffect, type CapabilityToolDefinition } from './definition';
import { listCursorSchema, listLimitSchema, listNextCursorSchema, listTotalCountSchema } from './list-page';

const resourceIDSchema = z.string()
  .min(1)
  .regex(/^\S(?:.*\S)?$/, 'Resource identity must not have leading or trailing whitespace.');

export enum WorkspaceEmploymentStatus {
  Active = 'active',
  Departed = 'departed',
}

export enum MemberKind {
  Human = 'human',
  Other = 'other',
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

export const personListInputSchema = z.strictObject({
  searchText: z.string().max(100).describe('Only the people whose name or email contains this text.').optional(),
  teamHint: teamHintSchema.describe('Only the people in this organization and the organizations under it: its exact team ID or its exact CURRENT name as a team_list result shows it. A name several organizations carry fails with the candidates to choose between.').optional(),
  limit: listLimitSchema('people').optional(),
  cursor: listCursorSchema('person_list').optional(),
});

export const personListInputIntentSchema = personListInputSchema;

const directoryPersonSchema = z.strictObject({
  personID: z.string(),
  name: z.string(),
  email: z.string(),
  mention: z.string().optional(),
  handle: z.string().optional(),
  isAdmin: z.boolean().optional(),
  kind: z.enum(MemberKind).optional(),
  employmentStatus: z.string().optional(),
  jobTitle: z.string().optional(),
  teamID: z.string().optional(),
  teamName: z.string().optional(),
  supervisorID: z.string().optional(),
  supervisorName: z.string().optional(),
  phoneNumber: z.string().optional(),
  hireDate: z.string().optional(),
  departureDate: z.string().optional(),
  timeZone: z.string().optional(),
});

export const personListResultSchema = z.strictObject({
  requesterID: z.string(),
  count: z.number().describe('How many people this page holds.'),
  totalCount: listTotalCountSchema('people'),
  people: z.array(directoryPersonSchema),
  nextCursor: listNextCursorSchema('people').optional(),
});

const personUpdateObjectSchema = z.strictObject({
  personHint: personHintSchema,
  name: z.string().max(256).describe('The name every screen and every mention shows for them.').optional(),
  isAdmin: z.boolean().describe('Whether they administer the company. Only an administrator may raise or lower this.').optional(),
  kind: z.enum(MemberKind).describe("What the account stands for: 'human' for somebody who works here, 'other' for anything else such as a test account. Only a human is counted in the headcount or drawn on the organization chart. Only an administrator changes this.").optional(),
  jobTitle: z.string().max(256).describe("Their job title, e.g. 'editor in chief'. An empty string clears it.").optional(),
  teamHint: teamMembershipHintSchema.optional(),
  supervisorHint: supervisorHintSchema.optional(),
  phoneNumber: z.string().max(64).describe('Their phone number as they write it. An empty string clears it.').optional(),
  hireDate: z.string().max(32).describe('The day they started, in YYYY-MM-DD format. An empty string clears it.').optional(),
  departureDate: z.string().max(32).describe('Their last working day, in YYYY-MM-DD format, which may be ahead. After it passes they are no longer counted in the headcount. An empty string clears it. Only an administrator sets it.').optional(),
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

export const personWithdrawInputSchema = z.strictObject({
  personHint: personHintSchema,
  purge: z.boolean()
    .describe('True only when the requester asked for the person to be deleted outright, such as an address added by mistake. A person whose attendance, leave or tasks still refer to them is withdrawn instead, and isDeleted says which happened.')
    .optional(),
});

export const personWithdrawInputIntentSchema = personWithdrawInputSchema.omit({ personHint: true });

export const personWithdrawResultSchema = z.strictObject({
  personID: resourceIDSchema,
  name: z.string(),
  email: z.string(),
  isDeleted: z.boolean(),
});

export const personPasswordResetInputSchema = z.strictObject({
  personHint: personHintSchema,
});

export const personPasswordResetInputIntentSchema = z.strictObject({});

export const personPasswordResetResultSchema = z.strictObject({
  personID: resourceIDSchema,
  email: z.string(),
  name: z.string(),
  temporaryPassword: z.string(),
});

export const personMessageDeleteInputSchema = z.strictObject({
  personHint: personHintSchema,
});

export const personMessageDeleteInputIntentSchema = z.strictObject({});

export const personMessageDeleteResultSchema = z.strictObject({
  email: z.string(),
  deletedCount: z.number().int(),
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
    description: 'Everyone who works here, with their exact name, email and mention, and what the company directory holds about them: their job title, their organization, who they report to, and when they started. Call this when somebody asks who a person is or who works here, and when a person hint was refused with a list of candidates to choose between. Do not call it to turn a fragment into a name before another call: person hints take the fragment as it was written and the server resolves it, so a name given partly or by a given name alone is already enough. requesterID names which of them is asking. A nextCursor in the result means more people match than this page holds: read the next page before concluding that somebody is not here.',
    version: '5',
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
    description: "Change what the company directory holds about one person: their name, their job title, their organization, who they report to, their phone number, the day they started, their last day, whether they administer the company, whether they still work here, and whether the account is a person at all. Only the fields the call names change. A person may correct their own phone number and start date; every other field, and anybody else's, is an administrator's. Ask the requester first before raising or lowering administrator rights or marking somebody departed; both are wide changes somebody has to mean.",
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
    name: 'person_withdraw',
    namespace: 'person',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_task',
    policyResource: 'tool:person_withdraw',
    description: 'Withdraw one person from the company: their membership ends, they can no longer sign in, they leave the company messenger, and every screen stops showing them, while the attendance, leave and tasks they left behind still read. Only an administrator withdraws people, and never themselves. With purge, the person is deleted outright instead when nothing refers to them. To delete what they wrote in the messenger, call person_message_delete first, while they are still in the directory. Requires approval; this action is irreversible.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: personWithdrawInputSchema,
    inputIntentSchema: personWithdrawInputIntentSchema,
    result: {
      schema: personWithdrawResultSchema,
      effects: [{
        objectType: 'person',
        effect: ResourceMutationEffect.Deleted,
        resultField: 'personID',
        effectIdentity: ResourceEffectIdentity.ID,
        when: { resultField: 'isDeleted', equals: true },
      }, {
        objectType: 'person',
        effect: ResourceMutationEffect.Archived,
        resultField: 'personID',
        effectIdentity: ResourceEffectIdentity.ID,
        when: { resultField: 'isDeleted', equals: false },
      }],
    },
    sideEffect: CapabilitySideEffect.Destructive,
    requiresApproval: true,
  },
  {
    name: 'person_password_reset',
    namespace: 'person',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_task',
    policyResource: 'tool:person_password_reset',
    description: "Reset one person's password: their old password stops working and they get a temporary password to sign in with once. Only an administrator resets a password, and an administrator's password only its owner resets. The answer carries the temporary password, which is the only time it is readable, so pass it to the person who asked. Requires approval; the old password cannot be brought back.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: personPasswordResetInputSchema,
    inputIntentSchema: personPasswordResetInputIntentSchema,
    result: {
      schema: personPasswordResetResultSchema,
      effects: [{
        objectType: 'person',
        effect: ResourceMutationEffect.Updated,
        resultField: 'personID',
        effectIdentity: ResourceEffectIdentity.ID,
      }],
    },
    sideEffect: CapabilitySideEffect.Destructive,
    requiresApproval: true,
  },
  {
    name: 'person_message_delete',
    namespace: 'person',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'platform_message',
    policyResource: 'tool:person_message_delete',
    description: 'Delete every message one person has written in the company messenger, in every conversation, for somebody leaving or asking to be forgotten. Only an administrator may. It reaches only a person still in the directory, so call it before person_withdraw. The answer counts what was deleted. Requires approval; deleted messages cannot be brought back.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.High,
    inputSchema: personMessageDeleteInputSchema,
    inputIntentSchema: personMessageDeleteInputIntentSchema,
    result: { schema: personMessageDeleteResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Destructive,
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
export type PersonListResult = z.infer<typeof personListResultSchema>;
export type PersonInviteResult = z.infer<typeof personInviteResultSchema>;
export type PersonWithdrawResult = z.infer<typeof personWithdrawResultSchema>;
export type PersonPasswordResetResult = z.infer<typeof personPasswordResetResultSchema>;
export type TeamResult = z.infer<typeof teamResultSchema>;
export type TeamListResult = z.infer<typeof teamListResultSchema>;
export type TeamAddInput = z.infer<typeof teamAddInputSchema>;
export type TeamUpdateInput = z.infer<typeof teamUpdateObjectSchema>;
export type TeamDeleteInput = z.infer<typeof teamDeleteInputSchema>;
export type TeamDeleteResult = z.infer<typeof teamDeleteResultSchema>;
