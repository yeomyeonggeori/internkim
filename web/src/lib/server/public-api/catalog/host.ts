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

export const hostDiagnosticViews = ['runs', 'run', 'model_call', 'turn_input', 'inbound', 'service_logs', 'requests'] as const;

export const hostDiagnosticsGetInputSchema = z.strictObject({
  view: z.enum(hostDiagnosticViews).describe('The existing work-history view to read: runs lists executions; run reads its event ledger; model_call reads a recorded model exchange; turn_input reads the recorded input; inbound reads connector events; service_logs reads service logs; requests reads request metrics.'),
  taskRunID: z.string().trim().min(1).optional(),
  id: z.string().trim().min(1).describe('The exact recorded model call or turn input ID, returned by the event ledger.').optional(),
  service: z.string().trim().min(1).describe('The exact service name accepted by the existing log viewer.').optional(),
  conversationID: z.string().trim().min(1).optional(),
  messageID: z.string().trim().min(1).optional(),
  status: z.string().trim().min(1).optional(),
  limit: z.number().int().min(1).max(1000).optional(),
  offset: z.number().int().nonnegative().optional(),
});

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
    name: 'host_diagnostics_get',
    namespace: 'host',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'workspace_company',
    policyResource: 'tool:host_diagnostics_get',
    description: "Read host diagnostics through the same execution records, event ledger, model exchanges, turn inputs, connector events, service logs and request metrics as the work-history web page. Only a company administrator may ask. Use recorded IDs for detail views. This reads existing evidence and executes no commands supplied by the caller.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: hostDiagnosticsGetInputSchema,
    result: { schema: z.strictObject({ document: z.string().describe('The original view response serialized as JSON, preserving every recorded field.') }), effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
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
