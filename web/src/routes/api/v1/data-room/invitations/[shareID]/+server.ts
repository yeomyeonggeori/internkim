import { error, json } from '@sveltejs/kit';
import { z } from 'zod';
import { environmentOf } from '$lib/server/agent-request';
import { answeredOrRefused, dataRoomCaller } from '$lib/server/data-room-request';
import { dataRoomShareAccept } from '$lib/server/public-api/record/data-room';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform, params }) => {
	if (!z.string().uuid().safeParse(params.shareID).success) error(404, 'invitation not found');
	const { caller } = await dataRoomCaller(request, environmentOf(platform));
	const { companyID } = await answeredOrRefused(dataRoomShareAccept(caller, params.shareID));
	return json({ companyID });
};
