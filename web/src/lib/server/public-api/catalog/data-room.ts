import { z } from 'zod';
import { dataRoomCategorySchema } from '$lib/data-room/model';
import { dataRoomGetResultSchema } from '$lib/data-room/schemas';
import { dataRoomLinkInputSchema, dataRoomLinksSchema, dataRoomLinkCreatedSchema, dataRoomLinkRevokeSchema } from '$lib/data-room/links';
export { dataRoomGetResultSchema, dataRoomShareSchema } from '$lib/data-room/schemas';
import { CapabilityAnsweredBy, CapabilityEstimatedLatency, CapabilitySideEffect, ResourceEffectIdentity } from './protocol';
import { ResourceMutationEffect, type CapabilityToolDefinition } from './definition';
import type { CapabilityNamespace } from './namespaces';

export const dataRoomCategorySetInputSchema = dataRoomCategorySchema.omit({ fields: true });
export const dataRoomFieldSetInputSchema = z.strictObject({
	field: z.string().min(1).describe("Exact field as stored: a company_info key such as 'capital', a company_metric key such as 'annualRevenue', or a company_record category such as 'funding'."),
	categoryCode: z.string().regex(/^[A-Z]{1,2}$/).describe('Exact code from dataroom_get whose readers should see the field. Omit to show it only to readers of X.').optional()
});
export const dataRoomShareCreateInputSchema = z.strictObject({
	circleID: z.string(),
	audience: z.enum(['email', 'public']),
	email: z.string().regex(/^[^\s@]+@[^\s@]+\.[^\s@]+$/).optional(),
	expiresAt: z.string().optional(),
	canDownload: z.boolean().optional()
});
export const dataRoomShareRevokeInputSchema = z.strictObject({ shareID: z.string().uuid() });
const dataRoomShareCreatedSchema = z.strictObject({ shareID: z.string().uuid() });
export const dataRoomInvitationSendInputSchema = z.strictObject({
	shareID: z.string().uuid().describe('Exact shareID of an email share, from dataroom_share_add or dataroom_get.')
});
export const dataRoomInvitationAcceptInputSchema = z.strictObject({
	shareID: z.string().uuid().describe('Exact shareID the invitation address names, the last segment of /share/invitations/<shareID>.')
});
export const dataRoomInvitationSentSchema = z.strictObject({ shareID: z.string().uuid(), sent: z.boolean() });
export const dataRoomInvitationAcceptedSchema = z.strictObject({ shareID: z.string().uuid(), companyID: z.string().uuid() });
export const recordSavedSchema = z.strictObject({ saved: z.boolean() });

export function companyRecordTool(namespace: CapabilityNamespace, objectType: string, subject: string) {
	return (name: string, description: string, inputSchema: z.ZodObject, resultSchema: z.ZodType,
		isWrite = false): CapabilityToolDefinition => ({
		name, namespace, description: `${subject}: ${description}`,
		answeredBy: CapabilityAnsweredBy.Record,
		privacyClass: 'workspace_company', policyResource: `tool:${name}`,
		version: '1', estimatedLatency: CapabilityEstimatedLatency.Low,
		inputSchema,
		result: { schema: resultSchema, effects: isWrite ? [{
			objectType, effect: ResourceMutationEffect.Updated,
			effectIdentity: ResourceEffectIdentity.Singleton
		}] : [] },
		sideEffect: isWrite ? CapabilitySideEffect.WorkspaceWrite : CapabilitySideEffect.Read,
		...(isWrite ? { requiresApproval: true, approvalScope: 'company', inputIntentSchema: inputSchema.partial() } : {})
	});
}

const dataRoomTool = companyRecordTool('dataroom', 'data_room', 'Company dataroom');

