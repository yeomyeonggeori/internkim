import { adminApiFetch } from '$lib/admin-api';
import { callCompanyApp } from '$lib/host-bridge';
import { isSupabaseConfigured } from '$lib/supabase';
import type { MemoryChange } from './memory-change';

export const memoryFactKinds = ['identity', 'preference', 'fact', 'episode', 'temporary'] as const;
export type MemoryFactKind = (typeof memoryFactKinds)[number];

export type MemoryFact = {
	factID: string;
	episodeID: string;
	ownerPersonID: string;
	circleIDs: string[];
	kind: MemoryFactKind;
	content: string;
	validFrom: string;
	validUntil?: string;
	reinforcementCount: number;
	lastRecalledAt?: string;
};

export type MemoryProfile = {
	identityLines: string[];
	currentLines: string[];
	builtAt?: string;
};

export type MemoryFactsResponse = {
	personID: string;
	embeddingModel?: string;
	profile: MemoryProfile;
	facts: MemoryFact[];
};

const defaultLimit = 200;

export async function fetchMemoryFacts(): Promise<MemoryFactsResponse> {
	const document = isSupabaseConfigured() ? await askTheCompanyApp() : await askTheDevice();
	return normalizeMemoryFactsResponse(document);
}

export function factForgetRequest(factIDs: string[], reason: string): MemoryChange {
	const trimmedReason = reason.trim();
	return {
		capability: 'person.memory.facts.forget',
		path: '/memory/api/facts/forget',
		body: { factIDs, ...(trimmedReason ? { reason: trimmedReason } : {}) }
	};
}

export async function forgetMemoryFact(factID: string, reason: string): Promise<void> {
	await changeMemory(factForgetRequest([factID], reason));
}

export function normalizeMemoryFactsResponse(document: unknown): MemoryFactsResponse {
	const record = readRecord(document) ?? {};
	const embeddingModel = readString(record.embeddingModel);
	return {
		personID: readString(record.personID) ?? '',
		...(embeddingModel ? { embeddingModel } : {}),
		profile: normalizeMemoryProfile(record.profile),
		facts: readArray(record.facts, normalizeMemoryFact)
	};
}

function normalizeMemoryProfile(document: unknown): MemoryProfile {
	const record = readRecord(document) ?? {};
	const builtAt = readTimestamp(record.builtAt);
	return {
		identityLines: readStringArray(record.identityLines),
		currentLines: readStringArray(record.currentLines),
		...(builtAt ? { builtAt } : {})
	};
}

function normalizeMemoryFact(document: unknown): MemoryFact | undefined {
	const record = readRecord(document);
	if (!record) return undefined;
	const factID = readString(record.factID);
	const content = readString(record.content);
	const validFrom = readTimestamp(record.validFrom);
	const kind = readMemberOf(record.kind, memoryFactKinds);
	if (!factID || !content || !validFrom || !kind) return undefined;
	const validUntil = readTimestamp(record.validUntil);
	const lastRecalledAt = readTimestamp(record.lastRecalledAt);
	return {
		factID,
		episodeID: readString(record.episodeID) ?? '',
		ownerPersonID: readString(record.ownerPersonID) ?? '',
		circleIDs: readStringArray(record.circleIDs),
		kind,
		content,
		validFrom,
		...(validUntil ? { validUntil } : {}),
		reinforcementCount: readCount(record.reinforcementCount),
		...(lastRecalledAt ? { lastRecalledAt } : {})
	};
}

async function askTheDevice(): Promise<unknown> {
	const response = await adminApiFetch(`/memory/api/facts?limit=${defaultLimit}`);
	if (!response.ok) {
		throw new Error(`Memory facts request returned ${response.status}`);
	}
	return response.json();
}

async function askTheCompanyApp(): Promise<unknown> {
	const answer = await callCompanyApp({ capability: 'person.memory.facts', body: { limit: defaultLimit } });
	if (answer.status >= 400) throw new Error(`Memory facts request returned ${answer.status}`);
	return answer.body;
}

async function changeMemory(change: MemoryChange): Promise<void> {
	if (isSupabaseConfigured()) {
		const answer = await callCompanyApp({ capability: change.capability, body: change.body });
		if (answer.status >= 400) throw new Error(`Memory forget request returned ${answer.status}`);
		return;
	}
	const response = await adminApiFetch(change.path, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(change.body)
	});
	if (!response.ok) {
		throw new Error(`Memory forget request returned ${response.status}`);
	}
}

function readMemberOf<T extends string>(value: unknown, members: readonly T[]): T | undefined {
	return members.find((member) => member === value);
}

function readArray<T>(value: unknown, normalizeItem: (document: unknown) => T | undefined): T[] {
	if (!Array.isArray(value)) return [];
	return value.flatMap((item) => {
		const normalized = normalizeItem(item);
		return normalized ? [normalized] : [];
	});
}

function readRecord(document: unknown): Record<string, unknown> | undefined {
	return isRecord(document) ? document : undefined;
}

function isRecord(document: unknown): document is Record<string, unknown> {
	return typeof document === 'object' && document !== null && !Array.isArray(document);
}

function readString(value: unknown): string | undefined {
	return typeof value === 'string' && value.trim() ? value : undefined;
}

function readTimestamp(value: unknown): string | undefined {
	const text = readString(value);
	if (!text || text.startsWith('0001-01-01')) return undefined;
	return Number.isNaN(Date.parse(text)) ? undefined : text;
}

function readStringArray(value: unknown): string[] {
	if (!Array.isArray(value)) return [];
	return value.flatMap((item) => {
		const text = readString(item);
		return text ? [text.trim()] : [];
	});
}

function readCount(value: unknown): number {
	return typeof value === 'number' && Number.isFinite(value) && value > 0 ? Math.floor(value) : 0;
}
