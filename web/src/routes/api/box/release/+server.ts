import { error, json } from '@sveltejs/kit';
import { z } from 'zod';
import { bearerTokenOf, environmentOfPlatform } from '$lib/server/agent-request';
import { aimStandbyBoxesAt, boxKeyOfAssertion, BoxRefused } from '$lib/server/box';
import { controlPlane, planeCredentialsOf } from '$lib/server/control-plane';
import type { RequestHandler } from './$types';

const aimedReleaseSchema = z.object({ targetVersion: z.string().min(1).max(80) }).strict();

export const POST: RequestHandler = async ({ request, platform }) => {
	const plane = planeCredentialsOf(environmentOfPlatform(platform?.env));
	if (!plane) error(500, 'the central plane is not configured');

	const publicKey = await boxKeyOfAssertion(bearerTokenOf(request));
	if (!publicKey) error(401, 'a release is aimed at with an assertion signed by the box key');

	const aimed = aimedReleaseSchema.safeParse(await request.json().catch(() => null));
	if (!aimed.success) error(400, 'name the release the service box is about to install as targetVersion');

	try {
		const shortOfIt = await aimStandbyBoxesAt(controlPlane(plane), publicKey, aimed.data.targetVersion);
		return json({ standbyBoxesShortOfIt: shortOfIt }, { headers: { 'Cache-Control': 'no-store' } });
	} catch (refusal) {
		if (refusal instanceof BoxRefused) error(refusal.status, refusal.message);
		throw refusal;
	}
};
