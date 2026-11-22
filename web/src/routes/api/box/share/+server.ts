import { error, json } from '@sveltejs/kit';
import { z } from 'zod';
import { boxShareReportSchema } from '$lib/company/box';
import { bearerTokenOf, environmentOfPlatform } from '$lib/server/agent-request';
import { boxKeyOfAssertion, boxOfPublicKey, BoxRefused, reportBoxShare } from '$lib/server/box';
import { controlPlane, planeCredentialsOf } from '$lib/server/control-plane';
import type { RequestHandler } from './$types';

const reportedShareSchema = z.object({ report: boxShareReportSchema.nullable() }).strict();

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOfPlatform(platform?.env);
	const plane = planeCredentialsOf(environment);
	if (!plane) error(500, 'the central plane is not configured');

	const publicKey = await boxKeyOfAssertion(bearerTokenOf(request));
	if (!publicKey) error(401, 'a share is reported with an assertion signed by the box key');

	const client = controlPlane(plane);
	const box = await boxOfPublicKey(client, publicKey);
	if (!box) error(404, 'this box belongs to no company yet');

	const reported = reportedShareSchema.safeParse(await request.json().catch(() => null));
	if (!reported.success) error(400, 'report the share as the digest it was placed against, the store time it is kept through, and its counts of pieces');

	try {
		return json(await reportBoxShare(client, box, reported.data.report), { headers: { 'Cache-Control': 'no-store' } });
	} catch (refusal) {
		if (refusal instanceof BoxRefused) error(refusal.status, refusal.message);
		throw refusal;
	}
};
