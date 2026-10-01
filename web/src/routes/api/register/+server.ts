import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { kv } from '$lib/kv';
import type { Device, FleetMember } from '$lib/types';
import { hashFleetSecret, isTheRegisterSecret, normalizeFleetID } from '$lib/device-auth';
import { isTheSameSecret } from '$lib/server/same-secret';
import {
	activeFleetMembers,
	findFleetMember,
	fleetMemberStatus,
	fleetQuorumSize,
	normalizeNodeID,
	normalizeNodeKey,
	pendingFleetMembers,
	registerFleetNode,
	resolveFleetNodeID
} from '$lib/fleet';

function normalizeEmail(email: string): string {
	return email.trim().toLowerCase();
}

function isValidDNSLabel(value: string): boolean {
	return /^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/.test(value);
}

async function ensureSameFleet(device: Device, fleetSecret: string): Promise<Device> {
	const fleetSecretHash = await hashFleetSecret(fleetSecret);
	const existingFleetSecretHash = device.fleet_secret_hash;
	if (existingFleetSecretHash && !(await isTheSameSecret(fleetSecretHash, existingFleetSecretHash))) {
		throw error(409, 'Fleet ID already registered');
	}
	return {
		...device,
		fleet_secret_hash: existingFleetSecretHash ?? fleetSecretHash
	};
}

export const POST: RequestHandler = async ({ request, platform }) => {
	try {
		return await handleRegister(request, platform);
	} catch (caughtError) {
		if (caughtError && typeof caughtError === 'object' && 'status' in caughtError) {
			throw caughtError;
		}
		const message = caughtError instanceof Error ? caughtError.message : String(caughtError);
		return json({ error: 'registration failed', details: message }, { status: 500 });
	}
};

export const DELETE: RequestHandler = async ({ request, platform }) => {
	try {
		return await handleRegistrationDelete(request, platform);
	} catch (caughtError) {
		if (caughtError && typeof caughtError === 'object' && 'status' in caughtError) {
			throw caughtError;
		}
		const message = caughtError instanceof Error ? caughtError.message : String(caughtError);
		return json({ error: 'registration cleanup failed', details: message }, { status: 500 });
	}
};

async function handleRegister(request: Request, platform: App.Platform | undefined) {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	const auth = request.headers.get('authorization');
	const requestBody = (await request.json()) as {
		fleet_id?: string;
		fleet_secret?: string;
		node_id?: string;
		node_key?: string;
		new_fleet_id?: string;
		admin_email: string;
	};
	const fleetID = normalizeFleetID(requestBody.fleet_id ?? '');
	const newFleetID = normalizeFleetID(requestBody.new_fleet_id ?? '');
	const fleetSecret = requestBody.fleet_secret ?? '';
	const requestedNodeID = normalizeNodeID(requestBody.node_id ?? '');
	const nodeKey = normalizeNodeKey(requestBody.node_key ?? requestBody.node_id ?? fleetID);
	const adminEmail = normalizeEmail(requestBody.admin_email ?? '');
	if (!fleetID || !fleetSecret) throw error(400, 'fleet_id and fleet_secret required');
	if (!isValidDNSLabel(fleetID)) throw error(400, 'fleet_id must be a valid DNS label');
	if (newFleetID && !isValidDNSLabel(newFleetID)) throw error(400, 'new_fleet_id must be a valid DNS label');
	if (requestedNodeID && !isValidDNSLabel(requestedNodeID)) throw error(400, 'node_id must be a valid DNS label');
	if (!nodeKey) throw error(400, 'node_key required');

	const existing = await kv.getDevice(env.KV, fleetID);
	const hasRegisterSecret = await isTheRegisterSecret(auth, env.INTERNKIM_REGISTER_SECRET);
	if (!existing && !hasRegisterSecret) {
		throw error(401, 'Invalid registration secret');
	}
	if (existing) {
		const ownedDevice = await ensureSameFleet(existing, fleetSecret);
		validateExistingFleetRegistrationAuthority(hasRegisterSecret, ownedDevice, newFleetID, adminEmail);
		if (newFleetID && newFleetID !== fleetID) {
			return migrateRegisteredFleet(env, ownedDevice, fleetID, newFleetID, requestedNodeID, nodeKey, adminEmail);
		}
		const resolvedAdminEmail = existingFleetAdminEmail(hasRegisterSecret, ownedDevice, adminEmail);
		const nodeID = resolveFleetNodeID(ownedDevice.fleet, requestedNodeID, nodeKey);
		const fleet = registerFleetNode(ownedDevice.fleet, fleetID, nodeID, nodeKey, new Date());
		const device = { ...ownedDevice, admin_email: resolvedAdminEmail, fleet };
		await kv.putDevice(env.KV, fleetID, device);
		return json({
			fleet_id: fleetID,
			node_id: nodeID,
			...fleetResponseFields(device.fleet, nodeID)
		});
	}

	const fleetSecretHash = await hashFleetSecret(fleetSecret);
	const nodeID = resolveFleetNodeID(undefined, requestedNodeID, nodeKey);
	const device: Device = {
		fleet_id: fleetID,
		fleet_secret_hash: fleetSecretHash,
		admin_email: adminEmail,
		created_at: new Date().toISOString(),
		versions: {
			blueclaw: '',
			cli: ''
		},
		fleet: registerFleetNode(undefined, fleetID, nodeID, nodeKey, new Date())
	};

	await kv.putDevice(env.KV, fleetID, device);

	return json({
		fleet_id: fleetID,
		node_id: nodeID,
		...fleetResponseFields(device.fleet, nodeID)
	});
}

