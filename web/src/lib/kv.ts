import type { Device, Invite, UserRecord, UserRole } from './types';

function normalizeEmail(email: string): string {
	return email.trim().toLowerCase();
}

function normalizeRole(role: unknown): UserRole {
	return role === 'admin' ? 'admin' : 'member';
}

function normalizeUserRecord(value: unknown): UserRecord | null {
	if (typeof value === 'string') {
		const email = normalizeEmail(value);
		return email ? { email, role: 'member' } : null;
	}
	if (!value || typeof value !== 'object') return null;
	const record = value as Partial<UserRecord>;
	const email = normalizeEmail(record.email ?? '');
	if (!email) return null;
	return {
		email,
		role: normalizeRole(record.role),
		mattermostUserID: record.mattermostUserID,
		mattermostUsername: record.mattermostUsername,
		status: record.status
	};
}

function ensureAdmin(records: UserRecord[], fallbackAdminEmail?: string): UserRecord[] {
	const normalizedFallback = normalizeEmail(fallbackAdminEmail ?? '');
	const normalizedRecords = records.filter((record) => record.email);
	if (normalizedRecords.some((record) => record.role === 'admin')) return normalizedRecords;
	if (normalizedRecords.length > 0) {
		return normalizedRecords.map((record, index) => index === 0 ? { ...record, role: 'admin' } : record);
	}
	return normalizedFallback ? [{ email: normalizedFallback, role: 'admin' }] : [];
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
		const records = ensureAdmin(emails.map((email) => ({ email: normalizeEmail(email), role: 'member' })));
		await this.putUserRecords(kv, deviceId, records);
	},

	async putUserRecords(kv: KVNamespace, deviceId: string, records: UserRecord[]): Promise<void> {
		await kv.put(`users:${deviceId}`, JSON.stringify(ensureAdmin(records)));
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
