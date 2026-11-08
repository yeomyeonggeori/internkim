import { error, json } from '@sveltejs/kit';
import { z } from 'zod';
import { environmentOf } from '$lib/server/agent-request';
import { answeredOrRefused } from '$lib/server/data-room-request';
import { signedInGuest } from '$lib/server/guest-request';
import { taskInvitationAccept } from '$lib/server/public-api/record/task-invitation';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform, params }) => {
	if (!z.string().uuid().safeParse(params.invitationID).success) error(404, 'invitation not found');
	const caller = await signedInGuest(request, environmentOf(platform));
	return json(await answeredOrRefused(taskInvitationAccept(caller, params.invitationID)));
};
