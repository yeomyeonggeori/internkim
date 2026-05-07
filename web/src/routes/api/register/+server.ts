import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { kv } from '$lib/kv';
import {
	createAccessApplication,
	createAccessPolicy,
	createNodeSSHTunnel,
	createTunnel,
	configureNodeSSHTunnel,
	configureTunnel,
	createDNSRecord,
	ensureNodeSSHDNSRecord,
	ensureWildcardDNSRecord,
	ensureOneTimePinIdentityProvider,
	ensureAdminAccessApplications,
	ensureCompanionBypassApplication,
	ensureNodeSSHAccessApplication,
	nodeSSHHostname,
	updateAccessApplicationLoginMethod,
	syncAccessPolicyEmails
} from '$lib/cloudflare';
import { adminEmails, userEmails } from '$lib/kv';
import type { Device, FleetMember } from '$lib/types';
import { hashDeviceSecret, normalizeDeviceID } from '$lib/device-auth';
import {
	activeFleetMembers,
	findFleetMember,
	fleetMemberStatus,
	fleetQuorumSize,
	normalizeBoardID,
	normalizeBoardKey,
	pendingFleetMembers,
	registerFleetBoard,
	resolveFleetBoardID
} from '$lib/fleet';

function normalizeEmail(email: string): string {
	return email.trim().toLowerCase();
}

function isValidDeviceID(deviceID: string): boolean {
	return /^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/.test(deviceID);
}

