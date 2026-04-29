import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { ensureAdminAccessApplications, ensureOneTimePinIdentityProvider, syncAccessPolicyEmails } from '$lib/cloudflare';
import { isBoardRequest, normalizeDeviceID } from '$lib/device-auth';
import { adminEmails, kv, userEmails } from '$lib/kv';
import type { Device, UserRecord, UserRole } from '$lib/types';

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

function normalizeRole(role: unknown): UserRole {
	return role === 'admin' ? 'admin' : 'member';
}

function mergeRecord(records: UserRecord[], nextRecord: UserRecord): UserRecord[] {
	const existingRecord = records.find((record) => record.email === nextRecord.email);
	const filtered = records.filter((record) => record.email !== nextRecord.email);
	return [
		...filtered,
		{
			...existingRecord,
			...nextRecord,
			mattermostUserID: nextRecord.mattermostUserID ?? existingRecord?.mattermostUserID,
			mattermostUsername: nextRecord.mattermostUsername ?? existingRecord?.mattermostUsername,
			status: nextRecord.status ?? existingRecord?.status
		}
	].sort((first, second) => first.email.localeCompare(second.email));
}

async function syncAccessPolicies(env: App.Platform['env'], deviceID: string, device: Device, records: UserRecord[]) {
	if (device.access_app_id) {
		await syncAccessPolicyEmails(cfEnv(env), device.access_app_id, userEmails(records));
	}
	const identityProviderID = await ensureOneTimePinIdentityProvider(cfEnv(env));
	await ensureAdminAccessApplications(cfEnv(env), deviceID, identityProviderID, adminEmails(records));
}

async function usersResponse(records: UserRecord[]) {
	return { users: userEmails(records), records, revision: await usersRevision(records) };
}

export const GET: RequestHandler = async ({ request, url, platform }) => {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	const device_id = normalizeDeviceID(url.searchParams.get('device_id') ?? '');
	if (!device_id) throw error(400, 'device_id required');

	const device = await kv.getDevice(env.KV, device_id);
	if (!device) throw error(404, 'Device not found');

	const records = await kv.getUserRecords(env.KV, device_id);
	const admin_token = url.searchParams.get('admin_token') ?? '';
	const isAuthorizedBoard = await isBoardRequest(request, device, device_id);
	if (!isAuthorizedBoard && !isAdminRequest(request, device, adminEmails(records), admin_token, env.INTERNKIM_REGISTER_SECRET)) {
		throw error(403, 'Admin only');
	}

	return json(await usersResponse(records), { headers: corsHeaders });
};

export const POST: RequestHandler = async ({ request, platform }) => {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	const { device_id, email, role, admin_token, mattermostUserID, mattermostUsername, status } = (await request.json()) as {
		device_id: string;
		email: string;
		role?: UserRole;
		admin_token: string;
		mattermostUserID?: string;
		mattermostUsername?: string;
		status?: string;
	};
	const deviceID = normalizeDeviceID(device_id ?? '');
	if (!deviceID || !email) throw error(400, 'device_id and email required');

	const device = await kv.getDevice(env.KV, deviceID);
	if (!device) throw error(404, 'Device not found');

	const records = await kv.getUserRecords(env.KV, deviceID);
	const isAuthorizedBoard = await isBoardRequest(request, device, deviceID);
	if (!isAuthorizedBoard && !isAdminRequest(request, device, adminEmails(records), admin_token, env.INTERNKIM_REGISTER_SECRET)) {
		throw error(403, 'Admin only');
	}

	const normalizedEmail = normalizeEmail(email);
	const normalizedRole = normalizeRole(role);
	const existingRecord = records.find((record) => record.email === normalizedEmail);
	if (!isAuthorizedBoard && existingRecord?.role === 'admin' && normalizedRole !== 'admin' && adminEmails(records).length <= 1) {
		throw error(400, 'Cannot demote the last admin user');
	}
	const nextRecords = mergeRecord(records, {
		email: normalizedEmail,
		role: normalizedRole,
		mattermostUserID,
		mattermostUsername,
		status
	});
	await kv.putUserRecords(env.KV, deviceID, nextRecords);
	await syncAccessPolicies(env, deviceID, device, nextRecords);

	return json(await usersResponse(nextRecords), { headers: corsHeaders });
};
