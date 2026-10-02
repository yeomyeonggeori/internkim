import { error, json } from '@sveltejs/kit';
import { environmentOfPlatform } from '$lib/server/agent-request';
import { BoxRefused } from '$lib/server/box';
import { requestWifiChange, wifiChangeRequestSchema, wifiChangeStatusFor } from '$lib/server/box-wifi';
import { callingHostAdministrator } from '$lib/server/company-host-setup';
import type { RequestHandler } from './$types';

export const PUT: RequestHandler = async ({ request, platform }) => {
	const member = await callingHostAdministrator(request, environmentOfPlatform(platform?.env));
	const requested = wifiChangeRequestSchema.safeParse(await request.json().catch(() => null));
	if (!requested.success) error(400, 'a Wi-Fi network arrives sealed to the box encryption key, named by the request it answers');

	try {
		const status = await requestWifiChange(member.record, member.companyID, requested.data.requestID, requested.data.sealed);
		return json({ status }, { headers: { 'Cache-Control': 'no-store' } });
	} catch (refusal) {
		if (refusal instanceof BoxRefused) error(409, refusal.message);
		throw refusal;
	}
};

export const GET: RequestHandler = async ({ request, platform }) => {
	const member = await callingHostAdministrator(request, environmentOfPlatform(platform?.env));
	try {
		const status = await wifiChangeStatusFor(member.record, member.companyID);
		return json({ status }, { headers: { 'Cache-Control': 'no-store' } });
	} catch (refusal) {
		if (refusal instanceof BoxRefused) error(409, refusal.message);
		throw refusal;
	}
};
