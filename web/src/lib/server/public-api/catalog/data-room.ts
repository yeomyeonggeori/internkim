import { z } from 'zod';
import { companyFieldKindSchema, dataRoomCategorySchema } from '$lib/data-room/model';
import { dataRoomGetResultSchema } from '$lib/data-room/schemas';
import { dataRoomLinkInputSchema, dataRoomLinksSchema, dataRoomLinkCreatedSchema, dataRoomLinkRevokeSchema } from '$lib/data-room/links';
export { dataRoomGetResultSchema } from '$lib/data-room/schemas';
import { CapabilityAnsweredBy, CapabilityEstimatedLatency, CapabilitySideEffect, ResourceEffectIdentity } from './protocol';
import { ResourceMutationEffect, type CapabilityToolDefinition } from './definition';
import type { CapabilityNamespace } from './namespaces';

export const dataRoomCategorySetInputSchema = dataRoomCategorySchema.omit({ fields: true });
const companyFieldIdentity = {
	kind: companyFieldKindSchema.describe("What the field is: 'profile' for a company_info key, 'activity' or 'ownership'; 'metric' for a company_metric key; 'record' for a company_record category"),
	field: z.string().min(1).describe("Exact field as stored: a company_info key such as 'capital', a company_metric key such as 'annualRevenue', or a company_record category such as 'funding'.")
};
export const dataRoomFieldSetInputSchema = z.strictObject({
	...companyFieldIdentity,
	categoryCode: z.string().regex(/^[A-Z]{1,2}$/).describe('Exact code from dataroom_get whose readers should see the field. Omit to show it only to readers of X.').optional()
});
export const dataRoomFieldNameSetInputSchema = z.strictObject({
	...companyFieldIdentity,
	name: z.string().trim().min(1).describe("What the company page calls the field in English, e.g. 'Installed units'. Omit to show the product's English name.").optional(),
	nameKO: z.string().trim().min(1).describe("What the company page calls the field in Korean. Omit to show the product's Korean name.").optional()
});
export const dataRoomPublicationUpdateInputSchema = z.strictObject({
	isPublic: z.boolean().describe('true puts it on the public page, false takes it off.'),
	documentHint: z.string().min(1).describe('Exact documentID or exact current title from company_document_list or company_document_search, for publishing one document. Give this or kind and field, never both.').optional(),
	kind: companyFieldIdentity.kind.describe('The kind of the company page field to publish, the same kind dataroom_field_update takes. Give it with field.').optional(),
	field: companyFieldIdentity.field.describe('Exact company page field as stored, the same key dataroom_field_update takes, for publishing one field. Give it with kind, never with documentHint.').optional()
});
export const dataRoomPublicationUpdatedSchema = z.strictObject({
	documentID: z.string().optional(), kind: companyFieldKindSchema.optional(), field: z.string().optional(), isPublic: z.boolean()
});
export const dataRoomLinksGetInputSchema = z.strictObject({});
export const dataRoomGetInputSchema = z.strictObject({});
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
		...(isWrite ? { requiresApproval: true, inputIntentSchema: inputSchema.partial() } : {})
	});
}

const dataRoomTool = companyRecordTool('dataroom', 'data_room', 'Company dataroom');

export const dataRoomToolDefinitions: CapabilityToolDefinition[] = [
	dataRoomTool('dataroom_links_get', 'Read your share links. Administrators can also see links created by other employees. Access codes are never returned again.', dataRoomLinksGetInputSchema, dataRoomLinksSchema),
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
		'Read the company data room categories, its company page fields, and what its public page shows. A parent code grants all current and future children. Reads expose only permitted categories; administrators can manage the template. Circles and who belongs to them are circle_list.',
		dataRoomGetInputSchema, dataRoomGetResultSchema),
	dataRoomTool('dataroom_category_update',
		'Create or rename a data room category. Use one uppercase mnemonic letter for a parent and two for a filing category. Code identity and ancestry remain stable. X is the reserved unclassified inbox. A new child is readable by every existing recipient of its parent; confirm that audience first.',
		dataRoomCategorySetInputSchema, recordSavedSchema, true),
	dataRoomTool('dataroom_field_update',
		'Choose who sees one company page field by naming the category whose readers see it; the field leaves the category that held it. Confirm the audience first: every circle and code link that reads the category sees the value. Whether the public page shows it is dataroom_publication_update.',
		dataRoomFieldSetInputSchema, recordSavedSchema, true),
	dataRoomTool('dataroom_field_name_update',
		"Name one company page field the way the company says it, in English and in Korean. Each call sets both names; a language left out shows the product's name, or the field as written when the product has none.",
		dataRoomFieldNameSetInputSchema, recordSavedSchema, true),
	dataRoomTool('dataroom_publication_update',
		'Update one publication on the company\'s public data room page: put one document or one company page field on it, or take it off. The public page is always at <company address>/room and anyone can open it without signing in; it shows only the company name and logo until something is published. A published document can be read and downloaded by anyone, and a downloaded copy cannot be recalled, so confirm the exact document or field first. A document still in X is classified before it is published. Only a company administrator publishes.',
		dataRoomPublicationUpdateInputSchema, dataRoomPublicationUpdatedSchema, true)
];
