import { z } from 'zod';
import { CapabilityAnsweredBy, CapabilityEstimatedLatency, CapabilitySideEffect, ResourceEffectIdentity } from './protocol';
import { ResourceMutationEffect, type CapabilityToolDefinition } from './definition';

const noInputSchema = z.strictObject({});
export const calendarSubscriptionGetResultSchema = z.strictObject({ registered: z.boolean() });
export const calendarSubscriptionAddResultSchema = z.strictObject({ address: z.string() });
export const calendarSubscriptionDeleteResultSchema = z.strictObject({ deleted: z.boolean() });

function calendarSubscriptionTool(
	name: string,
	description: string,
	resultSchema: z.ZodType,
	sideEffect: CapabilitySideEffect,
	effect?: ResourceMutationEffect
): CapabilityToolDefinition {
	return {
		name, namespace: 'calendar', description,
		answeredBy: CapabilityAnsweredBy.Record,
		privacyClass: 'workspace_calendar', policyResource: `tool:${name}`,
		version: '1', estimatedLatency: CapabilityEstimatedLatency.Low,
		inputSchema: noInputSchema,
		result: { schema: resultSchema, effects: effect ? [{
			objectType: 'calendar_subscription', effect, effectIdentity: ResourceEffectIdentity.Singleton
		}] : [] },
		sideEffect,
		...(effect ? { requiresApproval: true, inputIntentSchema: noInputSchema } : {})
	};
}

export const calendarSubscriptionToolDefinitions: CapabilityToolDefinition[] = [
	calendarSubscriptionTool('calendar_subscription_get',
		"Whether the company calendar has a subscription address, the .ics feed a calendar app such as Google Calendar or Apple Calendar subscribes to. The address itself is shown only when it is made, so this answers only whether one exists.",
		calendarSubscriptionGetResultSchema, CapabilitySideEffect.Read),
	calendarSubscriptionTool('calendar_subscription_add',
		'Make a new subscription address for the company calendar and answer it. It is shown only this once. Any earlier address stops working at once, so every calendar app subscribed to it must subscribe again. Only a company administrator makes one. Requires approval.',
		calendarSubscriptionAddResultSchema, CapabilitySideEffect.WorkspaceWrite, ResourceMutationEffect.Updated),
	calendarSubscriptionTool('calendar_subscription_delete',
		'Revoke the company calendar subscription address, so no calendar app can read it any longer. Only a company administrator revokes it. Requires approval; a revoked address never works again, and calendar_subscription_add makes a new one.',
		calendarSubscriptionDeleteResultSchema, CapabilitySideEffect.Destructive, ResourceMutationEffect.Deleted)
];