async function ensureSameDevice(device: Device, deviceSecret: string): Promise<Device> {
	const deviceSecretHash = await hashDeviceSecret(deviceSecret);
	if (device.device_secret_hash && device.device_secret_hash !== deviceSecretHash) {
		throw error(409, 'Device ID already registered');
	}
	return {
		...device,
		device_secret_hash: device.device_secret_hash ?? deviceSecretHash
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

async function handleRegister(request: Request, platform: App.Platform | undefined) {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	const auth = request.headers.get('authorization');
	if (auth !== `Bearer ${env.INTERNKIM_REGISTER_SECRET}`) {
		throw error(401, 'Invalid registration secret');
	}

	const { device_id, device_secret, admin_email, board_id, board_key } = (await request.json()) as {
		device_id: string;
		device_secret: string;
		admin_email: string;
		board_id?: string;
		board_key?: string;
	};
	const deviceID = normalizeDeviceID(device_id ?? '');
	const requestedBoardID = normalizeBoardID(board_id ?? '');
	const boardKey = normalizeBoardKey(board_key ?? board_id ?? deviceID);
	const adminEmail = normalizeEmail(admin_email ?? '');
	if (!deviceID || !device_secret) throw error(400, 'device_id and device_secret required');
	if (!isValidDeviceID(deviceID)) throw error(400, 'device_id must be a valid DNS label');
	if (requestedBoardID && !isValidDeviceID(requestedBoardID)) throw error(400, 'board_id must be a valid DNS label');
	if (!boardKey) throw error(400, 'board_key required');

	const cfEnv = {
		CF_API_TOKEN: env.CF_API_TOKEN,
		CF_ACCOUNT_ID: env.CF_ACCOUNT_ID,
		CF_ZONE_ID: env.CF_ZONE_ID,
		CF_DOMAIN: env.CF_DOMAIN
	};

	const existing = await kv.getDevice(env.KV, deviceID);
	const identityProviderId = await ensureOneTimePinIdentityProvider(cfEnv);
	if (existing) {
		const ownedDevice = await ensureSameDevice(existing, device_secret);
		const records = await kv.getUserRecords(env.KV, deviceID);
		const resolvedAdminEmail = adminEmail || ownedDevice.admin_email || '';
		const boardID = resolveFleetBoardID(ownedDevice.fleet, requestedBoardID, boardKey);
		const memberSSH = await ensureFleetMemberSSH(cfEnv, ownedDevice.fleet, deviceID, boardID, identityProviderId, sshAccessEmails(records, resolvedAdminEmail));
		const fleet = registerFleetBoard(ownedDevice.fleet, deviceID, boardID, boardKey, new Date(), memberSSH);
		await configureTunnel(cfEnv, ownedDevice.tunnel_id, deviceID);
		await ensureWildcardDNSRecord(cfEnv, ownedDevice.tunnel_id, deviceID);
		if (ownedDevice.access_app_id) {
			await updateAccessApplicationLoginMethod(cfEnv, deviceID, ownedDevice.access_app_id, identityProviderId);
		}
		await ensureCompanionBypassApplication(cfEnv, deviceID);
		await ensureAdminAccessApplications(cfEnv, deviceID, identityProviderId, []);
		const deviceWithAccess = await ensureAccessPolicy(cfEnv, { ...ownedDevice, admin_email: resolvedAdminEmail }, userEmails(records));
		const device = { ...deviceWithAccess, fleet };
		await kv.putDevice(env.KV, deviceID, device);
		return json({
			device_id: deviceID,
			board_id: boardID,
			tunnel_token: device.tunnel_token,
			node_tunnel_token: memberSSH.nodeTunnelToken,
			url: `https://${deviceID}.${env.CF_DOMAIN}`,
			mattermost_url: `https://${deviceID}.${env.CF_DOMAIN}`,
			ssh_hostname: memberSSH.sshHostname,
			...fleetResponseFields(device.fleet, boardID)
		});
	}

	const deviceSecretHash = await hashDeviceSecret(device_secret);
	const { tunnelId, tunnelToken } = await createTunnel(cfEnv, deviceID);
	await configureTunnel(cfEnv, tunnelId, deviceID);
	const dnsRecordId = await createDNSRecord(cfEnv, tunnelId, deviceID);
	await ensureWildcardDNSRecord(cfEnv, tunnelId, deviceID);
	const accessAppId = await createAccessApplication(cfEnv, deviceID, identityProviderId);
	const accessPolicyId = await createAccessPolicy(cfEnv, accessAppId, adminEmail);
	await ensureAdminAccessApplications(cfEnv, deviceID, identityProviderId, adminEmail);
	await ensureCompanionBypassApplication(cfEnv, deviceID);
	const boardID = resolveFleetBoardID(undefined, requestedBoardID, boardKey);
	const memberSSH = await ensureFleetMemberSSH(cfEnv, undefined, deviceID, boardID, identityProviderId, adminEmail);

	const device: Device = {
		device_id: deviceID,
		device_secret_hash: deviceSecretHash,
		tunnel_id: tunnelId,
		tunnel_token: tunnelToken,
		dns_record_id: dnsRecordId,
		access_app_id: accessAppId,
		access_policy_id: accessPolicyId,
		admin_email: adminEmail,
		created_at: new Date().toISOString(),
		versions: {
			blueclaw: '',
			cli: ''
		},
		fleet: registerFleetBoard(undefined, deviceID, boardID, boardKey, new Date(), memberSSH)
	};

	await kv.putDevice(env.KV, deviceID, device);
	await kv.putUserRecords(env.KV, deviceID, []);

	return json({
		device_id: deviceID,
		board_id: boardID,
		tunnel_token: tunnelToken,
		node_tunnel_token: memberSSH.nodeTunnelToken,
		url: `https://${deviceID}.${env.CF_DOMAIN}`,
		mattermost_url: `https://${deviceID}.${env.CF_DOMAIN}`,
		ssh_hostname: memberSSH.sshHostname,
		...fleetResponseFields(device.fleet, boardID)
	});
}

function fleetResponseFields(fleet: Device['fleet'], boardID: string) {
	if (!fleet) {
		return {
			fleet_role: 'active',
			fleet_active_count: 1,
			fleet_pending_count: 0,
			fleet_quorum_size: 1
		};
	}
	return {
		fleet_role: fleetMemberStatus(fleet, boardID),
		fleet_active_count: activeFleetMembers(fleet).length,
		fleet_pending_count: pendingFleetMembers(fleet).length,
		fleet_quorum_size: fleetQuorumSize(fleet)
	};
}

function sshAccessEmails(records: Parameters<typeof adminEmails>[0], fallbackAdminEmail: string) {
	const emails = adminEmails(records);
	if (emails.length > 0 || !fallbackAdminEmail.trim()) return emails;
	return [fallbackAdminEmail.trim().toLowerCase()];
}

async function ensureFleetMemberSSH(
	cfEnv: Parameters<typeof createDNSRecord>[0],
	fleet: Device['fleet'],
	deviceID: string,
	boardID: string,
	identityProviderId: string,
	emails: string[] | string
): Promise<Pick<FleetMember, 'nodeTunnelID' | 'nodeTunnelToken' | 'sshDNSRecordID' | 'sshAccessAppID' | 'sshHostname'>> {
	const existingMember = findFleetMember(fleet, boardID);
	const nodeTunnel = existingMember?.nodeTunnelID && existingMember.nodeTunnelToken
		? { tunnelId: existingMember.nodeTunnelID, tunnelToken: existingMember.nodeTunnelToken }
		: await createNodeSSHTunnel(cfEnv, deviceID, boardID);
	await configureNodeSSHTunnel(cfEnv, nodeTunnel.tunnelId, deviceID, boardID);
	const sshDNSRecordID = await ensureNodeSSHDNSRecord(cfEnv, nodeTunnel.tunnelId, deviceID, boardID);
	const sshAccessAppID = await ensureNodeSSHAccessApplication(cfEnv, deviceID, boardID, identityProviderId, emails);
	return {
		nodeTunnelID: nodeTunnel.tunnelId,
		nodeTunnelToken: nodeTunnel.tunnelToken,
		sshDNSRecordID,
		sshAccessAppID,
		sshHostname: nodeSSHHostname(cfEnv, deviceID, boardID)
	};
}

async function ensureAccessPolicy(
	cfEnv: Parameters<typeof createAccessPolicy>[0],
	device: Device,
	emails: string[]
): Promise<Device> {
	if (device.access_app_id) {
		try {
			const accessPolicyId = await syncAccessPolicyEmails(cfEnv, device.access_app_id, emails);
			return {
				...device,
				access_policy_id: device.access_policy_id ?? accessPolicyId ?? undefined
			};
		} catch (caughtError) {
			const message = caughtError instanceof Error ? caughtError.message : String(caughtError);
			if (!message.includes('unknown_application')) {
				throw caughtError;
			}
		}
	}

	const identityProviderId = await ensureOneTimePinIdentityProvider(cfEnv);
	const accessAppId = await createAccessApplication(cfEnv, device.device_id, identityProviderId);
	const accessPolicyId = await createAccessPolicy(cfEnv, accessAppId, emails[0]);
	await syncAccessPolicyEmails(cfEnv, accessAppId, emails);

	return {
		...device,
		access_app_id: accessAppId,
		access_policy_id: accessPolicyId
	};
}
