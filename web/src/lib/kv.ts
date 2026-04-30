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
	return {
		userID: typeof record.userID === 'string' && record.userID.trim() ? record.userID.trim() : stableUserID(email),
		handle: handle || normalizeHandleFromEmail(email),
		...(name ? { name } : {}),
		email,
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

export function userEmails(records: UserRecord[]): string[] {
	return records.map((record) => record.email);
}

export function adminEmails(records: UserRecord[]): string[] {
	return records.filter((record) => record.role === 'admin').map((record) => record.email);
}

export const kv = {
	async getDevice(kv: KVNamespace, id: string): Promise<Device | null> {
		return kv.get(`device:${id}`, 'json');
	},

	async putDevice(kv: KVNamespace, id: string, device: Device): Promise<void> {
		await kv.put(`device:${id}`, JSON.stringify(device));
	},

	async getUsers(kv: KVNamespace, deviceId: string): Promise<string[]> {
		return userEmails(await this.getUserRecords(kv, deviceId));
	},

	async getUserRecords(kv: KVNamespace, deviceId: string, fallbackAdminEmail?: string): Promise<UserRecord[]> {
		const users = await kv.get(`users:${deviceId}`, 'json');
		const values = Array.isArray(users) ? users : [];
		return ensureAdmin(values.map(normalizeUserRecord).filter((record): record is UserRecord => record !== null), fallbackAdminEmail);
	},

	async putUsers(kv: KVNamespace, deviceId: string, emails: string[]): Promise<void> {
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
		await this.putUserRecords(kv, deviceId, records);
	},

	async putUserRecords(kv: KVNamespace, deviceId: string, records: UserRecord[]): Promise<void> {
		await kv.put(`users:${deviceId}`, JSON.stringify(normalizeUserRecords(records)));
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
