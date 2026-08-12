import type { Device } from './types';

export function normalizeFleetID(fleetID: string): string {
	return fleetID.trim().toLowerCase();
}

export async function hashFleetSecret(fleetSecret: string): Promise<string> {
	const encodedSecret = new TextEncoder().encode(fleetSecret);
	const digest = await crypto.subtle.digest('SHA-256', encodedSecret);
	return Array.from(new Uint8Array(digest))
		.map((byte) => byte.toString(16).padStart(2, '0'))
		.join('');
}

export async function isNodeRequest(request: Request, device: Device, fleetID: string): Promise<boolean> {
	const headerFleetID = normalizeFleetID(request.headers.get('X-INTERNKIM-FLEET-ID') ?? '');
	const fleetSecret = request.headers.get('X-INTERNKIM-FLEET-SECRET') ?? '';
	const fleetSecretHash = device.fleet_secret_hash ?? '';
	if (!headerFleetID || !fleetSecret || headerFleetID !== normalizeFleetID(fleetID)) return false;
	if (!fleetSecretHash) return false;
	return (await hashFleetSecret(fleetSecret)) === fleetSecretHash;
}
