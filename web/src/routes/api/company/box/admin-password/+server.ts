import { error, json } from '@sveltejs/kit';
import { boxKeySchema } from '$lib/company/box';
import { environmentOfPlatform } from '$lib/server/agent-request';
import { BoxRefused } from '$lib/server/box';
import { adminPasswordRequestSchema, adminPasswordStatusFor, requestAdminPassword } from '$lib/server/box-admin-password';
import { callingHostAdministrator } from '$lib/server/company-host-setup';
import type { RequestHandler } from './$types';

export const PUT: RequestHandler = async ({ request, platform }) => {
	const member = await callingHostAdministrator(request, environmentOfPlatform(platform?.env));
	const requested = adminPasswordRequestSchema.safeParse(await request.json().catch(() => null));
	if (!requested.success) error(400, 'an admin password arrives sealed to the box encryption key, named by its setting');

	try {
		const status = await requestAdminPassword(member.record, member.companyID, requested.data.publicKey, requested.data.settingID, requested.data.sealed);
		return json({ status }, { headers: { 'Cache-Control': 'no-store' } });
	} catch (refusal) {
		if (refusal instanceof BoxRefused) error(409, refusal.message);
		throw refusal;
	}
};

export const GET: RequestHandler = async ({ request, platform, url }) => {
	const member = await callingHostAdministrator(request, environmentOfPlatform(platform?.env));
	const publicKey = boxKeySchema.safeParse(url.searchParams.get('publicKey'));
	if (!publicKey.success) error(400, 'name the box by its public key');
	try {
		const status = await adminPasswordStatusFor(member.record, member.companyID, publicKey.data);
		return json({ status }, { headers: { 'Cache-Control': 'no-store' } });
	} catch (refusal) {
		if (refusal instanceof BoxRefused) error(409, refusal.message);
		throw refusal;
	}
};
