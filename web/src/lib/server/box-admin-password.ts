import type { SupabaseClient } from '@supabase/supabase-js';
import { z } from 'zod';
import {
	adminPasswordOutcomeResultSchema,
	boxKeySchema,
	sealedSecretSchema,
	type AdminPasswordOutcomeResult,
	type AdminPasswordStatus,
	type SealedSecret
} from '$lib/company/box';
import { BoxRefused, boxOfCompany, boxOfPublicKey, writeBoxSettings, type BoxRow } from './box';

export const adminPasswordRequestSchema = z.object({
	publicKey: boxKeySchema,
	settingID: z.uuid(),
	sealed: sealedSecretSchema
}).strict();

export const adminPasswordOutcomeReportSchema = z.object({
	settingID: z.uuid(),
	result: adminPasswordOutcomeResultSchema
}).strict();

export type PendingAdminPassword = { settingID: string; sealed: SealedSecret } | null;

export async function requestAdminPassword(
	client: SupabaseClient,
	companyID: string,
	publicKey: string,
	settingID: string,
	sealed: SealedSecret,
	now: Date = new Date()
): Promise<AdminPasswordStatus> {
	const box = await boxOfCompany(client, companyID, publicKey);
	if (!box) throw new BoxRefused('connect the company computer before setting its admin password');
	if (!box.settings.hasAdminAccount) throw new BoxRefused('this company computer has no admin account to set a password for');
	const { adminPasswordOutcome, ...settingsWithoutOutcome } = box.settings;
	const settings = { ...settingsWithoutOutcome, adminPassword: { settingID, sealed, requestedAt: now.toISOString() } };
	await writeBoxSettings(client, box, settings);
	return adminPasswordStatusOf(settings);
}

export async function adminPasswordStatusFor(
	client: SupabaseClient,
	companyID: string,
	publicKey: string
): Promise<AdminPasswordStatus> {
	const box = await boxOfCompany(client, companyID, publicKey);
	if (!box) throw new BoxRefused('connect the company computer before setting its admin password');
	return adminPasswordStatusOf(box.settings);
}

export async function noteAdminAccount(client: SupabaseClient, box: BoxRow): Promise<void> {
	if (box.settings.hasAdminAccount) return;
	const latest = (await boxOfPublicKey(client, box.publicKey)) ?? box;
	await writeBoxSettings(client, latest, { ...latest.settings, hasAdminAccount: true });
}

export function pendingAdminPasswordOf(box: BoxRow): PendingAdminPassword {
	const adminPassword = box.settings.adminPassword;
	return adminPassword ? { settingID: adminPassword.settingID, sealed: adminPassword.sealed } : null;
}

export async function reportAdminPasswordOutcome(
	client: SupabaseClient,
	box: BoxRow,
	settingID: string,
	result: AdminPasswordOutcomeResult,
	now: Date = new Date()
): Promise<void> {
	const latest = (await boxOfPublicKey(client, box.publicKey)) ?? box;
	if (latest.settings.adminPassword?.settingID !== settingID) {
		throw new BoxRefused(`no pending admin password with setting ${settingID}`);
	}
	const { adminPassword, ...settingsWithoutPassword } = latest.settings;
	await writeBoxSettings(client, latest, {
		...settingsWithoutPassword,
		adminPasswordOutcome: { settingID, result, reportedAt: now.toISOString() }
	});
}

function adminPasswordStatusOf(settings: BoxRow['settings']): AdminPasswordStatus {
	return {
		hasAdminAccount: settings.hasAdminAccount ?? false,
		pendingSettingID: settings.adminPassword?.settingID ?? null,
		outcome: settings.adminPasswordOutcome ?? null
	};
}
