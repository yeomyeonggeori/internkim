import { z } from 'zod';

import {
  CapabilityAnsweredBy,
  CapabilityEstimatedLatency,
  CapabilitySideEffect,
  ResourceEffectIdentity,
} from './protocol';
import { ResourceMutationEffect, type CapabilityToolDefinition } from './definition';
import { personaSoulChangeSchema, personaSoulSchema } from '$lib/persona/persona-schema';
import { personaGetInputSchema } from './persona';

const soulRevisionSchema = z.strictObject({
  version: z.number().int().describe('The revision number. The first revision is 1 and each change adds one.'),
  document: personaSoulSchema,
  reason: z.string(),
  createdAt: z.string(),
  origin: z.string().describe('Who wrote the revision: an administrator, the agent learning, or the defaults.'),
  author: z.string().optional(),
});

const soulHistorySchema = z.strictObject({ history: z.array(soulRevisionSchema).describe('Every revision, oldest first. The last one is in force.') });

const learningSettingsSchema = z.strictObject({
  enabled: z.boolean(),
  activeLimit: z.number().int(),
});

const learnedSkillSchema = z.strictObject({
  id: z.string(),
  version: z.number().int(),
  audience: z.string(),
  description: z.string(),
  instruction: z.string(),
  evidenceIDs: z.array(z.string()).nullish(),
  reason: z.string().optional(),
  verification: z.string().optional(),
  status: z.string(),
  protected: z.boolean(),
  createdAt: z.string().optional(),
  updatedAt: z.string().optional(),
  retiredAt: z.string().optional(),
});

const learningOverviewSchema = z.strictObject({
  skills: z.array(learnedSkillSchema),
  settings: learningSettingsSchema,
  activeCount: z.number().int(),
});

const principlesRevertInputSchema = z.strictObject({
  version: z.number().int().min(1).describe('The revision number to bring back, as persona_principles_get lists it. It is written as a new revision on top, so the history keeps everything.'),
  reason: z.string().max(200).optional().describe('Why the principles go back, in one sentence.'),
});

const learningSettingsChangeSchema = z.strictObject({
  enabled: z.boolean().optional().describe('Whether the agent learns skills from its work. Turning it off keeps the skills already learned.'),
  activeLimit: z.number().int().optional().describe('How many learned skills may be in use at once, a whole number from 1 up. The server refuses a value it does not accept.'),
});

const learnedSkillUpdateInputSchema = z.strictObject({
  id: z.string().min(1).describe('The id of the learned skill, as persona_learning_get lists it.'),
  protected: z.boolean().optional().describe('true keeps the skill from being replaced by a newer lesson; false lifts that.'),
  retired: z.boolean().optional().describe('true takes the skill out of use and keeps it in the list; false puts a retired skill back in use.'),
});

const learnedSkillUpdateResultSchema = z.strictObject({
  id: z.string(),
  action: z.string(),
  ok: z.boolean(),
});

const principlesEffect = { objectType: 'persona_principles', effect: ResourceMutationEffect.Updated, effectIdentity: ResourceEffectIdentity.Singleton };

export const personaLearningToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: 'persona_principles_get',
    namespace: 'persona',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'workspace_persona',
    policyResource: 'tool:persona_principles_get',
    description: "Read the agent's persona working principles and every revision they have had, oldest first with the one in force last: what it holds to, what it never does, how it works, its tone and its reply language. There is one set for the whole company, and any member may read it.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: personaGetInputSchema,
    result: { schema: soulHistorySchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'persona_principles_update',
    namespace: 'persona',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'workspace_persona',
    policyResource: 'tool:persona_principles_update',
    description: "Change the agent's persona working principles for the whole company. Only an administrator may. Only the fields the call names change, and a list such as values replaces the whole list. The agent rereads these in every conversation, so put the change to the requester before calling. The new revision is answered back, and the history keeps the old one.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: personaSoulChangeSchema,
    inputIntentSchema: personaSoulChangeSchema,
    result: { schema: soulRevisionSchema, effects: [principlesEffect] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    requiresApproval: true,
  },
  {
    name: 'persona_principles_revert',
    namespace: 'persona',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'workspace_persona',
    policyResource: 'tool:persona_principles_revert',
    description: "Bring the agent's persona working principles back to an earlier revision for the whole company. Only an administrator may. The agent rereads these in every conversation, so put it to the requester before calling. An unknown version is refused. The new revision is answered back.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: principlesRevertInputSchema,
    inputIntentSchema: principlesRevertInputSchema.partial(),
    result: { schema: soulRevisionSchema, effects: [principlesEffect] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    requiresApproval: true,
  },
  {
    name: 'persona_learning_get',
    namespace: 'persona',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'workspace_persona',
    policyResource: 'tool:persona_learning_get',
    description: "Read the agent's persona learning: whether learning is on, how many learned skills may be in use at once and how many are, and every learned skill the requester may see, retired ones included.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: personaGetInputSchema,
    result: { schema: learningOverviewSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'persona_learning_update',
    namespace: 'persona',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'workspace_persona',
    policyResource: 'tool:persona_learning_update',
    description: "Change the agent's persona learning settings for the whole company. Only an administrator may. Only the settings the call names change. The settings are answered back.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: learningSettingsChangeSchema,
    inputIntentSchema: learningSettingsChangeSchema,
    result: { schema: learningSettingsSchema, effects: [{ objectType: 'persona_learning_settings', effect: ResourceMutationEffect.Updated, effectIdentity: ResourceEffectIdentity.Singleton }] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
  },
  {
    name: 'persona_learned_skill_update',
    namespace: 'persona',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'workspace_persona',
    policyResource: 'tool:persona_learned_skill_update',
    description: "Protect, unprotect, retire or restore one learned skill of the agent's persona. Only an administrator may. Name the skill's id and at least one of protected or retired. Retiring takes the skill out of use and keeps it in the list.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: learnedSkillUpdateInputSchema,
    inputIntentSchema: learnedSkillUpdateInputSchema.partial(),
    result: { schema: learnedSkillUpdateResultSchema, effects: [{ objectType: 'persona_learned_skill', effect: ResourceMutationEffect.Updated, resultField: 'id', effectIdentity: ResourceEffectIdentity.ID }] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
  },
];
