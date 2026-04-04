import type { Device, Invite } from './types';

export const kv = {
	async getDevice(kv: KVNamespace, id: string): Promise<Device | null> {
		return kv.get(`device:${id}`, 'json');
	},

	async putDevice(kv: KVNamespace, id: string, device: Device): Promise<void> {
		await kv.put(`device:${id}`, JSON.stringify(device));
	},

	async getUsers(kv: KVNamespace, deviceId: string): Promise<string[]> {
		const users = await kv.get(`users:${deviceId}`, 'json');
		return (users as string[]) ?? [];
	},

	async putUsers(kv: KVNamespace, deviceId: string, emails: string[]): Promise<void> {
		await kv.put(`users:${deviceId}`, JSON.stringify(emails));
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
