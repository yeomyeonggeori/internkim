import { z } from 'zod';

import {
  CapabilityAnsweredBy,
  CapabilityEstimatedLatency,
  CapabilitySideEffect,
  ResourceEffectIdentity,
} from './protocol';
import { ResourceMutationEffect, type CapabilityToolDefinition } from './definition';
import {
  personaIdentityChangeSchema,
  personaIdentitySchema,
  personaProfileChangeSchema,
  personaProfileSchema,
} from '$lib/persona/persona-schema';

export const personaGetInputSchema = z.strictObject({});

export const personaToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: 'persona_identity_get',
    namespace: 'persona',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'workspace_persona',
    policyResource: 'tool:persona_identity_get',
    description: "Read the agent's persona identity: every name it answers to, the first being the one it introduces itself with, and its handle, role, creature, emoji and introduction. There is one for the whole company, and any member may read it.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: personaGetInputSchema,
    result: { schema: personaIdentitySchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'persona_identity_update',
    namespace: 'persona',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'workspace_persona',
    policyResource: 'tool:persona_identity_update',
    description: "Change the agent's persona identity for the whole company. Only an administrator may. Only the fields the call names change, and names replaces the whole list. Every colleague meets the agent by these names, so put it to the requester before calling. The whole identity is answered back.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: personaIdentityChangeSchema,
    inputIntentSchema: personaIdentityChangeSchema,
    result: { schema: personaIdentitySchema, effects: [{ objectType: 'persona_identity', effect: ResourceMutationEffect.Updated, effectIdentity: ResourceEffectIdentity.Singleton }] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    requiresApproval: true,
  },
  {
    name: 'persona_profile_get',
    namespace: 'persona',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'workspace_persona',
    policyResource: 'tool:persona_profile_get',
    description: "Read the requester's persona profile, which says how they want the agent to work with them: what to call them, what to know about them, their preferences, tone and reply language, and the daily morning briefing's time and whether it is on. It is always the requester's own.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: personaGetInputSchema,
    result: { schema: personaProfileSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'persona_profile_update',
    namespace: 'persona',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'workspace_persona',
    policyResource: 'tool:persona_profile_update',
    description: "Change the requester's persona profile, the only profile this ever changes. Only the fields the call names change, down to one nested field: morningBriefing with only enabled false turns the briefing off and keeps its time. A list such as preferences replaces the whole list. The whole profile is answered back.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: personaProfileChangeSchema,
    inputIntentSchema: personaProfileChangeSchema,
    result: { schema: personaProfileSchema, effects: [{ objectType: 'persona_profile', effect: ResourceMutationEffect.Updated, effectIdentity: ResourceEffectIdentity.Singleton }] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
  },
];
