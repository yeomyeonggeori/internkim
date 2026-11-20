import { error, json } from '@sveltejs/kit';
import { boxFileClaimSchema } from '$lib/company/box';
import { bearerTokenOf, environmentOfPlatform } from '$lib/server/agent-request';
import { boxKeyOfAssertion, BoxRefused, claimBoxWithConnectionFile } from '$lib/server/box';
import { controlPlane, planeCredentialsOf } from '$lib/server/control-plane';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform }) => {
	const plane = planeCredentialsOf(environmentOfPlatform(platform?.env));
	if (!plane) error(500, 'the central plane is not configured');

	const publicKey = await boxKeyOfAssertion(bearerTokenOf(request));
	if (!publicKey) error(401, 'a claim is signed by the key it names');

	const claim = boxFileClaimSchema.safeParse(await request.json().catch(() => null));
	if (!claim.success) error(400, 'a claim carries the box encryption key and the connection file key');

	try {
		const { encryptionKey, connectionKey, replacesOtherComputers } = claim.data;
		const companyID = await claimBoxWithConnectionFile(controlPlane(plane), publicKey, encryptionKey, connectionKey, replacesOtherComputers);
		if (!companyID) error(403, 'that connection file was already used or replaced; download a new one from company setup');
		return json({ companyID });
	} catch (refusal) {
		if (refusal instanceof BoxRefused) error(refusal.status, refusal.message);
		throw refusal;
	}
};
