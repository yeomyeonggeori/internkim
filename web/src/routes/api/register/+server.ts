import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { kv } from '$lib/kv';
import {
	createAccessApplication,
	createAccessPolicy,
	createTunnel,
	configureTunnel,
	createDNSRecord,
	syncAccessPolicyEmails
} from '$lib/cloudflare';
import type { Device } from '$lib/types';
import { hashDeviceSecret, normalizeDeviceID } from '$lib/device-auth';

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

	const { device_id, device_secret, admin_email } = (await request.json()) as {
		device_id: string;
		device_secret: string;
		admin_email: string;
	};
	const deviceID = normalizeDeviceID(device_id ?? '');
	const adminEmail = normalizeEmail(admin_email ?? '');
	if (!deviceID || !device_secret || !adminEmail) throw error(400, 'device_id, device_secret, and admin_email required');
	if (!isValidDeviceID(deviceID)) throw error(400, 'device_id must be a valid DNS label');

	const cfEnv = {
		CF_API_TOKEN: env.CF_API_TOKEN,
		CF_ACCOUNT_ID: env.CF_ACCOUNT_ID,
		CF_ZONE_ID: env.CF_ZONE_ID,
		CF_DOMAIN: env.CF_DOMAIN
	};

	const existing = await kv.getDevice(env.KV, deviceID);
	if (existing) {
		const ownedDevice = await ensureSameDevice(existing, device_secret);
		await configureTunnel(cfEnv, ownedDevice.tunnel_id, deviceID);
		const users = await kv.getUsers(env.KV, deviceID);
		const seededUsers = users.length > 0 ? users : [normalizeEmail(existing.admin_email || adminEmail)];
		const device = await ensureAccessPolicy(cfEnv, ownedDevice, seededUsers);
		await kv.putUsers(env.KV, deviceID, seededUsers);
		await kv.putDevice(env.KV, deviceID, device);
		return json({
			device_id: deviceID,
			tunnel_token: device.tunnel_token,
			url: `https://${deviceID}.${env.CF_DOMAIN}`,
			mattermost_url: `https://${deviceID}.${env.CF_DOMAIN}`
		});
	}

	const deviceSecretHash = await hashDeviceSecret(device_secret);
	const { tunnelId, tunnelToken } = await createTunnel(cfEnv, deviceID);
	await configureTunnel(cfEnv, tunnelId, deviceID);
	const dnsRecordId = await createDNSRecord(cfEnv, tunnelId, deviceID);
	const accessAppId = await createAccessApplication(cfEnv, deviceID);
	const accessPolicyId = await createAccessPolicy(cfEnv, accessAppId, adminEmail);

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
		}
	};

	await kv.putDevice(env.KV, deviceID, device);
	await kv.putUsers(env.KV, deviceID, [adminEmail]);

	return json({
		device_id: deviceID,
		tunnel_token: tunnelToken,
		url: `https://${deviceID}.${env.CF_DOMAIN}`,
		mattermost_url: `https://${deviceID}.${env.CF_DOMAIN}`
	});
}

async function ensureAccessPolicy(
	cfEnv: Parameters<typeof createAccessApplication>[0],
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

	const accessAppId = await createAccessApplication(cfEnv, device.device_id);
	const accessPolicyId = await createAccessPolicy(cfEnv, accessAppId, emails[0]);
	await syncAccessPolicyEmails(cfEnv, accessAppId, emails);

	return {
		...device,
		access_app_id: accessAppId,
		access_policy_id: accessPolicyId
	};
}
