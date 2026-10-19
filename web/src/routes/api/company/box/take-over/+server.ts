import { error, json } from '@sveltejs/kit';
import { takeOverSchema } from '$lib/company/box';
import { environmentOfPlatform } from '$lib/server/agent-request';
import { BoxRefused, companyBoxesOf, keepTakeOver } from '$lib/server/box';
import { callingHostAdministrator } from '$lib/server/company-host-setup';
import type { RequestHandler } from './$types';

export const PUT: RequestHandler = async ({ request, platform }) => {
	const member = await callingHostAdministrator(request, environmentOfPlatform(platform?.env));
	const takeOver = takeOverSchema.safeParse(await request.json().catch(() => null));
	if (!takeOver.success) error(400, 'say whether a standby box takes over by itself, and after how many seconds');
	try {
		await keepTakeOver(member.record, member.companyID, takeOver.data);
	} catch (refusal) {
		if (refusal instanceof BoxRefused) error(refusal.status, refusal.message);
		throw refusal;
	}
	return json(await companyBoxesOf(member.record, member.companyID), { headers: { 'Cache-Control': 'no-store' } });
};
