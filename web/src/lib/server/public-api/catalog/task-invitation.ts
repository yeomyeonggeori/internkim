import { z } from 'zod';
import { CapabilityAnsweredBy, CapabilityEstimatedLatency, CapabilitySideEffect, ResourceEffectIdentity } from './protocol';
import { ResourceMutationEffect, type CapabilityToolDefinition } from './definition';
import { taskInvitationListResultSchema } from '$lib/task-invitation/invitations';
export { taskInvitationListResultSchema };

const invitationIDSchema = z.string().uuid();

export const taskInvitationListInputSchema = z.strictObject({});

export const taskInviteInputSchema = z.strictObject({
	email: z.string().regex(/^[^\s@]+@[^\s@]+\.[^\s@]+$/).describe('The address of the person outside the company who is invited.'),
	businesses: z.array(z.string().min(1)).min(1)
		.describe('Exact business names from registeredLabels in a task_list result. The invitation shows these businesses\' tasks and nothing else.')
});
export const taskInviteResultSchema = z.strictObject({ invitationID: invitationIDSchema, sent: z.boolean() });


export const taskInvitationDeleteInputSchema = z.strictObject({
	invitationID: invitationIDSchema.describe('Exact invitationID from task_invitation_list.')
});
export const taskInvitationDeleteResultSchema = z.strictObject({ invitationID: invitationIDSchema });

const invitationTool = {
	namespace: 'task',
	answeredBy: CapabilityAnsweredBy.Record,
	privacyClass: 'workspace_task',
	version: '1',
	estimatedLatency: CapabilityEstimatedLatency.Medium
} as const;

export const taskInvitationToolDefinitions: CapabilityToolDefinition[] = [
	{
		...invitationTool,
		name: 'task_invite',
		policyResource: 'tool:task_invite',
		description: 'Invite someone outside the company by email to read the tasks of one or more businesses, and send them the invitation email. They sign in with that address and see those businesses\' tasks read-only, unmasked, and no other task. Inviting an address that already holds an open invitation replaces its businesses and sends the email again. Only a company administrator invites. Requires approval; the email reaches someone outside the company.',
		inputSchema: taskInviteInputSchema,
		inputIntentSchema: taskInviteInputSchema.partial(),
		result: { schema: taskInviteResultSchema, effects: [{
			objectType: 'task_invitation', effect: ResourceMutationEffect.Sent,
			resultField: 'invitationID', effectIdentity: ResourceEffectIdentity.ID
		}] },
		sideEffect: CapabilitySideEffect.ExternalSend,
		requiresApproval: true
	},
	{
		...invitationTool,
		name: 'task_invitation_list',
		policyResource: 'tool:task_invitation_list',
		description: 'List the open task invitations that let people outside the company read the tasks of named businesses, with when each was accepted. Only a company administrator sees them.',
		estimatedLatency: CapabilityEstimatedLatency.Low,
		inputSchema: taskInvitationListInputSchema,
		result: { schema: taskInvitationListResultSchema, effects: [] },
		sideEffect: CapabilitySideEffect.Read
	},
	{
		...invitationTool,
		name: 'task_invitation_delete',
		policyResource: 'tool:task_invitation_delete',
		description: 'Delete an open task invitation by its invitationID from task_invitation_list, revoking it: its recipient sees nothing from their next read. Only a company administrator deletes one. Requires approval.',
		inputSchema: taskInvitationDeleteInputSchema,
		inputIntentSchema: taskInvitationDeleteInputSchema.partial(),
		result: { schema: taskInvitationDeleteResultSchema, effects: [{
			objectType: 'task_invitation', effect: ResourceMutationEffect.Deleted,
			resultField: 'invitationID', effectIdentity: ResourceEffectIdentity.ID
		}] },
		sideEffect: CapabilitySideEffect.WorkspaceWrite,
		requiresApproval: true
	}
];
