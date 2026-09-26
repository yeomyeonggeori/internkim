import { error, json } from '@sveltejs/kit';
import { sealedModelKeySchema } from '$lib/company/box';
import { environmentOfPlatform } from '$lib/server/agent-request';
import { BoxRefused, connectedBoxOf, keepSealedModelKey } from '$lib/server/box';
import { callingHostAdministrator } from '$lib/server/company-host-setup';
import type { RequestHandler } from './$types';

export const PUT: RequestHandler = async ({ request, platform }) => {
	const member = await callingHostAdministrator(request, environmentOfPlatform(platform?.env));
	const sealed = sealedModelKeySchema.safeParse(await request.json().catch(() => null));
	if (!sealed.success) error(400, 'a model key arrives sealed to the box encryption key');

	try {
		await keepSealedModelKey(member.record, member.companyID, sealed.data);
	} catch (refusal) {
		if (refusal instanceof BoxRefused) error(409, refusal.message);
		throw refusal;
	}
	return json({ connected: await connectedBoxOf(member.record, member.companyID) });
};
