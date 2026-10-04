import { error, json } from '@sveltejs/kit';
import { bearerTokenOf, environmentOfPlatform } from '$lib/server/agent-request';
import { boxKeyOfAssertion, releaseBoxOfKey } from '$lib/server/box';
import { planeCredentialsOf, controlPlane } from '$lib/server/control-plane';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOfPlatform(platform?.env);
	const plane = planeCredentialsOf(environment);
	if (!plane) error(500, 'the central plane is not configured');

	const publicKey = await boxKeyOfAssertion(bearerTokenOf(request));
	if (!publicKey) error(401, 'a box releases itself with an assertion signed by its own key');

	const wasClaimed = await releaseBoxOfKey(controlPlane(plane), publicKey);
	return json({ wasClaimed }, { headers: { 'Cache-Control': 'no-store' } });
};
