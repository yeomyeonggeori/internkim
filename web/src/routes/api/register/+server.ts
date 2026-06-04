import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { kv } from '$lib/kv';
import {
	createNodeSSHTunnel,
	createTunnel,
	configureNodeSSHTunnel,
	configureTunnel,
	createDNSRecord,
	ensureFleetDNSRecord,
	ensureNodeSSHDNSRecord,
	ensureWildcardDNSRecord,
	ensureOneTimePinIdentityProvider,
	ensureCompanionBypassApplication,
	ensureMaintenanceBypassApplication,
	ensureFleetCertificateCoverage,
	ensureNodeSSHAccessApplication,
	deleteDNSRecord,
	deleteTunnel,
	nodeSSHHostname
} from '$lib/cloudflare';
import { cloudflareEnvironment, ensureFleetAccessApplications, fleetAdminAccessEmails } from '$lib/fleet-access';
import type { Device, FleetMember } from '$lib/types';
import { hashFleetSecret, normalizeFleetID } from '$lib/device-auth';
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
	if (existingFleetSecretHash && existingFleetSecretHash !== fleetSecretHash) {
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
	if (auth !== `Bearer ${env.INTERNKIM_REGISTER_SECRET}`) {
		throw error(401, 'Invalid registration secret');
	}

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

	const cfEnv = cloudflareEnvironment(env);

	const existing = await kv.getDevice(env.KV, fleetID);
	const identityProviderId = await ensureOneTimePinIdentityProvider(cfEnv);
	if (existing) {
		const ownedDevice = await ensureSameFleet(existing, fleetSecret);
		if (newFleetID && newFleetID !== fleetID) {
			return migrateRegisteredFleet(env, cfEnv, ownedDevice, fleetID, newFleetID, requestedNodeID, nodeKey, adminEmail, identityProviderId);
		}
		const records = await kv.getUserRecords(env.KV, fleetID);
		const resolvedAdminEmail = adminEmail || ownedDevice.admin_email || '';
		const nodeID = resolveFleetNodeID(ownedDevice.fleet, requestedNodeID, nodeKey);
		const accessAdminEmails = accessAdminEmailsForRegistration(records, resolvedAdminEmail);
		const certificateCoverage = await ensureFleetCertificateCoverage(cfEnv, fleetID, nodeID);
		const memberSSH = await ensureFleetMemberSSH(cfEnv, ownedDevice.fleet, fleetID, nodeID, identityProviderId, accessAdminEmails);
		const fleet = registerFleetNode(ownedDevice.fleet, fleetID, nodeID, nodeKey, new Date(), memberSSH);
		const deviceWithFleet = { ...ownedDevice, admin_email: resolvedAdminEmail, fleet };
		await configureTunnel(cfEnv, ownedDevice.tunnel_id, fleetID);
		await ensureWildcardDNSRecord(cfEnv, ownedDevice.tunnel_id, fleetID);
		await ensurePublicBypassApplications(cfEnv, fleetID);
		const device = await ensureFleetAccessApplications(cfEnv, fleetID, identityProviderId, deviceWithFleet, records, resolvedAdminEmail);
		await kv.putDevice(env.KV, fleetID, device);
		return json({
			fleet_id: fleetID,
			node_id: nodeID,
			tunnel_token: device.tunnel_token,
			node_tunnel_token: memberSSH.nodeTunnelToken,
			url: `https://${fleetID}.${env.CF_DOMAIN}`,
			mattermost_url: `https://${fleetID}.${env.CF_DOMAIN}`,
			ssh_hostname: memberSSH.sshHostname,
			tls_certificate_status: certificateCoverage.status,
			...fleetResponseFields(device.fleet, nodeID)
		});
	}

	const fleetSecretHash = await hashFleetSecret(fleetSecret);
	const accessAdminEmails = accessAdminEmailsForRegistration([], adminEmail);
	const { tunnelId, tunnelToken } = await createTunnel(cfEnv, fleetID);
	await configureTunnel(cfEnv, tunnelId, fleetID);
	const dnsRecordId = await createDNSRecord(cfEnv, tunnelId, fleetID);
	await ensureWildcardDNSRecord(cfEnv, tunnelId, fleetID);
	await ensurePublicBypassApplications(cfEnv, fleetID);
	const nodeID = resolveFleetNodeID(undefined, requestedNodeID, nodeKey);
	const certificateCoverage = await ensureFleetCertificateCoverage(cfEnv, fleetID, nodeID);
	const memberSSH = await ensureFleetMemberSSH(cfEnv, undefined, fleetID, nodeID, identityProviderId, accessAdminEmails);

	const deviceWithoutAccess: Device = {
		fleet_id: fleetID,
		fleet_secret_hash: fleetSecretHash,
		tunnel_id: tunnelId,
		tunnel_token: tunnelToken,
		dns_record_id: dnsRecordId,
		admin_email: adminEmail,
		created_at: new Date().toISOString(),
		versions: {
			blueclaw: '',
			cli: ''
		},
		fleet: registerFleetNode(undefined, fleetID, nodeID, nodeKey, new Date(), memberSSH)
	};
	const device = await ensureFleetAccessApplications(cfEnv, fleetID, identityProviderId, deviceWithoutAccess, [], adminEmail);

	await kv.putDevice(env.KV, fleetID, device);
	await kv.putUserRecords(env.KV, fleetID, []);

	return json({
		fleet_id: fleetID,
		node_id: nodeID,
		tunnel_token: tunnelToken,
		node_tunnel_token: memberSSH.nodeTunnelToken,
		url: `https://${fleetID}.${env.CF_DOMAIN}`,
		mattermost_url: `https://${fleetID}.${env.CF_DOMAIN}`,
		ssh_hostname: memberSSH.sshHostname,
		tls_certificate_status: certificateCoverage.status,
		...fleetResponseFields(device.fleet, nodeID)
	});
}

