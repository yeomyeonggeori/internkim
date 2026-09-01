import { json } from '@sveltejs/kit';
import { callingAgent, environmentOf } from '$lib/server/agent-request';
import { askAboutUnclosedShifts } from '$lib/server/unclosed-shifts';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOf(platform);
	const { client } = await callingAgent(request, environment);
	return json(await askAboutUnclosedShifts(environment, client, new Date()));
};
