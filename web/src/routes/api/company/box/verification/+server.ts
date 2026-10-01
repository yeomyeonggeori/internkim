import { error, json } from '@sveltejs/kit';
import { boxVerificationSchema } from '$lib/company/box';
import { environmentOfPlatform } from '$lib/server/agent-request';
import { BoxRefused, verifyBoxCode } from '$lib/server/box';
import { callingHostAdministrator } from '$lib/server/company-host-setup';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform }) => {
	const member = await callingHostAdministrator(request, environmentOfPlatform(platform?.env));
	const asked = boxVerificationSchema.safeParse(await request.json().catch(() => null));
	if (!asked.success) error(400, 'name the box by its public key and the code it shows');

	const company = await member.record.from('company').select('name').eq('id', member.companyID).single<{ name: string }>();
	if (company.error) throw new Error(`company ${member.companyID}: ${company.error.message}`);
	try {
		const verified = await verifyBoxCode(member.record, member.companyID, asked.data.publicKey, asked.data.pairingCode);
		return json({ ...verified, companyName: company.data.name }, { headers: { 'Cache-Control': 'no-store' } });
	} catch (refusal) {
		if (refusal instanceof BoxRefused) error(refusal.status, refusal.message);
		throw refusal;
	}
};
