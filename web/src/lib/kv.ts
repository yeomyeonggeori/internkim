import { normalizeFleet } from './fleet';
import type { Device, Invite, UserRecord, UserRole } from './types';

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

function stableUserID(email: string): string {
	return `user_${calculateStableStringHash(email)}`;
}

function normalizeRole(role: unknown): UserRole {
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

function normalizeUserRecord(value: unknown): UserRecord | null {
	if (typeof value === 'string') {
		const email = normalizeEmail(value);
		return email
			? {
					userID: stableUserID(email),
					handle: normalizeHandleFromEmail(email),
					email,
					role: 'member',
					isIncomplete: true
				}
			: null;
	}
	if (!value || typeof value !== 'object') return null;
	const record = value as Partial<UserRecord>;
	const email = normalizeEmail(record.email ?? '');
	if (!email) return null;
	const handle = normalizeHandle(record.handle ?? record.mattermostUsername ?? normalizeHandleFromEmail(email));
	const name = typeof record.name === 'string' ? record.name.trim() : '';
	const hireDate = normalizeISODate(record.hireDate);
	const note = normalizeNote(record.note);
	return {
		userID: typeof record.userID === 'string' && record.userID.trim() ? record.userID.trim() : stableUserID(email),
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

function ensureAdmin(records: UserRecord[], fallbackAdminEmail?: string): UserRecord[] {
	const normalizedFallback = normalizeEmail(fallbackAdminEmail ?? '');
	const normalizedRecords = records.filter((record) => record.email);
	if (normalizedRecords.some((record) => record.role === 'admin')) return normalizedRecords;
	return normalizedFallback
		? [
				{
					userID: stableUserID(normalizedFallback),
					handle: normalizeHandleFromEmail(normalizedFallback),
					email: normalizedFallback,
					role: 'admin',
					isIncomplete: true
				},
				...normalizedRecords
			]
		: normalizedRecords;
}

function normalizeUserRecords(records: UserRecord[]): UserRecord[] {
	return records
		.map((record) => normalizeUserRecord(record))
		.filter((record): record is UserRecord => record !== null);
}

function normalizeDevice(value: unknown, fleetID: string): Device | null {
	if (!value || typeof value !== 'object') return null;
	const record = value as Partial<Device>;
	const resolvedFleetID = record.fleet_id ?? fleetID;
	if (!resolvedFleetID || !record.tunnel_id || !record.tunnel_token || !record.dns_record_id) return null;
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

export function userEmails(records: UserRecord[]): string[] {
	return records.map((record) => record.email);
}

export function adminEmails(records: UserRecord[]): string[] {
	return records.filter((record) => record.role === 'admin').map((record) => record.email);
}

export const kv = {
	async getDevice(kv: KVNamespace, id: string): Promise<Device | null> {
		return normalizeDevice(await kv.get(fleetKey(id), 'json'), id);
	},

	async putDevice(kv: KVNamespace, id: string, device: Device): Promise<void> {
		await kv.put(fleetKey(id), JSON.stringify(normalizeDevice(device, id) ?? device));
	},

	async deleteDevice(kv: KVNamespace, id: string): Promise<void> {
		await kv.delete(fleetKey(id));
	},

	async getUsers(kv: KVNamespace, fleetID: string): Promise<string[]> {
		return userEmails(await this.getUserRecords(kv, fleetID));
	},

	async getUserRecords(kv: KVNamespace, fleetID: string, fallbackAdminEmail?: string): Promise<UserRecord[]> {
		const users = await kv.get(fleetUsersKey(fleetID), 'json');
		const values = Array.isArray(users) ? users : [];
		return ensureAdmin(values.map(normalizeUserRecord).filter((record): record is UserRecord => record !== null), fallbackAdminEmail);
	},

	async putUsers(kv: KVNamespace, fleetID: string, emails: string[]): Promise<void> {
		const records = normalizeUserRecords(
			emails.map((email) => {
				const normalizedEmail = normalizeEmail(email);
				return {
					userID: stableUserID(normalizedEmail),
					handle: normalizeHandleFromEmail(normalizedEmail),
					email: normalizedEmail,
					role: 'member',
					isIncomplete: true
				};
			})
		);
		await this.putUserRecords(kv, fleetID, records);
	},

	async putUserRecords(kv: KVNamespace, fleetID: string, records: UserRecord[]): Promise<void> {
		await kv.put(fleetUsersKey(fleetID), JSON.stringify(normalizeUserRecords(records)));
	},

	async deleteUserRecords(kv: KVNamespace, fleetID: string): Promise<void> {
		await kv.delete(fleetUsersKey(fleetID));
	},

	async getInvite(kv: KVNamespace, token: string): Promise<Invite | null> {
		return kv.get(`invite:${token}`, 'json');
	},

	async putInvite(kv: KVNamespace, token: string, invite: Invite): Promise<void> {
		const ttl = Math.max(Math.floor((invite.expires_at - Date.now()) / 1000), 60);
		await kv.put(`invite:${token}`, JSON.stringify(invite), { expirationTtl: ttl });
	},

	async deleteInvite(kv: KVNamespace, token: string): Promise<void> {
		await kv.delete(`invite:${token}`);
	}
};
