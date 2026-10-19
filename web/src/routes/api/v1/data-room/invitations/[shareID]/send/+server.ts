import { error, json } from '@sveltejs/kit';
import { z } from 'zod';
import { environmentOf } from '$lib/server/agent-request';
import { theAppAddressAnswering } from '$lib/server/company-host-redirect';
import { answeredOrRefused, dataRoomCaller } from '$lib/server/data-room-request';
import { dataRoomInvitationSend } from '$lib/server/public-api/record/data-room';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform, params, url }) => {
	if (!z.string().uuid().safeParse(params.shareID).success) error(404, 'invitation not found');
	const environment = environmentOf(platform);
	const { caller } = await dataRoomCaller(request, environment);
	const place = { environment, appAddress: theAppAddressAnswering(environment, url) };
	const { sent } = await answeredOrRefused(dataRoomInvitationSend(caller, place, params.shareID));
	return json({ sent });
};
