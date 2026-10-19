import { z } from 'zod';
import { createClient, type SupabaseClient } from '@supabase/supabase-js';
import { dataRoomGetResultSchema, dataRoomCategorySetInputSchema, dataRoomFieldSetInputSchema,
	dataRoomShareCreateInputSchema, dataRoomShareRevokeInputSchema,
	dataRoomInvitationAcceptInputSchema, dataRoomInvitationSendInputSchema } from '../catalog/data-room';
import { planeCredentialsOf } from '$lib/server/control-plane';
import type { RecordContext, ToolPlace } from './company';
import { RecordRefusedTheWrite, statusOfPostgresCode } from './tasks';
import { dataRoomLinkInputSchema, dataRoomLinkRevokeSchema, dataRoomLinksSchema, generateDataRoomCode } from '$lib/data-room/links';

const categoryRowSchema = z.object({
	code: z.string(), parent: z.string().nullable(), slug: z.string(),
	name: z.string(), name_ko: z.string(), description: z.string(), fields: z.array(z.string()),
	choice_group: z.number().nullable()
});
const shareRowSchema = z.object({
	id: z.string(), circle_id: z.string(), audience: z.enum(['email', 'public']),
	email: z.string().nullable(),
	accepted_at: z.string().nullable(), expires_at: z.string().nullable(), revoked_at: z.string().nullable(),
	can_download: z.boolean()
});

export async function companyRows(client: SupabaseClient, companyID: string, table: string): Promise<unknown[]> {
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
	const [categories, shares, canManage] = await Promise.all([
		companyRows(context.caller, context.companyID, 'data_room_category'),
		companyRows(context.caller, context.companyID, 'data_room_share'),
		dataRoomCall(context.caller, 'data_room_administrator', { target_company: context.companyID })
	]);
	return dataRoomGetResultSchema.parse({
		categories: categories.map((value) => {
			const row = categoryRowSchema.parse(value);
			return { code: row.code, parent: row.parent, slug: row.slug,
				name: row.name, nameKO: row.name_ko, description: row.description, fields: row.fields,
				choiceGroup: row.choice_group };
		}),
		shares: shares.map(answeredShare), canManage
	});
}

