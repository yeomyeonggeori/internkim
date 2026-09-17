import { memberAccessToken } from '$lib/public-api-call';
import type { PushDeviceKind, PushReachability } from './reachability';

export const pushDevicePath = '/api/member/push-device';

export type PushDeviceAddress = { endpoint: string; kind?: PushDeviceKind };

export type PushDeviceClaim = PushDeviceAddress & { publicKey?: string; authenticationSecret?: string };

function isPushReachability(answered: unknown): answered is PushReachability {
	return (
		typeof answered === 'object' &&
		answered !== null &&
		'serverKey' in answered &&
		typeof answered.serverKey === 'string' &&
		'isServerKeyVaulted' in answered &&
		typeof answered.isServerKeyVaulted === 'boolean' &&
		'hasClaimedDevice' in answered &&
		typeof answered.hasClaimedDevice === 'boolean'
	);
}

function refusalOf(answered: unknown): string {
	if (typeof answered !== 'object' || answered === null || !('message' in answered)) return '';
	return typeof answered.message === 'string' ? answered.message : '';
}

async function answeredReachability(address: string, init: RequestInit): Promise<PushReachability> {
	const response = await fetch(address, {
		...init,
		headers: { Authorization: `Bearer ${await memberAccessToken()}`, 'Content-Type': 'application/json' }
	});
	const answered: unknown = await response.json().catch(() => null);
	if (!response.ok) throw new Error(refusalOf(answered) || `the push device answered ${response.status}`);
	if (!isPushReachability(answered)) throw new Error('the push device answered without saying who it reaches');
	return answered;
}

export function askPushReachability(): Promise<PushReachability> {
	return answeredReachability(pushDevicePath, { method: 'GET' });
}

export function claimPushDevice(claim: PushDeviceClaim): Promise<PushReachability> {
	return answeredReachability(pushDevicePath, { method: 'PUT', body: JSON.stringify(claim) });
}

export function releasePushDevice(address: PushDeviceAddress): Promise<PushReachability> {
	const query = new URLSearchParams({ endpoint: address.endpoint });
	if (address.kind) query.set('kind', address.kind);
	return answeredReachability(`${pushDevicePath}?${query}`, { method: 'DELETE' });
}
