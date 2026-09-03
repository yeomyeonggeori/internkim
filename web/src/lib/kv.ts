import { normalizeFleet } from './fleet';
import type { Device, Invite } from './types';

export type KVStore = {
	get<T = unknown>(key: string, type: 'json'): Promise<T | null>;
	get(key: string): Promise<string | null>;
	put(key: string, value: string, options?: { expirationTtl?: number }): Promise<void>;
	delete(key: string): Promise<void>;
};

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
