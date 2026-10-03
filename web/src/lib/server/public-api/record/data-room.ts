import { z } from 'zod';
import type { SupabaseClient } from '@supabase/supabase-js';
import { dataRoomGetResultSchema, dataRoomCategorySetInputSchema, dataRoomRoleSetInputSchema,
	dataRoomShareCreateInputSchema, dataRoomShareRevokeInputSchema, dataRoomMemberRolesInputSchema } from '../catalog/data-room';
import type { RecordContext } from './company';
import { RecordRefusedTheWrite, statusOfPostgresCode } from './tasks';
import { dataRoomLinkInputSchema, dataRoomLinkRevokeSchema, dataRoomLinksSchema, generateDataRoomCode } from '$lib/data-room/links';

const categoryRowSchema = z.object({
	code: z.string(), parent: z.string().nullable(), slug: z.string(),
	name: z.string(), name_ko: z.string(), description: z.string(), choice_group: z.number().nullable()
});
const roleRowSchema = z.object({ code: z.string(), name: z.string(), name_ko: z.string() });
const permissionRowSchema = z.object({ role_code: z.string(), category_code: z.string() });
const shareRowSchema = z.object({
	id: z.string(), role_code: z.string(), audience: z.enum(['member', 'email', 'public']),
	email: z.string().nullable(), member_id: z.string().nullable(),
	accepted_at: z.string().nullable(), expires_at: z.string().nullable(), revoked_at: z.string().nullable(),
	can_download: z.boolean()
});

async function dataRoomRows(client: SupabaseClient, companyID: string, table: string): Promise<unknown[]> {
	const query = client.from(table).select('*').eq('company_id', companyID);
	const { data, error } = await (table === 'data_room_category' ? query.order('position') : query);
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return data ?? [];
}

export async function dataRoomCall(client: SupabaseClient, name: string, parameters: Record<string, unknown>): Promise<unknown> {
	const { data, error } = await client.rpc(name, parameters);
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return data;
}

export async function companyDataRoomGet(context: RecordContext) {
	const [categories, roles, permissions, shares, canManage] = await Promise.all([
		dataRoomRows(context.caller, context.companyID, 'data_room_category'),
		dataRoomRows(context.caller, context.companyID, 'data_room_role'),
		dataRoomRows(context.caller, context.companyID, 'data_room_role_category'),
		dataRoomRows(context.caller, context.companyID, 'data_room_share'),
		dataRoomCall(context.caller, 'data_room_administrator', { target_company: context.companyID })
	]);
	const grants = z.array(permissionRowSchema).parse(permissions);
	return dataRoomGetResultSchema.parse({
		categories: categories.map((value) => {
			const row = categoryRowSchema.parse(value);
			return { code: row.code, parent: row.parent, slug: row.slug,
				name: row.name, nameKO: row.name_ko, description: row.description, choiceGroup: row.choice_group };
		}),
		roles: roles.map((value) => {
			const row = roleRowSchema.parse(value);
			return { code: row.code, name: row.name, nameKO: row.name_ko,
				readableCategories: grants.filter((grant) => grant.role_code === row.code).map((grant) => grant.category_code) };
		}),
		shares: shares.map(answeredShare), canManage
	});
}

function answeredShare(value: unknown) {
	const row = shareRowSchema.parse(value);
	return { id: row.id, roleCode: row.role_code, audience: row.audience,
		email: row.email, memberID: row.member_id,
		acceptedAt: row.accepted_at, expiresAt: row.expires_at, revokedAt: row.revoked_at,
		canDownload: row.can_download };
}

export async function companyDataRoomCategorySet(context: RecordContext, value: unknown) {
	const category = dataRoomCategorySetInputSchema.parse(value);
	const { error } = await context.caller.from('data_room_category').upsert({
		company_id: context.companyID, code: category.code, parent: category.parent, slug: category.slug,
		name: category.name, name_ko: category.nameKO, description: category.description,
		...(category.choiceGroup === undefined ? {} : { choice_group: category.choiceGroup })
	});
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return { saved: true };
}

export async function companyDataRoomRoleSet(context: RecordContext, value: unknown) {
	const role = dataRoomRoleSetInputSchema.parse(value);
	await dataRoomCall(context.caller, 'data_room_role_set', {
		target_company: context.companyID, role_code: role.code, role_name: role.name,
		role_name_ko: role.nameKO, readable_categories: role.readableCategories
	});
	return { saved: true };
}

export async function companyDataRoomShareCreate(context: RecordContext, value: unknown) {
	const share = dataRoomShareCreateInputSchema.parse(value);
	const shareID = await dataRoomCall(context.caller, 'data_room_share_create', {
		target_company: context.companyID, role_code: share.roleCode, audience: share.audience,
		recipient_email: share.email, recipient_member: share.memberID,
		expires_at: share.expiresAt, can_download: share.canDownload ?? false
	});
	return { shareID: z.string().uuid().parse(shareID) };
}

export async function companyDataRoomShareRevoke(context: RecordContext, value: unknown) {
	const share = dataRoomShareRevokeInputSchema.parse(value);
	await dataRoomCall(context.caller, 'data_room_share_revoke', { target_share: share.shareID });
	return { saved: true };
}

export async function companyDataRoomMemberRolesSet(context: RecordContext, value: unknown) {
	const assignment = dataRoomMemberRolesInputSchema.parse(value);
	await dataRoomCall(context.caller, 'data_room_member_roles_set', {
		target_company: context.companyID, target_member: assignment.memberID, role_codes: assignment.roleCodes
	});
	return { saved: true };
}

export async function companyDataRoomLinksGet(context: RecordContext) {
	const [shareableRoleCodes, downloadableRoleCodes] = await Promise.all([
		dataRoomCall(context.caller, 'data_room_shareable_roles', { target_company: context.companyID }),
		dataRoomCall(context.caller, 'data_room_shareable_roles', { target_company: context.companyID, download: true })
	]);
	const { data, error } = await context.caller.from('data_room_link')
		.select('id,role_code,label,can_download,created_at,expires_at,revoked_at')
		.eq('company_id', context.companyID).order('created_at', { ascending: false });
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return dataRoomLinksSchema.parse({ shareableRoleCodes, downloadableRoleCodes, links: (data ?? []).map((link) => ({
		id: link.id, roleCode: link.role_code, label: link.label, canDownload: link.can_download,
		createdAt: link.created_at, expiresAt: link.expires_at, revokedAt: link.revoked_at
	})) });
}

export async function companyDataRoomLinkCreate(context: RecordContext, value: unknown) {
	const link = dataRoomLinkInputSchema.parse(value);
	const accessCode = generateDataRoomCode();
	const linkID = await dataRoomCall(context.caller, 'data_room_link_create', {
		target_company: context.companyID, role_code: link.roleCode, label: link.label,
		access_code: accessCode, lifetime_hours: link.lifetimeHours, can_download: link.canDownload
	});
	return { linkID: z.string().uuid().parse(linkID), accessCode };
}

export async function companyDataRoomLinkRevoke(context: RecordContext, value: unknown) {
	const link = dataRoomLinkRevokeSchema.parse(value);
	await dataRoomCall(context.caller, 'data_room_link_revoke', { target_link: link.linkID });
	return { saved: true };
}
