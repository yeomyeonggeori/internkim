import { z } from 'zod';
import { dataRoomCategorySchema, dataRoomRoleSchema } from '$lib/data-room/model';
import { dataRoomGetResultSchema } from '$lib/data-room/schemas';
export { dataRoomGetResultSchema, dataRoomShareSchema } from '$lib/data-room/schemas';
import { CapabilityAnsweredBy, CapabilityEstimatedLatency, CapabilitySideEffect, ResourceEffectIdentity } from './protocol';
import { ResourceMutationEffect, type CapabilityToolDefinition } from './definition';


export const dataRoomCategorySetInputSchema = dataRoomCategorySchema;
export const dataRoomRoleSetInputSchema = dataRoomRoleSchema;
export const dataRoomShareCreateInputSchema = z.strictObject({
	roleCode: z.string(),
	audience: z.enum(['member', 'circle', 'email', 'public']),
	email: z.string().regex(/^[^\s@]+@[^\s@]+\.[^\s@]+$/).optional(),
	memberID: z.string().uuid().optional(),
	circleID: z.string().uuid().optional(),
	expiresAt: z.string().optional(),
	canDownload: z.boolean().optional()
});

export const dataRoomShareRevokeInputSchema = z.strictObject({ shareID: z.string().uuid() });
const dataRoomShareCreatedSchema = z.strictObject({ shareID: z.string().uuid() });
const dataRoomSavedSchema = z.strictObject({ saved: z.boolean() });

function dataRoomTool(
	name: string,
	description: string,
	inputSchema: z.ZodObject,
	resultSchema: z.ZodType,
	isWrite = false
): CapabilityToolDefinition {
	return {
		name, namespace: 'company', description: `Company dataroom: ${description}`,
		answeredBy: CapabilityAnsweredBy.Record,
		privacyClass: 'workspace_company', policyResource: `tool:${name}`,
		version: '1', estimatedLatency: CapabilityEstimatedLatency.Low,
		inputSchema,
		result: { schema: resultSchema, effects: isWrite ? [{
			objectType: 'data_room', effect: ResourceMutationEffect.Updated,
			effectIdentity: ResourceEffectIdentity.Singleton
		}] : [] },
		sideEffect: isWrite ? CapabilitySideEffect.WorkspaceWrite : CapabilitySideEffect.Read,
		...(isWrite ? { requiresApproval: true, approvalScope: 'company', inputIntentSchema: inputSchema.partial() } : {})
	};
}

export const dataRoomToolDefinitions: CapabilityToolDefinition[] = [
	{
		name: 'company_document_classify', namespace: 'company', answeredBy: CapabilityAnsweredBy.Company,
		privacyClass: 'workspace_company', policyResource: 'tool:company_document_classify', version: '1',
		description: 'Classify a document against the company data room taxonomy in one model request. Supply its title and extracted text or a factual summary. The result is an exact permitted leaf category code, including a parent without children, or X when ambiguous. Parents with children are never choices. Classify before uploading and registering a new document.',
		estimatedLatency: CapabilityEstimatedLatency.Medium,
		inputSchema: z.strictObject({ title: z.string().min(1), text: z.string().max(100000) }),
		result: { schema: z.strictObject({ categoryCode: z.string().regex(/^[A-Z]{1,2}$/) }), effects: [] },
		sideEffect: CapabilitySideEffect.Computation
	},
	dataRoomTool('company_dataroom_get',
		'Read the company data room categories, reader roles and active invitations. A parent code grants all current and future children. Reads expose only permitted categories; administrators can manage the template.',
		z.strictObject({}), dataRoomGetResultSchema),
	dataRoomTool('company_dataroom_category_update',
		'Create or rename a data room category. Use one uppercase mnemonic letter for a parent and two for a filing category. Code identity and ancestry remain stable. X is the reserved unclassified inbox. A new child is readable by every existing recipient of its parent; confirm that audience first.',
		dataRoomCategorySetInputSchema, dataRoomSavedSchema, true),
	dataRoomTool('company_dataroom_role_update',
		'Create or edit a reader role using readableCategories. A parent grants all descendants; otherwise name intermediate codes. Existing recipients of this role change access immediately. Read company_dataroom_get and confirm the affected recipients first. Read roles confer no administrative or editing rights.',
		dataRoomRoleSetInputSchema, dataRoomSavedSchema, true),
	dataRoomTool('company_dataroom_share_add',
		'Share live data room categories by assigning a reader role to a member, internal circle, external email or explicitly public audience. Current and future documents in the permitted categories become readable. Confirm the audience and scope first. External recipients must accept using their verified email; do not invite them as company members. Public publication is a distinct explicit request and never includes X.',
		dataRoomShareCreateInputSchema, dataRoomShareCreatedSchema, true),
	dataRoomTool('company_dataroom_share_delete',
		'Revoke a data room share by its exact shareID from company_dataroom_get. Other active grants still apply. Already downloaded files cannot be recalled; existing signed URLs expire within ten minutes.',
		dataRoomShareRevokeInputSchema, dataRoomSavedSchema, true)
];
