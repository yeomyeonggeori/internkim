import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { syncAccessPolicyEmails } from '$lib/cloudflare';
import { normalizeDeviceID } from '$lib/device-auth';
import { kv } from '$lib/kv';
import type { Device } from '$lib/types';

const corsHeaders = {
	'Access-Control-Allow-Origin': '*',
	'Access-Control-Allow-Methods': 'GET, POST, DELETE, OPTIONS',
	'Access-Control-Allow-Headers': 'Content-Type'
};

export const OPTIONS: RequestHandler = async () => {
	return new Response(null, { headers: corsHeaders });
};

function normalizeEmail(email: string): string {
	return email.trim().toLowerCase();
}

async function usersRevision(users: string[]): Promise<string> {
	const encodedUsers = new TextEncoder().encode(JSON.stringify(users));
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

function isAdminRequest(request: Request, device: Device, users: string[], adminToken: string, registerSecret: string): boolean {
	if (adminToken && adminToken === registerSecret) return true;
	const adminEmail = users[0] ?? normalizeEmail(device.admin_email);
	return callerEmail(request) === adminEmail;
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

	const users = await kv.getUsers(env.KV, device_id);
	if (!isAdminRequest(request, device, users, admin_token, env.INTERNKIM_REGISTER_SECRET)) {
		throw error(403, 'Admin only');
	}
	if (email === (users[0] ?? normalizeEmail(device.admin_email))) {
		throw error(400, 'Cannot remove the admin user');
	}

	const filtered = users.filter((u) => u !== email);
	await kv.putUsers(env.KV, device_id, filtered);
	if (device.access_app_id) {
		await syncAccessPolicyEmails(cfEnv(env), device.access_app_id, filtered);
	}

	return json({ users: filtered, revision: await usersRevision(filtered) }, { headers: corsHeaders });
};
