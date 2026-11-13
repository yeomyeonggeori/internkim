import { z } from 'zod';
import type { SupabaseClient } from '@supabase/supabase-js';
import { dataRoomGetResultSchema, dataRoomCategorySetInputSchema, dataRoomFieldSetInputSchema, dataRoomFieldNameSetInputSchema,
	dataRoomPublicationUpdateInputSchema } from '../catalog/data-room';
import type { RecordContext } from './company';
import { companyDocumentOfHint } from './company-ledger-tools';
import { companyFieldKindSchema, type CompanyFieldKind } from '$lib/data-room/model';
import { fieldNamesOf, fieldNamesSchema } from '$lib/data-room/schemas';
import { dataRoomLinkInputSchema, dataRoomLinkRevokeSchema, dataRoomLinksSchema, generateDataRoomCode } from '$lib/data-room/links';
import { InputRefused, refusalOfPostgresError } from '../refusal';

const categoryRowSchema = z.object({
	code: z.string(), parent: z.string().nullable(), slug: z.string(),
	name: z.string(), name_ko: z.string(), description: z.string(),
	choice_group: z.number().nullable()
});
const fieldRowSchema = z.object({
	kind: companyFieldKindSchema, field: z.string(), category_code: z.string().nullable(),
	names: fieldNamesSchema.nullable(), unit: z.string().nullable(), is_public: z.boolean()
});

export async function companyRows(client: SupabaseClient, companyID: string, table: string): Promise<unknown[]> {
	const query = client.from(table).select('*').eq('company_id', companyID);
	const { data, error } = await (table === 'data_room_category' ? query.order('position') : query);
	if (error) throw refusalOfPostgresError(error);
	return data ?? [];
}

export async function dataRoomCall(client: SupabaseClient, name: string, parameters: Record<string, unknown>): Promise<unknown> {
	const { data, error } = await client.rpc(name, parameters);
	if (error) throw refusalOfPostgresError(error);
	return data;
}

export async function companyDataRoomGet(context: RecordContext) {
	const [categories, fields, publicDocumentIDs, canManage] = await Promise.all([
		companyRows(context.caller, context.companyID, 'data_room_category'),
		companyRows(context.caller, context.companyID, 'company_field'),
		publicDocumentIDsOf(context),
		dataRoomCall(context.caller, 'data_room_administrator', { target_company: context.companyID })
	]);
	return dataRoomGetResultSchema.parse({
		categories: categories.map((value) => {
			const row = categoryRowSchema.parse(value);
			return { code: row.code, parent: row.parent, slug: row.slug,
				name: row.name, nameKO: row.name_ko, description: row.description,
				choiceGroup: row.choice_group };
		}),
		fields: fields.map((value) => {
			const row = fieldRowSchema.parse(value);
			return { kind: row.kind, field: row.field, categoryCode: row.category_code,
				name: row.names?.en ?? null, nameKO: row.names?.ko ?? null, unit: row.unit, isPublic: row.is_public };
		}),
		publicDocumentIDs, canManage
	});
}

async function publicDocumentIDsOf(context: RecordContext): Promise<string[]> {
	const { data, error } = await context.caller.from('company_document').select('id')
		.eq('company_id', context.companyID).not('public_since', 'is', null).order('id');
	if (error) throw refusalOfPostgresError(error);
	return (data ?? []).map((row) => z.object({ id: z.string() }).parse(row).id);
}

export async function companyDataRoomCategorySet(context: RecordContext, category: z.infer<typeof dataRoomCategorySetInputSchema>) {
	const { error } = await context.caller.from('data_room_category').upsert({
		company_id: context.companyID, code: category.code, parent: category.parent, slug: category.slug,
		name: category.name, name_ko: category.nameKO, description: category.description,
		...(category.choiceGroup === undefined ? {} : { choice_group: category.choiceGroup })
	});
	if (error) throw refusalOfPostgresError(error);
	return { saved: true };
}

