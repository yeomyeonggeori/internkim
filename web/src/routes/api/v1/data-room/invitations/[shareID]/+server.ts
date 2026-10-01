import { error, json } from '@sveltejs/kit';
import { z } from 'zod';
import { environmentOf } from '$lib/server/agent-request';
import { dataRoomCaller } from '$lib/server/data-room-request';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform, params }) => {
	if (!z.string().uuid().safeParse(params.shareID).success) error(404, 'invitation not found');
	const caller = await dataRoomCaller(request, environmentOf(platform));
	const { data, error: refusal } = await caller.rpc('data_room_share_accept', { target_share: params.shareID });
	if (refusal) error(403, 'this invitation is not available to the signed-in email');
	return json({ companyID: z.string().uuid().parse(data) });
};
