import { error, json } from '@sveltejs/kit';
import { z } from 'zod';
import { environmentOf } from '$lib/server/agent-request';
import { websitePreviewOf } from '$lib/server/company-website-preview';
import { websiteShownTo } from '$lib/server/data-room-company';
import { dataRoomCaller } from '$lib/server/data-room-request';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ request, platform, params }) => {
	if (!z.guid().safeParse(params.companyID).success) error(404, 'data room not found');
	const { caller } = await dataRoomCaller(request, environmentOf(platform));
	const website = await websiteShownTo(caller, params.companyID);
	return json({ preview: website ? await websitePreviewOf(website) : null });
};
