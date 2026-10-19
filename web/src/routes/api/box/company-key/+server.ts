import { error, json } from '@sveltejs/kit';
import { z } from 'zod';
import { boxKeySchema, sealedSecretSchema } from '$lib/company/box';
import { bearerTokenOf, environmentOfPlatform } from '$lib/server/agent-request';
import { boxKeyOfAssertion, boxOfPublicKey, BoxRefused, keepSealedCompanyKeys } from '$lib/server/box';
import { controlPlane, planeCredentialsOf } from '$lib/server/control-plane';
import type { RequestHandler } from './$types';

const handedOnKeysSchema = z.object({
	sealedKeys: z.array(z.object({ publicKey: boxKeySchema, sealed: sealedSecretSchema }).strict()).min(1).max(64)
}).strict();

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOfPlatform(platform?.env);
	const plane = planeCredentialsOf(environment);
	if (!plane) error(500, 'the central plane is not configured');

	const publicKey = await boxKeyOfAssertion(bearerTokenOf(request));
	if (!publicKey) error(401, 'the company key is handed on with an assertion signed by the box key');

	const client = controlPlane(plane);
	const holder = await boxOfPublicKey(client, publicKey);
	if (!holder) error(404, 'this box belongs to no company yet');

	const handedOn = handedOnKeysSchema.safeParse(await request.json().catch(() => null));
	if (!handedOn.success) error(400, 'name each waiting box by its public key with the company key sealed to it');

	try {
		await keepSealedCompanyKeys(client, holder, handedOn.data.sealedKeys);
	} catch (refusal) {
		if (refusal instanceof BoxRefused) error(refusal.status, refusal.message);
		throw refusal;
	}
	return json({}, { headers: { 'Cache-Control': 'no-store' } });
};
