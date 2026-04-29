import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { ensureAdminAccessApplications, ensureOneTimePinIdentityProvider, syncAccessPolicyEmails } from '$lib/cloudflare';
import { isBoardRequest, normalizeDeviceID } from '$lib/device-auth';
import { adminEmails, kv, userEmails } from '$lib/kv';
import type { Device, UserRecord } from '$lib/types';

const corsHeaders = {
	'Access-Control-Allow-Origin': '*',
	'Access-Control-Allow-Methods': 'GET, POST, DELETE, OPTIONS',
	'Access-Control-Allow-Headers': 'Content-Type, X-InternKim-Device-ID, X-InternKim-Device-Secret'
};

export const OPTIONS: RequestHandler = async () => {
	return new Response(null, { headers: corsHeaders });
};

function normalizeEmail(email: string): string {
	return email.trim().toLowerCase();
}

async function usersRevision(records: UserRecord[]): Promise<string> {
	const encodedUsers = new TextEncoder().encode(JSON.stringify(records));
	const digest = await crypto.subtle.digest('SHA-256', encodedUsers);
	return Array.from(new Uint8Array(digest))
		.map((byte) => byte.toString(16).padStart(2, '0'))
		.join('');
}

function cfEnv(env: App.Platform['env']) {
	return {
		CF_API_TOKEN: env.CF_API_TOKEN,
		CF_ACCOUNT_ID: env.CF_ACCOUNT_ID,
		CF_ZONE_ID: env.CF_ZONE_ID,
		CF_DOMAIN: env.CF_DOMAIN
	};
}

function callerEmail(request: Request): string {
	return normalizeEmail(request.headers.get('Cf-Access-Authenticated-User-Email') ?? '');
}

function isAdminRequest(request: Request, device: Device, adminUsers: string[], adminToken: string, registerSecret: string): boolean {
	if (adminToken && adminToken === registerSecret) return true;
	const authorizedAdmins = adminUsers.length > 0 ? adminUsers : [normalizeEmail(device.admin_email)];
	return authorizedAdmins.includes(callerEmail(request));
}

export const DELETE: RequestHandler = async ({ params, request, url, platform }) => {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	const device_id = normalizeDeviceID(url.searchParams.get('device_id') ?? '');
	const admin_token = url.searchParams.get('admin_token') ?? '';
	const email = normalizeEmail(decodeURIComponent(params.email));
	if (!device_id || !email) throw error(400, 'device_id and email required');

	const device = await kv.getDevice(env.KV, device_id);
	if (!device) throw error(404, 'Device not found');

	const records = await kv.getUserRecords(env.KV, device_id);
	const isAuthorizedBoard = await isBoardRequest(request, device, device_id);
	if (!isAuthorizedBoard && !isAdminRequest(request, device, adminEmails(records), admin_token, env.INTERNKIM_REGISTER_SECRET)) {
		throw error(403, 'Admin only');
	}
	const record = records.find((item) => item.email === email);
	if (!isAuthorizedBoard && record?.role === 'admin' && adminEmails(records).length <= 1) {
		throw error(400, 'Cannot remove the last admin user');
	}

	const filtered = records.filter((item) => item.email !== email);
	await kv.putUserRecords(env.KV, device_id, filtered);
	if (device.access_app_id) {
		await syncAccessPolicyEmails(cfEnv(env), device.access_app_id, userEmails(filtered));
	}
	const identityProviderID = await ensureOneTimePinIdentityProvider(cfEnv(env));
	await ensureAdminAccessApplications(cfEnv(env), device_id, identityProviderID, adminEmails(filtered));

	return json({ users: userEmails(filtered), records: filtered, revision: await usersRevision(filtered) }, { headers: corsHeaders });
};
