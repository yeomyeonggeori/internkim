import {
	deleteRootAccessApplication,
	ensureAdminAccessApplications,
	ensureOneTimePinIdentityProvider,
	ensureWebSessionAccessApplication,
	syncSSHAccessPolicyEmails,
	type CFEnv
} from './cloudflare';
import { adminEmails } from './kv';
import type { Device, UserRecord } from './types';

function normalizeEmail(email: string): string {
	return email.trim().toLowerCase();
}

export function cloudflareEnvironment(env: App.Platform['env']): CFEnv {
	return {
		CF_API_TOKEN: env.CF_API_TOKEN,
		CF_ACCOUNT_ID: env.CF_ACCOUNT_ID,
		CF_ZONE_ID: env.CF_ZONE_ID,
		CF_DOMAIN: env.CF_DOMAIN
	};
}

export function fleetAdminAccessEmails(records: UserRecord[], fallbackAdminEmail: string): string[] {
	const emails = adminEmails(records);
	if (emails.length > 0) return emails;
	if (records.length > 0) return [];
	const fallbackEmail = normalizeEmail(fallbackAdminEmail);
	return fallbackEmail ? [fallbackEmail] : [];
}

export function fleetWebSessionAccessEmails(records: UserRecord[], fallbackAdminEmail: string): string[] {
	const emails = records
		.filter((record) => !record.isIncomplete && record.status !== 'inactive')
		.map((record) => normalizeEmail(record.email))
		.filter(Boolean);
	if (emails.length > 0) return [...new Set(emails)];
	const fallbackEmail = normalizeEmail(fallbackAdminEmail);
	return fallbackEmail ? [fallbackEmail] : [];
}

function fleetSSHAccessApplicationIDs(device: Device): string[] {
	const values = [
		device.ssh_access_app_id,
		...(device.fleet?.members ?? []).map((member) => member.sshAccessAppID)
	];
	return [...new Set(values.filter((value): value is string => Boolean(value)))];
}

export async function syncFleetAccessPolicies(env: App.Platform['env'], fleetID: string, device: Device, records: UserRecord[]) {
	const cloudflareEnv = cloudflareEnvironment(env);
	const identityProviderID = await ensureOneTimePinIdentityProvider(cloudflareEnv);
	return ensureFleetAccessApplications(cloudflareEnv, fleetID, identityProviderID, device, records, device.admin_email);
}

export async function ensureFleetAccessApplications(
	env: CFEnv,
	fleetID: string,
	identityProviderID: string,
	device: Device,
	records: UserRecord[],
	fallbackAdminEmail: string
): Promise<Device> {
	const adminAccessEmails = fleetAdminAccessEmails(records, fallbackAdminEmail);
	if (adminAccessEmails.length === 0) {
		throw new Error('Cloudflare admin and SSH access require at least one admin user');
	}

	const deviceWithoutRootAccess = await removeLegacyRootAccessApplication(env, fleetID, device);
	await ensureAdminAccessApplications(env, fleetID, identityProviderID, adminAccessEmails);
	await ensureWebSessionAccessApplication(env, fleetID, identityProviderID, fleetWebSessionAccessEmails(records, fallbackAdminEmail));
	await Promise.all(
		fleetSSHAccessApplicationIDs(deviceWithoutRootAccess).map((applicationID) =>
			syncSSHAccessPolicyEmails(env, applicationID, adminAccessEmails)
		)
	);

	return deviceWithoutRootAccess;
}

async function removeLegacyRootAccessApplication(
	env: CFEnv,
	fleetID: string,
	device: Device
): Promise<Device> {
	await deleteRootAccessApplication(env, fleetID, device.access_app_id);
	return {
		...device,
		access_app_id: undefined,
		access_policy_id: undefined
	};
}
