import { error } from '@sveltejs/kit';
import { kv } from '$lib/kv';
import { environmentOfPlatform, fleetDirectory } from '$lib/server/agent-request';
import type { FleetDirectory } from '$lib/server/fleet-user-directory';
import type { Device, FleetUserRecord } from '$lib/types';

export function normalizeEmail(email: string): string {
	return email.trim().toLowerCase();
}

export function adminEmailsOf(records: FleetUserRecord[]): string[] {
	return records.filter((record) => record.role === 'admin').map((record) => record.email);
}

export function isAdminRequest(
	request: Request,
	device: Device,
	adminUsers: string[],
	adminToken: string,
	registerSecret: string
): boolean {
	if (adminToken && adminToken === registerSecret) return true;
	const authorizedAdmins = adminUsers.length > 0 ? adminUsers : [normalizeEmail(device.admin_email)];
	return authorizedAdmins.includes(callerEmail(request));
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
	const directory = await fleetDirectory(environmentOfPlatform(platform?.env), fleetID);
	if (!directory) throw error(404, 'this fleet belongs to no company yet');
	return { device, directory };
}

function callerEmail(request: Request): string {
	return normalizeEmail(request.headers.get('Cf-Access-Authenticated-User-Email') ?? '');
}

async function usersRevision(records: FleetUserRecord[]): Promise<string> {
	const encodedUsers = new TextEncoder().encode(JSON.stringify(records));
	const digest = await crypto.subtle.digest('SHA-256', encodedUsers);
	return Array.from(new Uint8Array(digest))
		.map((byte) => byte.toString(16).padStart(2, '0'))
		.join('');
}
