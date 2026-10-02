import type { SupabaseClient } from '@supabase/supabase-js';
import { z } from 'zod';
import {
	nearbyNetworkListSchema,
	sealedSecretSchema,
	wifiOutcomeResultSchema,
	type NearbyNetwork,
	type SealedSecret,
	type WifiChangeStatus,
	type WifiOutcomeResult
} from '$lib/company/box';
import { BoxRefused, boxOfCompany, writeBoxSettings, type BoxRow } from './box';

export const wifiChangeRequestSchema = z.object({
	requestID: z.uuid(),
	sealed: sealedSecretSchema
}).strict();

export const wifiOutcomeReportSchema = z.object({
	requestID: z.uuid(),
	result: wifiOutcomeResultSchema
}).strict();

export const wifiChangeFetchSchema = z.object({
	nearbyNetworks: nearbyNetworkListSchema.optional()
}).strict();

export type PendingWifiChange = { requestID: string; sealed: SealedSecret } | null;

export async function requestWifiChange(
	client: SupabaseClient,
	companyID: string,
	requestID: string,
	sealed: SealedSecret,
	now: Date = new Date()
): Promise<WifiChangeStatus> {
	const box = await boxOfCompany(client, companyID);
	if (!box) throw new BoxRefused('connect the company computer before changing its Wi-Fi');
	const { wifiOutcome, ...settingsWithoutOutcome } = box.settings;
	const settings = { ...settingsWithoutOutcome, wifiChange: { requestID, sealed, requestedAt: now.toISOString() } };
	await writeBoxSettings(client, box, settings);
	return wifiChangeStatusOf(settings);
}

export async function wifiChangeStatusFor(client: SupabaseClient, companyID: string): Promise<WifiChangeStatus> {
	const box = await boxOfCompany(client, companyID);
	if (!box) throw new BoxRefused('connect the company computer before changing its Wi-Fi');
	return wifiChangeStatusOf(box.settings);
}

export async function recordNearbyNetworks(
	client: SupabaseClient,
	box: BoxRow,
	networks: NearbyNetwork[],
	now: Date = new Date()
): Promise<void> {
	const latest = (await boxOfCompany(client, box.companyID)) ?? box;
	const stored = latest.settings.nearbyNetworks;
	if (stored && !nearbyNetworksDiffer(stored.networks, networks)) return;
	await writeBoxSettings(client, latest, { ...latest.settings, nearbyNetworks: { networks, scannedAt: now.toISOString() } });
}

export function nearbyNetworksDiffer(stored: NearbyNetwork[], reported: NearbyNetwork[]): boolean {
	const identityOf = (network: NearbyNetwork): string =>
		JSON.stringify([network.ssid, network.isSecured, network.isConnected ?? false]);
	const storedIdentities = new Set(stored.map(identityOf));
	const reportedIdentities = new Set(reported.map(identityOf));
	if (storedIdentities.size !== reportedIdentities.size) return true;
	return [...reportedIdentities].some((identity) => !storedIdentities.has(identity));
}

export function pendingWifiChangeOf(box: BoxRow): PendingWifiChange {
	const wifiChange = box.settings.wifiChange;
	return wifiChange ? { requestID: wifiChange.requestID, sealed: wifiChange.sealed } : null;
}

export async function reportWifiOutcome(
	client: SupabaseClient,
	box: BoxRow,
	requestID: string,
	result: WifiOutcomeResult,
	now: Date = new Date()
): Promise<void> {
	const latest = (await boxOfCompany(client, box.companyID)) ?? box;
	if (latest.settings.wifiChange?.requestID !== requestID) {
		throw new BoxRefused(`no pending Wi-Fi change with request ${requestID}`);
	}
	const { wifiChange, ...settingsWithoutChange } = latest.settings;
	await writeBoxSettings(client, latest, {
		...settingsWithoutChange,
		wifiOutcome: { requestID, result, reportedAt: now.toISOString() }
	});
}

function wifiChangeStatusOf(settings: BoxRow['settings']): WifiChangeStatus {
	return {
		pendingRequestID: settings.wifiChange?.requestID ?? null,
		outcome: settings.wifiOutcome ?? null,
		nearbyNetworks: settings.nearbyNetworks?.networks ?? [],
		scannedAt: settings.nearbyNetworks?.scannedAt ?? null
	};
}