export const dataRoomToolDefinitions: CapabilityToolDefinition[] = [
	dataRoomTool('dataroom_links_get', 'Read your share links. Administrators can also see links created by other employees. Access codes are never returned again.', z.strictObject({}), dataRoomLinksSchema),
	dataRoomTool('dataroom_link_add', 'Create a code-protected share link that reads as one circle, within your own permissions. Every category follows the circle and creator permissions. Lifetime defaults to three days and cannot exceed seven days. Returns a newly generated six digit code once. Recipients must acknowledge the confidentiality notice.', dataRoomLinkInputSchema, dataRoomLinkCreatedSchema, true),
	dataRoomTool('dataroom_link_delete', 'Revoke a link created by you, or any company link if you are an administrator. Existing guest sessions lose access immediately; downloaded files cannot be recalled.', dataRoomLinkRevokeSchema, recordSavedSchema, true),
	{
		name: 'company_document_classify', namespace: 'company', answeredBy: CapabilityAnsweredBy.Company,
		privacyClass: 'workspace_company', policyResource: 'tool:company_document_classify', version: '1',
		description: 'Classify a document against the company data room taxonomy in one model request. Supply its title and extracted text or a factual summary. The result is an exact permitted leaf category code, including a parent without children, or X when ambiguous. Parents with children are never choices. Classify before uploading and registering a new document.',
		estimatedLatency: CapabilityEstimatedLatency.Medium,
		inputSchema: z.strictObject({ title: z.string().min(1), text: z.string().max(100000) }),
		result: { schema: z.strictObject({ categoryCode: z.string().regex(/^[A-Z]{1,2}$/) }), effects: [] },
		sideEffect: CapabilitySideEffect.Computation
	},
	dataRoomTool('dataroom_get',
		'Read the company data room categories and its shares with people outside the company. A parent code grants all current and future children. Reads expose only permitted categories; administrators can manage the template. Circles and who belongs to them are circle_list.',
		z.strictObject({}), dataRoomGetResultSchema),
	dataRoomTool('dataroom_category_update',
		'Create or rename a data room category. Use one uppercase mnemonic letter for a parent and two for a filing category. Code identity and ancestry remain stable. X is the reserved unclassified inbox. A new child is readable by every existing recipient of its parent; confirm that audience first.',
		dataRoomCategorySetInputSchema, recordSavedSchema, true),
	dataRoomTool('dataroom_field_update',
		'Choose who sees one company page field by naming the category whose readers see it; the field leaves the category that held it. Confirm the audience first: every circle and share that reads the category sees the value.',
		dataRoomFieldSetInputSchema, recordSavedSchema, true),
	dataRoomTool('dataroom_share_add',
		'Share live data room categories with someone outside the company by lending them what one circle reads: an external email or an explicitly public audience. Current and future documents in those categories become readable. Confirm the audience and scope first. Recipients must accept using their verified email; do not invite them as company members. Company members read through the circles they belong to, set with circle_member_update. Public publication is a distinct explicit request and never includes X.',
		dataRoomShareCreateInputSchema, dataRoomShareCreatedSchema, true),
	{
		name: 'dataroom_invitation_send', namespace: 'dataroom', answeredBy: CapabilityAnsweredBy.Record,
		privacyClass: 'workspace_company', policyResource: 'tool:dataroom_invitation_send', version: '1',
		description: 'Company dataroom: send the invitation email of an email share, so its recipient can sign in with that address and accept it. Only a company administrator sends invitations; a revoked or expired share is refused. Requires approval; the email reaches someone outside the company.',
		estimatedLatency: CapabilityEstimatedLatency.Medium,
		inputSchema: dataRoomInvitationSendInputSchema,
		inputIntentSchema: dataRoomInvitationSendInputSchema.partial(),
		result: { schema: dataRoomInvitationSentSchema, effects: [{
			objectType: 'data_room_share', effect: ResourceMutationEffect.Sent,
			resultField: 'shareID', effectIdentity: ResourceEffectIdentity.ID
		}] },
		sideEffect: CapabilitySideEffect.ExternalSend,
		requiresApproval: true
	},
	{
		name: 'dataroom_invitation_accept', namespace: 'dataroom', answeredBy: CapabilityAnsweredBy.Record,
		privacyClass: 'workspace_company', policyResource: 'tool:dataroom_invitation_accept', version: '1',
		description: "Company dataroom: accept an invitation another company sent to the requester's own verified email, which opens the categories it shares to the requester. Answers the inviting company's ID. An invitation addressed to another email, revoked, expired, or accepted by someone else is refused.",
		estimatedLatency: CapabilityEstimatedLatency.Low,
		inputSchema: dataRoomInvitationAcceptInputSchema,
		inputIntentSchema: dataRoomInvitationAcceptInputSchema.partial(),
		result: { schema: dataRoomInvitationAcceptedSchema, effects: [{
			objectType: 'data_room_share', effect: ResourceMutationEffect.Updated,
			resultField: 'shareID', effectIdentity: ResourceEffectIdentity.ID
		}] },
		sideEffect: CapabilitySideEffect.WorkspaceWrite
	},
	dataRoomTool('dataroom_share_delete',
		'Revoke a data room share by its exact shareID from dataroom_get. Other active grants still apply. Already downloaded files cannot be recalled; existing signed URLs expire within ten minutes.',
		dataRoomShareRevokeInputSchema, recordSavedSchema, true)
];
