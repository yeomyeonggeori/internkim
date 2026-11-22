import { error, json } from '@sveltejs/kit';
import { logPositionSchema } from '$lib/company/box';
import { bearerTokenOf, environmentOfPlatform } from '$lib/server/agent-request';
import { boxKeyOfAssertion, noteBoxMove } from '$lib/server/box';
import { controlPlane, planeCredentialsOf } from '$lib/server/control-plane';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOfPlatform(platform?.env);
	const plane = planeCredentialsOf(environment);
	if (!plane) error(500, 'the central plane is not configured');

	const publicKey = await boxKeyOfAssertion(bearerTokenOf(request));
	if (!publicKey) error(401, 'a move is reported with an assertion signed by the box key');

	const position = logPositionSchema.safeParse(await request.json().catch(() => null));
	if (!position.success) {
		error(400, 'say where the company\'s logs end: the WAL timeline and position, and the file journal entry or null');
	}
	const outcome = await noteBoxMove(controlPlane(plane), publicKey, position.data);
	return json({ outcome }, { headers: { 'Cache-Control': 'no-store' } });
};
