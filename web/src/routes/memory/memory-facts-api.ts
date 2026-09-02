import { callCompanyApp } from '$lib/host-bridge';
import { isSupabaseConfigured } from '$lib/supabase';

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
	if (isSupabaseConfigured()) return normalizeMemoryFactsResponse(await askTheCompanyApp());
	const response = await fetch(`/memory/api/facts?limit=${defaultLimit}`, { credentials: 'include' });
	if (!response.ok) {
		throw new Error(`Memory facts request returned ${response.status}`);
	}
	return normalizeMemoryFactsResponse(await response.json());
}

export async function forgetMemoryFact(factID: string, reason: string): Promise<void> {
	const body = { factIDs: [factID], reason };
	if (isSupabaseConfigured()) {
		const answer = await callCompanyApp({ capability: 'person.memory.facts.forget', body });
		if (answer.status >= 400) throw new Error(`Memory forget request returned ${answer.status}`);
		return;
	}
	const response = await fetch('/memory/api/facts/forget', {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
	if (!response.ok) {
		throw new Error(`Memory forget request returned ${response.status}`);
	}
}

export function normalizeMemoryFactsResponse(document: unknown): MemoryFactsResponse {
	const record = readRecord(document) ?? {};
	return {
		personID: readString(record.personID) ?? '',
		...(readString(record.embeddingModel) ? { embeddingModel: readString(record.embeddingModel) } : {}),
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
	return typeof document === 'object' && document !== null && !Array.isArray(document)
		? (document as Record<string, unknown>)
		: undefined;
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
	return typeof value === 'number' && Number.isFinite(value) && value > 0 ? Math.floor(value) : 1;
}

async function askTheCompanyApp(): Promise<unknown> {
	const answer = await callCompanyApp({ capability: 'person.memory.facts', body: { limit: defaultLimit } });
	if (answer.status >= 400) throw new Error(`Memory facts request returned ${answer.status}`);
	return answer.body;
}