export async function companyDataRoomFieldSet(context: RecordContext, input: z.infer<typeof dataRoomFieldSetInputSchema>) {
	await upsertCompanyField(context, { kind: input.kind, field: input.field, category_code: input.categoryCode ?? null });
	return { saved: true };
}

export async function companyDataRoomFieldNameSet(context: RecordContext, input: z.infer<typeof dataRoomFieldNameSetInputSchema>) {
	await upsertCompanyField(context, { kind: input.kind, field: input.field, names: fieldNamesOf(input) });
	return { saved: true };
}

type CompanyFieldWrite = {
	kind: CompanyFieldKind;
	field: string;
	category_code?: string | null;
	names?: Record<string, string> | null;
	is_public?: boolean;
};

async function upsertCompanyField(context: RecordContext, write: CompanyFieldWrite) {
	const { error } = await context.caller.from('company_field')
		.upsert({ company_id: context.companyID, ...write }, { onConflict: 'company_id,kind,field' });
	if (error) throw refusalOfPostgresError(error);
}

export async function companyDataRoomPublicationUpdate(context: RecordContext, publication: z.infer<typeof dataRoomPublicationUpdateInputSchema>) {
	const { documentHint, kind, field, isPublic } = publication;
	if (documentHint !== undefined && kind === undefined && field === undefined) return publishDocument(context, documentHint, isPublic);
	if (documentHint === undefined && kind !== undefined && field !== undefined) return publishField(context, kind, field, isPublic);
	throw new InputRefused('name one document with documentHint, or one field with kind and field, never both');
}

async function publishDocument(context: RecordContext, documentHint: string, isPublic: boolean) {
	const document = await companyDocumentOfHint(context, documentHint);
	await dataRoomCall(context.caller, 'data_room_document_publication_set', { target_document: document.id, is_public: isPublic });
	return { documentID: document.id, isPublic };
}

async function publishField(context: RecordContext, kind: CompanyFieldKind, field: string, isPublic: boolean) {
	await upsertCompanyField(context, { kind, field, is_public: isPublic });
	return { kind, field, isPublic };
}

export async function companyDataRoomLinksGet(context: RecordContext) {
	const [shareableCircleIDs, downloadableCircleIDs] = await Promise.all([
		dataRoomCall(context.caller, 'data_room_shareable_circles', { target_company: context.companyID }),
		dataRoomCall(context.caller, 'data_room_shareable_circles', { target_company: context.companyID, download: true })
	]);
	const { data, error } = await context.caller.from('data_room_link')
		.select('id,circle_id,label,can_download,created_at,expires_at,revoked_at')
		.eq('company_id', context.companyID).order('created_at', { ascending: false });
	if (error) throw refusalOfPostgresError(error);
	return dataRoomLinksSchema.parse({ shareableCircleIDs, downloadableCircleIDs, links: (data ?? []).map((link) => ({
		id: link.id, circleID: link.circle_id, label: link.label, canDownload: link.can_download,
		createdAt: link.created_at, expiresAt: link.expires_at, revokedAt: link.revoked_at
	})) });
}

export async function companyDataRoomLinkCreate(context: RecordContext, link: z.infer<typeof dataRoomLinkInputSchema>) {
	const accessCode = generateDataRoomCode();
	const linkID = await dataRoomCall(context.caller, 'data_room_link_create', {
		target_company: context.companyID, circle_id: link.circleID, label: link.label,
		access_code: accessCode, lifetime_hours: link.lifetimeHours, can_download: link.canDownload
	});
	return { linkID: z.string().uuid().parse(linkID), accessCode };
}

export async function companyDataRoomLinkRevoke(context: RecordContext, link: z.infer<typeof dataRoomLinkRevokeSchema>) {
	await dataRoomCall(context.caller, 'data_room_link_revoke', { target_link: link.linkID });
	return { saved: true };
}
