import { error, json } from '@sveltejs/kit';
import { bearerTokenOf, environmentOfPlatform } from '$lib/server/agent-request';
import { boxKeyOfAssertion, boxOfPublicKey } from '$lib/server/box';
import { noteAdminAccount, pendingAdminPasswordOf } from '$lib/server/box-admin-password';
import { planeCredentialsOf, controlPlane } from '$lib/server/control-plane';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOfPlatform(platform?.env);
	const plane = planeCredentialsOf(environment);
	if (!plane) error(500, 'the central plane is not configured');

	const publicKey = await boxKeyOfAssertion(bearerTokenOf(request));
	if (!publicKey) error(401, 'an admin password is asked for with an assertion signed by the box key');

	const client = controlPlane(plane);
	const box = await boxOfPublicKey(client, publicKey);
	if (!box) error(404, 'this box belongs to no company yet');

	await noteAdminAccount(client, box);
	return json({ companyID: box.companyID, change: pendingAdminPasswordOf(box) }, { headers: { 'Cache-Control': 'no-store' } });
};
