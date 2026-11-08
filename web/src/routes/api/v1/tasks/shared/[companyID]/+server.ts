import { error, json } from '@sveltejs/kit';
import { z } from 'zod';
import { environmentOf } from '$lib/server/agent-request';
import { answeredOrRefused } from '$lib/server/data-room-request';
import { signedInGuest } from '$lib/server/guest-request';
import { taskInvitationBoard } from '$lib/server/public-api/record/task-invitation';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ request, platform, params }) => {
	if (!z.guid().safeParse(params.companyID).success) error(404, 'shared tasks not found');
	const caller = await signedInGuest(request, environmentOf(platform));
	const board = await answeredOrRefused(taskInvitationBoard(caller, params.companyID));
	if (!board) error(403, 'no accepted invitation of this company reaches the signed-in email');
	return json(board);
};