async function handleRegistrationDelete(request: Request, platform: App.Platform | undefined) {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	const auth = request.headers.get('authorization');
	if (auth !== `Bearer ${env.INTERNKIM_REGISTER_SECRET}`) {
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

	const cfEnv = cloudflareEnvironment(env);
	const deletedResources: string[] = [];
	if (device.dns_record_id) {
		await deleteDNSRecord(cfEnv, device.dns_record_id);
		deletedResources.push('dns_record');
	}
	if (device.ssh_dns_record_id) {
		await deleteDNSRecord(cfEnv, device.ssh_dns_record_id);
		deletedResources.push('ssh_dns_record');
	}
	for (const member of device.fleet?.members ?? []) {
		if (member.sshDNSRecordID && member.sshDNSRecordID !== device.ssh_dns_record_id) {
			await deleteDNSRecord(cfEnv, member.sshDNSRecordID);
			deletedResources.push(`node_ssh_dns_record:${member.nodeID}`);
		}
		if (member.nodeTunnelID && member.nodeTunnelID !== device.tunnel_id) {
			await deleteTunnel(cfEnv, member.nodeTunnelID);
			deletedResources.push(`node_tunnel:${member.nodeID}`);
		}
	}
	if (device.tunnel_id) {
		await deleteTunnel(cfEnv, device.tunnel_id);
		deletedResources.push('tunnel');
	}

	await kv.deleteDevice(env.KV, fleetID);
	await kv.deleteUserRecords(env.KV, fleetID);

	return json({ deleted: true, fleet_id: fleetID, resources: deletedResources });
}

async function migrateRegisteredFleet(
	env: App.Platform['env'],
	cfEnv: Parameters<typeof createDNSRecord>[0],
	ownedDevice: Device,
	oldFleetID: string,
	newFleetID: string,
	requestedNodeID: string,
	nodeKey: string,
	adminEmail: string,
	identityProviderId: string
) {
	if (await kv.getDevice(env.KV, newFleetID)) {
		throw error(409, 'New fleet ID already registered');
	}

	const records = await kv.getUserRecords(env.KV, oldFleetID);
	const resolvedAdminEmail = adminEmail || ownedDevice.admin_email || '';
	const nodeID = resolveFleetNodeID(ownedDevice.fleet, requestedNodeID, nodeKey);
	const fleet = registerFleetNode(renameFleet(ownedDevice.fleet, newFleetID), newFleetID, nodeID, nodeKey, new Date());
	const accessAdminEmails = accessAdminEmailsForRegistration(records, resolvedAdminEmail);
	const certificateCoverage = await ensureFleetCertificateCoverage(cfEnv, newFleetID, nodeID);
	const memberSSH = await ensureFleetMemberSSH(cfEnv, fleet, newFleetID, nodeID, identityProviderId, accessAdminEmails, [oldFleetID]);
	const migratedFleet = registerFleetNode(fleet, newFleetID, nodeID, nodeKey, new Date(), memberSSH);
	const deviceWithoutAccess: Device = {
		...ownedDevice,
		fleet_id: newFleetID,
		access_app_id: undefined,
		access_policy_id: undefined,
		admin_email: resolvedAdminEmail,
		fleet: migratedFleet
	};

	await configureTunnel(cfEnv, ownedDevice.tunnel_id, newFleetID, [oldFleetID]);
	await ensureFleetDNSRecord(cfEnv, ownedDevice.tunnel_id, newFleetID);
	await ensureWildcardDNSRecord(cfEnv, ownedDevice.tunnel_id, newFleetID);
	await ensurePublicBypassApplications(cfEnv, newFleetID);
	const device = await ensureFleetAccessApplications(cfEnv, newFleetID, identityProviderId, deviceWithoutAccess, records, resolvedAdminEmail);

	await kv.putDevice(env.KV, newFleetID, device);
	await kv.putUserRecords(env.KV, newFleetID, records);

	return json({
		fleet_id: newFleetID,
		old_fleet_id: oldFleetID,
		node_id: nodeID,
		tunnel_token: device.tunnel_token,
		node_tunnel_token: memberSSH.nodeTunnelToken,
		url: `https://${newFleetID}.${env.CF_DOMAIN}`,
		mattermost_url: `https://${newFleetID}.${env.CF_DOMAIN}`,
		alias_url: `https://${oldFleetID}.${env.CF_DOMAIN}`,
		ssh_hostname: memberSSH.sshHostname,
		tls_certificate_status: certificateCoverage.status,
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

function accessAdminEmailsForRegistration(records: Parameters<typeof fleetAdminAccessEmails>[0], fallbackAdminEmail: string) {
	const emails = fleetAdminAccessEmails(records, fallbackAdminEmail);
	if (emails.length > 0) return emails;
	throw error(400, 'admin_email is required for SSH and admin access');
}

async function ensurePublicBypassApplications(cfEnv: Parameters<typeof ensureCompanionBypassApplication>[0], fleetID: string) {
	await ensureCompanionBypassApplication(cfEnv, fleetID);
	await ensureMaintenanceBypassApplication(cfEnv, fleetID);
}

async function ensureFleetMemberSSH(
	cfEnv: Parameters<typeof createDNSRecord>[0],
	fleet: Device['fleet'],
	fleetID: string,
	nodeID: string,
	identityProviderId: string,
	emails: string[] | string,
	aliasFleetIDs: string[] = []
): Promise<Pick<FleetMember, 'nodeTunnelID' | 'nodeTunnelToken' | 'sshDNSRecordID' | 'sshAccessAppID' | 'sshHostname'>> {
	const existingMember = findFleetMember(fleet, nodeID);
	const nodeTunnel = existingMember?.nodeTunnelID && existingMember.nodeTunnelToken
		? { tunnelId: existingMember.nodeTunnelID, tunnelToken: existingMember.nodeTunnelToken }
		: await createNodeSSHTunnel(cfEnv, fleetID, nodeID);
	await configureNodeSSHTunnel(cfEnv, nodeTunnel.tunnelId, fleetID, nodeID, aliasFleetIDs);
	const sshDNSRecordID = await ensureNodeSSHDNSRecord(cfEnv, nodeTunnel.tunnelId, fleetID, nodeID);
	const sshAccessAppID = await ensureNodeSSHAccessApplication(cfEnv, fleetID, nodeID, identityProviderId, emails);
	return {
		nodeTunnelID: nodeTunnel.tunnelId,
		nodeTunnelToken: nodeTunnel.tunnelToken,
		sshDNSRecordID,
		sshAccessAppID,
		sshHostname: nodeSSHHostname(cfEnv, fleetID, nodeID)
	};
}
