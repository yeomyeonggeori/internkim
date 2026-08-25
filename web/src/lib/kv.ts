import { normalizeFleet } from './fleet';
import type { Device, Invite, FleetUserRecord, UserRole } from './types';

export type KVStore = {
	get<T = unknown>(key: string, type: 'json'): Promise<T | null>;
	get(key: string): Promise<string | null>;
	put(key: string, value: string, options?: { expirationTtl?: number }): Promise<void>;
	delete(key: string): Promise<void>;
};

function normalizeEmail(email: string): string {
	return email.trim().toLowerCase();
}

function normalizeHandle(handle: string): string {
	return handle
		.trim()
		.toLowerCase()
		.replace(/[^a-z0-9._-]+/g, '-')
		.replace(/^[._-]+|[._-]+$/g, '')
		.slice(0, 22);
}

function normalizeHandleFromEmail(email: string): string {
	const localPart = email.split('@')[0] ?? '';
	const handle = normalizeHandle(localPart);
	if (handle.length >= 3 && /^[a-z]/.test(handle)) return handle;
	return `user-${calculateStableStringHash(email).slice(0, 6)}`;
}

function calculateStableStringHash(value: string): string {
	let hashValue = 2166136261;
	for (const character of value) {
		hashValue ^= character.charCodeAt(0);
		hashValue = Math.imul(hashValue, 16777619);
	}
	return (hashValue >>> 0).toString(16).padStart(8, '0');
}

function normalizeRole(role: unknown): UserRole {
	if (role === 'operationsAdmin') return 'operationsAdmin';
	return role === 'admin' ? 'admin' : 'member';
}

function normalizeISODate(value: unknown): string {
	if (typeof value !== 'string') return '';
	const date = value.trim();
	if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) return '';
	const parsed = new Date(`${date}T00:00:00.000Z`);
	if (Number.isNaN(parsed.getTime())) return '';
	return parsed.toISOString().slice(0, 10) === date ? date : '';
}

function normalizeNote(value: unknown): string {
	return typeof value === 'string' ? value.trim() : '';
}

function normalizeUserRecord(value: unknown): FleetUserRecord | null {
	if (typeof value === 'string') {
		const email = normalizeEmail(value);
		return email
			? {
					handle: normalizeHandleFromEmail(email),
					email,
					role: 'member',
					isIncomplete: true
				}
			: null;
	}
	if (!value || typeof value !== 'object') return null;
	const record = value as Partial<FleetUserRecord>;
	const email = normalizeEmail(record.email ?? '');
	if (!email) return null;
	const handle = normalizeHandle(record.handle ?? record.mattermostUsername ?? normalizeHandleFromEmail(email));
	const name = typeof record.name === 'string' ? record.name.trim() : '';
	const hireDate = normalizeISODate(record.hireDate);
	const note = normalizeNote(record.note);
	return {
		handle: handle || normalizeHandleFromEmail(email),
		...(name ? { name } : {}),
		email,
		...(hireDate ? { hireDate } : {}),
		...(note ? { note } : {}),
		role: normalizeRole(record.role),
		mattermostUserID: record.mattermostUserID,
		mattermostUsername: record.mattermostUsername,
		status: record.status,
		isIncomplete: !name
	};
}

function ensureAdmin(records: FleetUserRecord[], fallbackAdminEmail?: string): FleetUserRecord[] {
	const normalizedFallback = normalizeEmail(fallbackAdminEmail ?? '');
	const normalizedRecords = records.filter((record) => record.email);
	if (normalizedRecords.some((record) => record.role === 'admin')) return normalizedRecords;
	return normalizedFallback
		? [
				{
					handle: normalizeHandleFromEmail(normalizedFallback),
					email: normalizedFallback,
					role: 'admin',
					isIncomplete: true
				},
				...normalizedRecords
			]
		: normalizedRecords;
}

function normalizeUserRecords(records: FleetUserRecord[]): FleetUserRecord[] {
	return records
		.map((record) => normalizeUserRecord(record))
		.filter((record): record is FleetUserRecord => record !== null);
}

function normalizeDevice(value: unknown, fleetID: string): Device | null {
	if (!value || typeof value !== 'object') return null;
	const record = value as Partial<Device>;
	const resolvedFleetID = record.fleet_id ?? fleetID;
	if (!resolvedFleetID) return null;
	return {
		...record,
		fleet_id: resolvedFleetID,
		fleet_secret_hash: record.fleet_secret_hash,
		fleet: normalizeFleet(record.fleet, resolvedFleetID)
	} as Device;
}

function fleetKey(fleetID: string): string {
	return `fleet:${fleetID}`;
}

function fleetUsersKey(fleetID: string): string {
	return `fleet-users:${fleetID}`;
}

export function userEmails(records: FleetUserRecord[]): string[] {
	return records.map((record) => record.email);
}

export function adminEmails(records: FleetUserRecord[]): string[] {
	return records.filter((record) => record.role === 'admin').map((record) => record.email);
}

export const kv = {
	async getDevice(kv: KVStore, id: string): Promise<Device | null> {
		return normalizeDevice(await kv.get(fleetKey(id), 'json'), id);
	},

	async putDevice(kv: KVStore, id: string, device: Device): Promise<void> {
		await kv.put(fleetKey(id), JSON.stringify(normalizeDevice(device, id) ?? device));
	},

	async deleteDevice(kv: KVStore, id: string): Promise<void> {
		await kv.delete(fleetKey(id));
	},

	async getInvite(kv: KVStore, token: string): Promise<Invite | null> {
		return kv.get<Invite>(`invite:${token}`, 'json');
	},

	async putInvite(kv: KVStore, token: string, invite: Invite): Promise<void> {
		const ttl = Math.max(Math.floor((invite.expires_at - Date.now()) / 1000), 60);
		await kv.put(`invite:${token}`, JSON.stringify(invite), { expirationTtl: ttl });
	},

	async deleteInvite(kv: KVStore, token: string): Promise<void> {
		await kv.delete(`invite:${token}`);
	}
};
