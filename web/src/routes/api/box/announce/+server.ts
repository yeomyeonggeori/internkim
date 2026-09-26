import { error, json } from '@sveltejs/kit';
import { boxAnnouncementSchema } from '$lib/company/box';
import { bearerTokenOf, environmentOfPlatform } from '$lib/server/agent-request';
import { announceBox, boxKeyOfAssertion } from '$lib/server/box';
import { controlPlane, planeCredentialsOf } from '$lib/server/control-plane';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform, getClientAddress }) => {
	const plane = planeCredentialsOf(environmentOfPlatform(platform?.env));
	if (!plane) error(500, 'the central plane is not configured');

	const publicKey = await boxKeyOfAssertion(bearerTokenOf(request));
	if (!publicKey) error(401, 'an announcement is signed by the key it names');

	const announcement = boxAnnouncementSchema.safeParse(await request.json().catch(() => null));
	if (!announcement.success) error(400, 'an announcement carries the box encryption key');

	return json(await announceBox(controlPlane(plane), publicKey, announcement.data.encryptionKey, getClientAddress()));
};
