import { error } from '@sveltejs/kit';
import type { SupabaseClient } from '@supabase/supabase-js';
import {
	pushDeviceKinds,
	type PushDeviceKind,
	type PushReachability
} from '$lib/notifications/reachability';
import { statusOfPostgresCode } from './public-api/record/tasks';

export type PushDeviceAddress = { kind: PushDeviceKind; endpoint: string };

export type EncryptionKeys = { p256dh: string; auth: string };

export type PushDeviceClaim = PushDeviceAddress & { encryptionKeys: EncryptionKeys | null };

type Carried = Record<string, unknown>;

function isCarried(body: unknown): body is Carried {
	return typeof body === 'object' && body !== null && !Array.isArray(body);
}

function textOf(carried: Carried, field: string): string {
	const value = carried[field];
	return typeof value === 'string' ? value.trim() : '';
}

function carriedOf(body: unknown): Carried {
	if (!isCarried(body)) error(400, 'this call carried a body that is not a json object');
	return body;
}

function kindAsked(carried: Carried): PushDeviceKind {
	const named = textOf(carried, 'kind');
	if (named === '') return 'web-push';
	const kind = pushDeviceKinds.find((known) => known === named);
	if (!kind) error(400, `a device is reached by one of ${pushDeviceKinds.join(', ')}`);
	return kind;
}

function endpointAsked(carried: Carried): string {
	const endpoint = textOf(carried, 'endpoint');
	if (!endpoint) error(400, 'this call names the subscription it is about');
	return endpoint;
}

function encryptionKeysAsked(carried: Carried): EncryptionKeys {
	const publicKey = textOf(carried, 'publicKey');
	const authenticationSecret = textOf(carried, 'authenticationSecret');
	if (!publicKey || !authenticationSecret) {
		error(400, 'a claimed subscription carries both of the keys push is encrypted to');
	}
	return { p256dh: publicKey, auth: authenticationSecret };
}

function addressAsked(carried: Carried): PushDeviceAddress {
	return { kind: kindAsked(carried), endpoint: endpointAsked(carried) };
}

export function pushDeviceAddressOf(body: unknown): PushDeviceAddress {
	return addressAsked(carriedOf(body));
}

export function pushDeviceClaimOf(body: unknown): PushDeviceClaim {
	const carried = carriedOf(body);
	const address = addressAsked(carried);
	const encryptionKeys = address.kind === 'web-push' ? encryptionKeysAsked(carried) : null;
	return { ...address, encryptionKeys };
}

async function vaultedServerKey(caller: SupabaseClient): Promise<string> {
	const { data, error: refusal } = await caller.rpc('vapid_public_key');
	if (refusal) throw new Error(refusal.message);
	return typeof data === 'string' ? data : '';
}

async function hasClaimedDevice(caller: SupabaseClient): Promise<boolean> {
	const { data, error: refusal } = await caller
		.from('push_device')
		.select('address')
		.limit(1)
		.returns<{ address: string }[]>();
	if (refusal) throw new Error(refusal.message);
	return (data ?? []).length > 0;
}

export async function pushReachabilityOf(caller: SupabaseClient): Promise<PushReachability> {
	const serverKey = await vaultedServerKey(caller);
	return {
		serverKey,
		isServerKeyVaulted: serverKey !== '',
		hasClaimedDevice: await hasClaimedDevice(caller)
	};
}

export async function claimThePushDevice(
	caller: SupabaseClient,
	claim: PushDeviceClaim
): Promise<PushReachability> {
	const { error: refusal } = await caller.rpc('push_device_claim', {
		device_kind: claim.kind,
		device_address: claim.endpoint,
		device_keys: claim.encryptionKeys ?? {}
	});
	if (refusal) error(statusOfPostgresCode(refusal.code), refusal.message);
	return pushReachabilityOf(caller);
}

export async function releaseThePushDevice(
	caller: SupabaseClient,
	address: PushDeviceAddress
): Promise<PushReachability> {
	const { error: refusal } = await caller.rpc('push_device_release', {
		device_kind: address.kind,
		device_address: address.endpoint
	});
	if (refusal) error(statusOfPostgresCode(refusal.code), refusal.message);
	return pushReachabilityOf(caller);
}
