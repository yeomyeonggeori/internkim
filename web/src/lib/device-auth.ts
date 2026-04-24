import type { Device } from './types';

export function normalizeDeviceID(deviceID: string): string {
	return deviceID.trim().toLowerCase();
}

export async function hashDeviceSecret(deviceSecret: string): Promise<string> {
	const encodedSecret = new TextEncoder().encode(deviceSecret);
	const digest = await crypto.subtle.digest('SHA-256', encodedSecret);
	return Array.from(new Uint8Array(digest))
		.map((byte) => byte.toString(16).padStart(2, '0'))
		.join('');
}

export async function isBoardRequest(request: Request, device: Device, deviceID: string): Promise<boolean> {
	const headerDeviceID = normalizeDeviceID(request.headers.get('X-InternKim-Device-ID') ?? '');
	const deviceSecret = request.headers.get('X-InternKim-Device-Secret') ?? '';
	if (!headerDeviceID || !deviceSecret || headerDeviceID !== normalizeDeviceID(deviceID)) return false;
	if (!device.device_secret_hash) return false;
	return (await hashDeviceSecret(deviceSecret)) === device.device_secret_hash;
}
