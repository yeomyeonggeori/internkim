import { error, json } from '@sveltejs/kit';
import { boxClaimSchema } from '$lib/company/box';
import { environmentOfPlatform } from '$lib/server/agent-request';
import { BoxRefused, claimBox, connectedBoxOf, emptyBoxesAt } from '$lib/server/box';
import { callingHostAdministrator } from '$lib/server/company-host-setup';
import type { RequestHandler } from './$types';

const privateHeaders = { 'Cache-Control': 'no-store', 'Pragma': 'no-cache' };

export const GET: RequestHandler = async ({ request, platform, getClientAddress }) => {
	const member = await callingHostAdministrator(request, environmentOfPlatform(platform?.env));
	return json(
		{
			connected: await connectedBoxOf(member.record, member.companyID),
			empty: await emptyBoxesAt(member.record, getClientAddress())
		},
		{ headers: privateHeaders }
	);
};

export const POST: RequestHandler = async ({ request, platform }) => {
	const member = await callingHostAdministrator(request, environmentOfPlatform(platform?.env));
	const claim = boxClaimSchema.safeParse(await request.json().catch(() => null));
	if (!claim.success) error(400, 'name the box to connect by its public key and the code it shows');

	try {
		await claimBox(member.record, member.companyID, claim.data.publicKey, claim.data.pairingCode);
	} catch (refusal) {
		if (refusal instanceof BoxRefused) error(refusal.status, refusal.message);
		throw refusal;
	}
	return json({ connected: await connectedBoxOf(member.record, member.companyID) }, { headers: privateHeaders });
};
