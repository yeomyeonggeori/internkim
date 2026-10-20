import { error, json } from '@sveltejs/kit';
import { z } from 'zod';
import { bearerTokenOf, environmentOfPlatform } from '$lib/server/agent-request';
import { boxKeyOfAssertion, boxSessionFor } from '$lib/server/box';
import { planeCredentialsOf } from '$lib/server/control-plane';
import type { RequestHandler } from './$types';

const reportedReleaseSchema = z.object({ packageVersion: z.string().max(80).optional() });

export const POST: RequestHandler = async ({ request, platform, url }) => {
	const environment = environmentOfPlatform(platform?.env);
	const plane = planeCredentialsOf(environment);
	if (!plane) error(500, 'the central plane is not configured');

	const publicKey = await boxKeyOfAssertion(bearerTokenOf(request));
	if (!publicKey) error(401, 'a session is asked for with an assertion signed by the box key');

	const reported = reportedReleaseSchema.safeParse(await request.json().catch(() => ({})));
	const session = await boxSessionFor(plane, publicKey, environment, url.origin, reported.success ? reported.data.packageVersion ?? null : null);
	if (!session) error(404, 'this box belongs to no company yet');
	return json(session, { headers: { 'Cache-Control': 'no-store' } });
};