function answeredShare(value: unknown) {
	const row = shareRowSchema.parse(value);
	return { id: row.id, circleID: row.circle_id, audience: row.audience,
		email: row.email,
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

export async function companyDataRoomFieldSet(context: RecordContext, value: unknown) {
	const input = dataRoomFieldSetInputSchema.parse(value);
	await dataRoomCall(context.caller, 'data_room_field_set', {
		target_company: context.companyID, target_field: input.field, target_category: input.categoryCode ?? null
	});
	return { saved: true };
}

export async function companyDataRoomShareCreate(context: RecordContext, value: unknown) {
	const share = dataRoomShareCreateInputSchema.parse(value);
	const shareID = await dataRoomCall(context.caller, 'data_room_share_create', {
		target_company: context.companyID, circle_id: share.circleID, audience: share.audience,
		recipient_email: share.email,
		expires_at: share.expiresAt, can_download: share.canDownload ?? false
	});
	return { shareID: z.string().uuid().parse(shareID) };
}

export async function companyDataRoomShareRevoke(context: RecordContext, value: unknown) {
	const share = dataRoomShareRevokeInputSchema.parse(value);
	await dataRoomCall(context.caller, 'data_room_share_revoke', { target_share: share.shareID });
	return { saved: true };
}

export async function companyDataRoomLinksGet(context: RecordContext) {
	const [shareableCircleIDs, downloadableCircleIDs] = await Promise.all([
		dataRoomCall(context.caller, 'data_room_shareable_circles', { target_company: context.companyID }),
		dataRoomCall(context.caller, 'data_room_shareable_circles', { target_company: context.companyID, download: true })
	]);
	const { data, error } = await context.caller.from('data_room_link')
		.select('id,circle_id,label,can_download,created_at,expires_at,revoked_at')
		.eq('company_id', context.companyID).order('created_at', { ascending: false });
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return dataRoomLinksSchema.parse({ shareableCircleIDs, downloadableCircleIDs, links: (data ?? []).map((link) => ({
		id: link.id, circleID: link.circle_id, label: link.label, canDownload: link.can_download,
		createdAt: link.created_at, expiresAt: link.expires_at, revokedAt: link.revoked_at
	})) });
}

export async function companyDataRoomLinkCreate(context: RecordContext, value: unknown) {
	const link = dataRoomLinkInputSchema.parse(value);
	const accessCode = generateDataRoomCode();
	const linkID = await dataRoomCall(context.caller, 'data_room_link_create', {
		target_company: context.companyID, circle_id: link.circleID, label: link.label,
		access_code: accessCode, lifetime_hours: link.lifetimeHours, can_download: link.canDownload
	});
	return { linkID: z.string().uuid().parse(linkID), accessCode };
}

export async function companyDataRoomLinkRevoke(context: RecordContext, value: unknown) {
	const link = dataRoomLinkRevokeSchema.parse(value);
	await dataRoomCall(context.caller, 'data_room_link_revoke', { target_link: link.linkID });
	return { saved: true };
}

const invitationRowSchema = z.object({
	company_id: z.string(), email: z.string(),
	expires_at: z.string().nullable(), revoked_at: z.string().nullable()
});

export async function dataRoomShareAccept(caller: SupabaseClient, shareID: string) {
	const companyID = await dataRoomCall(caller, 'data_room_share_accept', { target_share: shareID });
	return { shareID, companyID: z.string().uuid().parse(companyID) };
}

export async function dataRoomInvitationSend(caller: SupabaseClient, place: ToolPlace, shareID: string) {
	const invitation = await invitationOf(caller, shareID);
	const isAdministrator = await dataRoomCall(caller, 'data_room_administrator', { target_company: invitation.company_id });
	if (isAdministrator !== true) throw new RecordRefusedTheWrite('only an administrator sends invitations', 403);
	if (!isInvitationActive(invitation)) throw new RecordRefusedTheWrite('invitation is no longer active', 403);
	await sendInvitationEmail(place, invitation.email, shareID);
	return { shareID, sent: true };
}

async function invitationOf(caller: SupabaseClient, shareID: string) {
	const { data, error } = await caller.from('data_room_share')
		.select('company_id,email,expires_at,revoked_at').eq('id', shareID).eq('audience', 'email').maybeSingle();
	if (error || !data) throw new RecordRefusedTheWrite('invitation not found', 404);
	return invitationRowSchema.parse(data);
}

function isInvitationActive(invitation: z.infer<typeof invitationRowSchema>): boolean {
	if (invitation.revoked_at) return false;
	return !invitation.expires_at || new Date(invitation.expires_at) > new Date();
}

async function sendInvitationEmail(place: ToolPlace, email: string, shareID: string) {
	const plane = planeCredentialsOf(place.environment);
	if (!plane) throw new RecordRefusedTheWrite('the data room is not configured', 503);
	const sender = createClient(plane.projectURL, plane.publishableKey, {
		auth: { persistSession: false, autoRefreshToken: false }
	});
	const { error } = await sender.auth.signInWithOtp({ email, options: {
		shouldCreateUser: true, emailRedirectTo: `${place.appAddress}/share/invitations/${shareID}`
	} });
	if (error) throw new RecordRefusedTheWrite(error.message, error.status ?? 502);
}

export async function companyDataRoomInvitationSend(context: RecordContext, value: unknown, place: ToolPlace) {
	const { shareID } = dataRoomInvitationSendInputSchema.parse(value);
	return dataRoomInvitationSend(context.caller, place, shareID);
}

export async function companyDataRoomInvitationAccept(context: RecordContext, value: unknown) {
	const { shareID } = dataRoomInvitationAcceptInputSchema.parse(value);
	return dataRoomShareAccept(context.caller, shareID);
}
