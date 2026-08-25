import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { isNodeRequest, normalizeFleetID } from '$lib/device-auth';
import { kv } from '$lib/kv';
import { environmentOfPlatform, fleetDirectory } from '$lib/server/agent-request';
import {
	fleetUserRecords,
	saveFleetUserRecord,
	type FleetDirectory
} from '$lib/server/fleet-user-directory';
import type { Device, FleetUserRecord, UserRole } from '$lib/types';

const corsHeaders = {
	'Access-Control-Allow-Origin': '*',
	'Access-Control-Allow-Methods': 'GET, POST, DELETE, OPTIONS',
	'Access-Control-Allow-Headers': 'Content-Type, X-INTERNKIM-FLEET-ID, X-INTERNKIM-FLEET-SECRET, X-INTERNKIM-DEVICE-ID, X-INTERNKIM-DEVICE-SECRET'
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

async function usersRevision(records: FleetUserRecord[]): Promise<string> {
	const encodedUsers = new TextEncoder().encode(JSON.stringify(records));
	const digest = await crypto.subtle.digest('SHA-256', encodedUsers);
	return Array.from(new Uint8Array(digest))
		.map((byte) => byte.toString(16).padStart(2, '0'))
		.join('');
}

function callerEmail(request: Request): string {
	return normalizeEmail(request.headers.get('Cf-Access-Authenticated-User-Email') ?? '');
}

export function adminEmailsOf(records: FleetUserRecord[]): string[] {
	return records.filter((record) => record.role === 'admin').map((record) => record.email);
}

export function isAdminRequest(request: Request, device: Device, adminUsers: string[], adminToken: string, registerSecret: string): boolean {
	if (adminToken && adminToken === registerSecret) return true;
	const authorizedAdmins = adminUsers.length > 0 ? adminUsers : [normalizeEmail(device.admin_email)];
	return authorizedAdmins.includes(callerEmail(request));
}

function normalizeRole(role: unknown): UserRole {
	if (role === 'operationsAdmin') return 'operationsAdmin';
	return role === 'admin' ? 'admin' : 'member';
}

function refuseInvalidHireDate(value: unknown): void {
	if (typeof value !== 'string') return;
	const date = value.trim();
	if (!date) return;
	if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) throw error(400, 'hireDate must be YYYY-MM-DD');
	const parsed = new Date(`${date}T00:00:00.000Z`);
	if (Number.isNaN(parsed.getTime()) || parsed.toISOString().slice(0, 10) !== date) throw error(400, 'hireDate must be a valid date');
}

export async function usersResponse(records: FleetUserRecord[]) {
	return {
		users: records.map((record) => record.email),
		records,
		revision: await usersRevision(records)
	};
}

export async function askedDirectory(
	platform: App.Platform | undefined,
	fleetID: string
): Promise<{ device: Device; directory: FleetDirectory }> {
	const store = platform?.env?.KV;
	if (!store) throw error(500, 'the fleet register is not available');
	const device = await kv.getDevice(store, fleetID);
	if (!device) throw error(404, 'Fleet not found');
	const directory = await fleetDirectory(
		environmentOfPlatform(platform?.env),
		fleetID
	);
	if (!directory) throw error(404, 'this fleet belongs to no company yet');
	return { device, directory };
}

export const GET: RequestHandler = async ({ request, url, platform }) => {
	const fleetID = normalizeFleetID(url.searchParams.get('fleet_id') ?? '');
	if (!fleetID) throw error(400, 'fleet_id required');

	const { device, directory } = await askedDirectory(platform, fleetID);
	const records = await fleetUserRecords(directory);
	const adminToken = url.searchParams.get('admin_token') ?? '';
	const isAuthorizedNode = await isNodeRequest(request, device, fleetID);
	if (!isAuthorizedNode && !isAdminRequest(request, device, adminEmailsOf(records), adminToken, platform?.env?.INTERNKIM_REGISTER_SECRET ?? '')) {
		throw error(403, 'Admin only');
	}

	return json(await usersResponse(records), { headers: corsHeaders });
};

export const POST: RequestHandler = async ({ request, platform }) => {
	const { fleet_id, handle, name, email, hireDate, note, role, admin_token, mattermostUserID, mattermostUsername, status } = (await request.json()) as {
		fleet_id?: string;
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
	refuseInvalidHireDate(hireDate);

	const { device, directory } = await askedDirectory(platform, fleetID);
	const records = await fleetUserRecords(directory);
	const isAuthorizedNode = await isNodeRequest(request, device, fleetID);
	if (!isAuthorizedNode && !isAdminRequest(request, device, adminEmailsOf(records), admin_token, platform?.env?.INTERNKIM_REGISTER_SECRET ?? '')) {
		throw error(403, 'Admin only');
	}

	const normalizedEmail = normalizeEmail(email);
	const normalizedRole = normalizeRole(role);
	const existingRecord = records.find((record) => record.email === normalizedEmail);
	const normalizedHandle = normalizeHandle(handle ?? existingRecord?.handle ?? '');
	const normalizedName = typeof name === 'string' ? name.trim() : existingRecord?.name;
	if (!existingRecord && (!normalizedHandle || !normalizedName)) throw error(400, 'handle, name, and email required');
	if (normalizedHandle && !isValidHandle(normalizedHandle)) throw error(400, 'handle must start with a letter and contain 3-22 lowercase letters, numbers, dots, dashes, or underscores');
	if (!isAuthorizedNode && existingRecord?.role === 'admin' && normalizedRole !== 'admin' && adminEmailsOf(records).length <= 1) {
		throw error(400, 'Cannot demote the last admin user');
	}

	const nextRecords = await saveFleetUserRecord(directory, {
		handle: normalizedHandle,
		...(normalizedName ? { name: normalizedName } : {}),
		email: normalizedEmail,
		note: note === undefined ? existingRecord?.note : (note ?? '').trim(),
		role: normalizedRole,
		mattermostUserID,
		mattermostUsername,
		status
	});

	return json(await usersResponse(nextRecords), { headers: corsHeaders });
};
