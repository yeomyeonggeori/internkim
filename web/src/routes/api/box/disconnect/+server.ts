import { error } from '@sveltejs/kit';
import { boxDisconnectionSchema } from '$lib/company/box';
import { bearerTokenOf, environmentOfPlatform } from '$lib/server/agent-request';
import { boxKeyOfAssertion, disconnectComputer } from '$lib/server/box';
import { controlPlane, planeCredentialsOf } from '$lib/server/control-plane';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform }) => {
	const plane = planeCredentialsOf(environmentOfPlatform(platform?.env));
	if (!plane) error(500, 'the central plane is not configured');

	const publicKey = await boxKeyOfAssertion(bearerTokenOf(request));
	if (!publicKey) error(401, 'a computer disconnects itself with an assertion signed by its own box key');

	const disconnection = boxDisconnectionSchema.safeParse(await request.json().catch(() => null));
	if (!disconnection.success) error(400, 'a disconnection carries nothing, or the key of the connection file the computer serves on');

	await disconnectComputer(controlPlane(plane), publicKey, disconnection.data.connectionKey);
	return new Response(null, { status: 204, headers: { 'Cache-Control': 'no-store' } });
};
