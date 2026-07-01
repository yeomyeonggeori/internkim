import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { syncFleetAccessPolicies } from '$lib/fleet-access';
import { isNodeRequest, normalizeFleetID } from '$lib/device-auth';
import { adminEmails, kv, userEmails } from '$lib/kv';
import type { Device, UserRecord, UserRole } from '$lib/types';

const corsHeaders = {
	'Access-Control-Allow-Origin': '*',
	'Access-Control-Allow-Methods': 'GET, POST, DELETE, OPTIONS',
	'Access-Control-Allow-Headers': 'Content-Type, X-InternKim-Fleet-ID, X-InternKim-Fleet-Secret, X-InternKim-Device-ID, X-InternKim-Device-Secret'
};

export const OPTIONS: RequestHandler = async () => {
	return new Response(null, { headers: corsHeaders });
};

function normalizeEmail(email: string): string {
	return email.trim().toLowerCase();
}

function normalizeHandle(handle: string): string {
	return handle.trim().toLowerCase();
}

function isValidHandle(handle: string): boolean {
	return /^[a-z][a-z0-9._-]{2,21}$/.test(handle);
}

function newUserID(): string {
	return crypto.randomUUID();
}

function normalizedUserID(userID: string | undefined): string {
	return userID?.trim() || newUserID();
}

async function usersRevision(records: UserRecord[]): Promise<string> {
	const encodedUsers = new TextEncoder().encode(JSON.stringify(records));
	const digest = await crypto.subtle.digest('SHA-256', encodedUsers);
	return Array.from(new Uint8Array(digest))
		.map((byte) => byte.toString(16).padStart(2, '0'))
		.join('');
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
	if (role === 'operationsAdmin') return 'operationsAdmin';
	return role === 'admin' ? 'admin' : 'member';
}

function normalizeISODate(value: unknown): string {
	if (typeof value !== 'string') return '';
	const date = value.trim();
	if (!date) return '';
	if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) throw error(400, 'hireDate must be YYYY-MM-DD');
	const parsed = new Date(`${date}T00:00:00.000Z`);
	if (Number.isNaN(parsed.getTime()) || parsed.toISOString().slice(0, 10) !== date) throw error(400, 'hireDate must be a valid date');
	return date;
}

function normalizeNote(value: unknown): string {
	return typeof value === 'string' ? value.trim() : '';
}

function mergeRecord(records: UserRecord[], nextRecord: UserRecord): UserRecord[] {
	const existingRecord = records.find((record) => record.email === nextRecord.email);
	const filtered = records.filter((record) => record.email !== nextRecord.email);
	return [
		...filtered,
		{
			...existingRecord,
			...nextRecord,
			userID: existingRecord?.userID ?? nextRecord.userID,
			handle: nextRecord.handle || existingRecord?.handle || '',
			name: nextRecord.name ?? existingRecord?.name,
			hireDate: nextRecord.hireDate ?? existingRecord?.hireDate,
			note: nextRecord.note ?? existingRecord?.note,
			mattermostUserID: nextRecord.mattermostUserID ?? existingRecord?.mattermostUserID,
			mattermostUsername: nextRecord.mattermostUsername ?? existingRecord?.mattermostUsername,
			status: nextRecord.status ?? existingRecord?.status,
			isIncomplete: !(nextRecord.name ?? existingRecord?.name)
		}
	].sort((first, second) => first.email.localeCompare(second.email));
}

function duplicateHandle(records: UserRecord[]): string {
	const seenHandles = new Set<string>();
	for (const record of records) {
		const handle = normalizeHandle(record.handle);
		if (!handle) continue;
		if (seenHandles.has(handle)) return handle;
		seenHandles.add(handle);
	}
	return '';
}

async function usersResponse(records: UserRecord[]) {
	return { users: userEmails(records), records, revision: await usersRevision(records) };
}

