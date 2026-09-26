import { json } from '@sveltejs/kit';
import { environmentOf } from '$lib/server/agent-request';
import { callingScheduledJob } from '$lib/server/scheduled-job-request';
import { askAboutUnclosedShifts } from '$lib/server/unclosed-shifts';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOf(platform);
	const client = await callingScheduledJob(request, environment);
	return json(await askAboutUnclosedShifts(environment, client, new Date()));
};
