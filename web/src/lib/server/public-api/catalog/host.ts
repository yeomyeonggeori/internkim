import { z } from 'zod';

import {
  CapabilityAnsweredBy,
  CapabilityEstimatedLatency,
  CapabilitySideEffect,
  ResourceEffectIdentity,
} from './protocol';
import { ResourceMutationEffect, type CapabilityToolDefinition } from './definition';
import { hostUpdateResultSchema, hostVersionGetResultSchema, releaseTagSchema } from '$lib/host/host-version';

export const hostVersionGetInputSchema = z.strictObject({});

export const hostUpdateInputSchema = z.strictObject({
  targetVersion: releaseTagSchema
    .describe('The exact tag of a stable release to install, from host_version_get. Omit it to install the latest stable release; name an older one, such as previousStable, to go back.')
    .optional(),
  startsAt: z.string()
    .describe('An RFC 3339 instant with an offset, only when the requester named a time for the update.')
    .optional(),
  isRequestedNow: z.boolean()
    .describe('True when the requester asked for the update to happen right away.')
    .optional(),
});

export const hostToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: 'host_version_get',
    namespace: 'host',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'workspace_company',
    policyResource: 'tool:host_version_get',
    description: "Read which version of internkim this company's host runs, the release channel it follows, how it is updated (apt, dnf, pacman or brew), and the latest stable release with its date and release notes, which says whether an update is available. previousStable is the stable release before the installed one. Any member may ask.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: hostVersionGetInputSchema,
    result: { schema: hostVersionGetResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'host_update',
    namespace: 'host',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'workspace_company',
    policyResource: 'tool:host_update',
    description: "Update this company's host to a stable release, or take it back to an older stable one. Only an administrator may, and only on a host that follows the stable channel. The requester is asked to confirm with the consequences and a choice of when; the agent, relay and messenger then restart, and the agent reports the result in this conversation when it is back.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: hostUpdateInputSchema,
    inputIntentSchema: hostUpdateInputSchema,
    result: { schema: hostUpdateResultSchema, effects: [{ objectType: 'host', effect: ResourceMutationEffect.Updated, effectIdentity: ResourceEffectIdentity.Singleton }] },
    sideEffect: CapabilitySideEffect.Destructive,
    requiresApproval: true,
  },
];