export const GET: RequestHandler = async ({ request, url, platform }) => {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	const fleetID = normalizeFleetID(url.searchParams.get('fleet_id') ?? '');
	if (!fleetID) throw error(400, 'fleet_id required');

	const device = await kv.getDevice(env.KV, fleetID);
	if (!device) throw error(404, 'Fleet not found');

	const records = await kv.getUserRecords(env.KV, fleetID);
	const admin_token = url.searchParams.get('admin_token') ?? '';
	const isAuthorizedNode = await isNodeRequest(request, device, fleetID);
	if (!isAuthorizedNode && !isAdminRequest(request, device, adminEmails(records), admin_token, env.INTERNKIM_REGISTER_SECRET)) {
		throw error(403, 'Admin only');
	}

	return json(await usersResponse(records), { headers: corsHeaders });
};

export const POST: RequestHandler = async ({ request, platform }) => {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	const { fleet_id, userID, handle, name, email, hireDate, note, role, admin_token, mattermostUserID, mattermostUsername, status } = (await request.json()) as {
		fleet_id?: string;
		userID?: string;
		handle?: string;
		name?: string;
		email: string;
		hireDate?: string;
		note?: string;
		role?: UserRole;
		admin_token: string;
		mattermostUserID?: string;
		mattermostUsername?: string;
		status?: string;
	};
	const fleetID = normalizeFleetID(fleet_id ?? '');
	if (!fleetID || !email) throw error(400, 'fleet_id and email required');

	const device = await kv.getDevice(env.KV, fleetID);
	if (!device) throw error(404, 'Fleet not found');

	const records = await kv.getUserRecords(env.KV, fleetID);
	const isAuthorizedNode = await isNodeRequest(request, device, fleetID);
	if (!isAuthorizedNode && !isAdminRequest(request, device, adminEmails(records), admin_token, env.INTERNKIM_REGISTER_SECRET)) {
		throw error(403, 'Admin only');
	}

	const normalizedEmail = normalizeEmail(email);
	const normalizedRole = normalizeRole(role);
	const existingRecord = records.find((record) => record.email === normalizedEmail);
	const normalizedHandle = normalizeHandle(handle ?? existingRecord?.handle ?? '');
	const normalizedName = typeof name === 'string' ? name.trim() : existingRecord?.name;
	const normalizedHireDate = hireDate === undefined ? (existingRecord?.hireDate ?? '') : normalizeISODate(hireDate);
	const normalizedNote = note === undefined ? existingRecord?.note : normalizeNote(note);
	if (!existingRecord && (!normalizedHandle || !normalizedName)) throw error(400, 'handle, name, and email required');
	if (normalizedHandle && !isValidHandle(normalizedHandle)) throw error(400, 'handle must start with a letter and contain 3-22 lowercase letters, numbers, dots, dashes, or underscores');
	if (!isAuthorizedNode && existingRecord?.role === 'admin' && normalizedRole !== 'admin' && adminEmails(records).length <= 1) {
		throw error(400, 'Cannot demote the last admin user');
	}
	const nextRecords = mergeRecord(records, {
		userID: existingRecord?.userID ?? normalizedUserID(userID),
		handle: normalizedHandle,
		...(normalizedName ? { name: normalizedName } : {}),
		email: normalizedEmail,
		hireDate: normalizedHireDate,
		note: normalizedNote ?? '',
		role: normalizedRole,
		mattermostUserID,
		mattermostUsername,
		status
	});
	const duplicatedHandle = duplicateHandle(nextRecords);
	if (duplicatedHandle) throw error(400, `Duplicate handle: ${duplicatedHandle}`);
	await kv.putUserRecords(env.KV, fleetID, nextRecords);
	const syncedDevice = await syncFleetAccessPolicies(env, fleetID, device, nextRecords);
	if (syncedDevice.access_app_id !== device.access_app_id || syncedDevice.access_policy_id !== device.access_policy_id) {
		await kv.putDevice(env.KV, fleetID, syncedDevice);
	}

	return json(await usersResponse(nextRecords), { headers: corsHeaders });
};