function validateExistingFleetRegistrationAuthority(hasRegisterSecret: boolean, device: Device, newFleetID: string, adminEmail: string) {
	if (hasRegisterSecret) return;
	if (newFleetID && newFleetID !== normalizeFleetID(device.fleet_id)) {
		throw error(401, 'Register secret required for fleet migration');
	}
	const currentAdminEmail = normalizeEmail(device.admin_email ?? '');
	if (adminEmail && adminEmail !== currentAdminEmail) {
		throw error(401, 'Register secret required for admin email changes');
	}
}

function existingFleetAdminEmail(hasRegisterSecret: boolean, device: Device, adminEmail: string) {
	if (!hasRegisterSecret) return normalizeEmail(device.admin_email ?? '');
	return adminEmail || normalizeEmail(device.admin_email ?? '');
}

async function handleRegistrationDelete(request: Request, platform: App.Platform | undefined) {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	if (!(await isTheRegisterSecret(request.headers.get('authorization'), env.INTERNKIM_REGISTER_SECRET))) {
		throw error(401, 'Invalid registration secret');
	}

	const requestBody = (await request.json()) as {
		fleet_id?: string;
		fleet_secret?: string;
	};
	const fleetID = normalizeFleetID(requestBody.fleet_id ?? '');
	const fleetSecret = requestBody.fleet_secret ?? '';
	if (!fleetID || !fleetSecret) throw error(400, 'fleet_id and fleet_secret required');

	const device = await kv.getDevice(env.KV, fleetID);
	if (!device) {
		return json({ deleted: false, reason: 'not_found' });
	}
	await ensureSameFleet(device, fleetSecret);

	await kv.deleteDevice(env.KV, fleetID);

	return json({ deleted: true, fleet_id: fleetID });
}

async function migrateRegisteredFleet(
	env: App.Platform['env'],
	ownedDevice: Device,
	oldFleetID: string,
	newFleetID: string,
	requestedNodeID: string,
	nodeKey: string,
	adminEmail: string
) {
	if (await kv.getDevice(env.KV, newFleetID)) {
		throw error(409, 'New fleet ID already registered');
	}

	const resolvedAdminEmail = adminEmail || ownedDevice.admin_email || '';
	const nodeID = resolveFleetNodeID(ownedDevice.fleet, requestedNodeID, nodeKey);
	const fleet = registerFleetNode(renameFleet(ownedDevice.fleet, newFleetID), newFleetID, nodeID, nodeKey, new Date());
	const device: Device = {
		...ownedDevice,
		fleet_id: newFleetID,
		admin_email: resolvedAdminEmail,
		fleet
	};

	await kv.putDevice(env.KV, newFleetID, device);
	await kv.deleteDevice(env.KV, oldFleetID);

	return json({
		fleet_id: newFleetID,
		node_id: nodeID,
		...fleetResponseFields(device.fleet, nodeID)
	});
}

function renameFleet(fleet: Device['fleet'], fleetID: string): Device['fleet'] {
	if (!fleet) return undefined;
	return {
		...fleet,
		fleetID
	};
}

function fleetResponseFields(fleet: Device['fleet'], nodeID: string) {
	if (!fleet) {
		return {
			fleet_role: 'active',
			fleet_active_count: 1,
			fleet_pending_count: 0,
			fleet_quorum_size: 1
		};
	}
	return {
		fleet_role: fleetMemberStatus(fleet, nodeID),
		fleet_active_count: activeFleetMembers(fleet).length,
		fleet_pending_count: pendingFleetMembers(fleet).length,
		fleet_quorum_size: fleetQuorumSize(fleet)
	};
}
