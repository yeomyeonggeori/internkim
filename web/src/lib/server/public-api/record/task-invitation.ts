import { z } from 'zod';
import type { SupabaseClient } from '@supabase/supabase-js';
import { personName } from '$lib/person-name';
import { taskInviteInputSchema, taskInvitationDeleteInputSchema, taskInvitationListResultSchema } from '../catalog/task-invitation';
import type { RecordContext, ToolPlace } from './company';
import { RecordRefusedTheWrite, statusOfPostgresCode } from './tasks';
import { sendGuestInvitationEmail } from './guest-sign-in-email';
import { sharedTaskBoardSchema, type SharedTaskBoard } from '$lib/task-invitation/board';

const noDataFound = 'P0002';

const invitationRowSchema = z.object({
	id: z.string(), email: z.string(), businesses: z.array(z.string()),
	created_at: z.string(), accepted_at: z.string().nullable()
});

export function taskInvitationLandingPath(invitationID: string): string {
	return `/share/tasks/invitations/${invitationID}`;
}

async function recordCall(client: SupabaseClient, name: string, parameters: Record<string, unknown>): Promise<unknown> {
	const { data, error } = await client.rpc(name, parameters);
	if (!error) return data;
	const status = error.code === noDataFound ? 404 : statusOfPostgresCode(error.code);
	throw new RecordRefusedTheWrite(error.message, status);
}

export async function taskInvite(context: RecordContext, value: unknown, place: ToolPlace) {
	const invitation = taskInviteInputSchema.parse(value);
	const invitationID = z.string().uuid().parse(await recordCall(context.caller, 'task_invite', {
		target_company: context.companyID,
		recipient_email: invitation.email,
		invited_businesses: invitation.businesses
	}));
	await sendGuestInvitationEmail(place, {
		email: invitation.email.trim().toLowerCase(),
		landingPath: taskInvitationLandingPath(invitationID),
		companyName: await companyNameOf(context),
		inviterName: personName(context.people.find((person) => person.personID === context.requesterID)?.name ?? '', context.locale),
		locale: context.locale
	});
	return { invitationID, sent: true };
}

async function companyNameOf(context: RecordContext): Promise<string> {
	const { data, error } = await context.caller.from('company').select('name').eq('id', context.companyID).single();
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return z.object({ name: z.string() }).parse(data).name;
}

export async function taskInvitationList(context: RecordContext) {
	const { data, error } = await context.caller.from('task_invitation')
		.select('id,email,businesses,created_at,accepted_at')
		.eq('company_id', context.companyID).is('revoked_at', null)
		.order('created_at', { ascending: false });
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return taskInvitationListResultSchema.parse({
		invitations: (data ?? []).map((value) => {
			const row = invitationRowSchema.parse(value);
			return { invitationID: row.id, email: row.email, businesses: row.businesses,
				createdAt: row.created_at, acceptedAt: row.accepted_at };
		})
	});
}

export async function taskInvitationDelete(context: RecordContext, value: unknown) {
	const { invitationID } = taskInvitationDeleteInputSchema.parse(value);
	await recordCall(context.caller, 'task_invitation_revoke', { target_invitation: invitationID });
	return { invitationID };
}

export async function taskInvitationAccept(caller: SupabaseClient, invitationID: string) {
	const companyID = await recordCall(caller, 'task_invitation_accept', { target_invitation: invitationID });
	return { companyID: z.guid().parse(companyID) };
}

export async function taskInvitationBoard(caller: SupabaseClient, companyID: string): Promise<SharedTaskBoard | null> {
	const board = await recordCall(caller, 'task_invitation_board', { target_company: companyID });
	return board === null ? null : sharedTaskBoardSchema.parse(board);
}
