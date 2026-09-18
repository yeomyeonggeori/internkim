import { adminApiFetch } from '$lib/admin-api';
import { callCompanyApp } from '$lib/host-bridge';
import { isSupabaseConfigured } from '$lib/supabase';

export type MyCompanion = {
	companionID: string;
	displayName: string;
	isOnline: boolean;
	lastSeenAt: string;
	canControlComputer: boolean;
};

export type CompanionPairing = {
	code: string;
	expiresAt: string;
	installCommand: string;
	pairCommand: string;
	serviceCommand: string;
};

const computerControlToolName = 'computer_task';

export function readMyCompanions(value: unknown): MyCompanion[] {
	if (!isRecord(value) || !Array.isArray(value.companions)) throw new Error('the companion list is not a list');
	return value.companions.map(readMyCompanion);
}

function readMyCompanion(value: unknown): MyCompanion {
	if (!isRecord(value) || typeof value.companionID !== 'string') throw new Error('a companion carried no id');
	return {
		companionID: value.companionID,
		displayName: typeof value.displayName === 'string' && value.displayName.trim() ? value.displayName : value.companionID,
		isOnline: value.isOnline === true,
		lastSeenAt: typeof value.lastSeenAt === 'string' ? value.lastSeenAt : '',
		canControlComputer: advertisesComputerControl(value.capabilities)
	};
}

function advertisesComputerControl(capabilities: unknown): boolean {
	if (!Array.isArray(capabilities)) return false;
	return capabilities.some((descriptor) => isRecord(descriptor) && descriptor.name === computerControlToolName);
}

export function readCompanionPairing(value: unknown): CompanionPairing {
	if (!isRecord(value)) throw new Error('the pairing code answer is not an object');
	return {
		code: requiredText(value, 'code'),
		expiresAt: requiredText(value, 'expiresAt'),
		installCommand: requiredText(value, 'installCommand'),
		pairCommand: requiredText(value, 'pairCommand'),
		serviceCommand: requiredText(value, 'serviceCommand')
	};
}

function requiredText(record: Record<string, unknown>, field: string): string {
	const offered = record[field];
	if (typeof offered !== 'string' || !offered) throw new Error(`the pairing code answer carried no ${field}`);
	return offered;
}

export async function fetchMyCompanions(): Promise<MyCompanion[]> {
	return readMyCompanions(await askTheCompany('person.companion.mine', '/companion/api/mine'));
}

export async function issueCompanionPairing(): Promise<CompanionPairing> {
	return readCompanionPairing(await askTheCompany('person.companion.pairing_code', '/companion/api/pairing-codes', {}));
}

export async function disconnectMyCompanion(companionID: string): Promise<void> {
	await askTheCompany('person.companion.disconnect', '/companion/api/mine/disconnect', { companionID });
}

async function askTheCompany(
	capability: string,
	path: string,
	body?: Record<string, unknown>
): Promise<unknown> {
	if (isSupabaseConfigured()) {
		const answer = await callCompanyApp({ capability, body });
		if (answer.status >= 400) throw new Error(refusalText(answer.body) ?? `the company computer answered ${answer.status}`);
		return answer.body;
	}
	const response = await adminApiFetch(path, body ? { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) } : {});
	if (!response.ok) throw new Error((await response.text()).trim() || `the company computer answered ${response.status}`);
	return response.json();
}

function refusalText(body: unknown): string | null {
	if (typeof body === 'string' && body.trim()) return body.trim();
	if (isRecord(body) && typeof body.error === 'string') return body.error;
	return null;
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}
