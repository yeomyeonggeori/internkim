import { adminApiFetch } from '$lib/admin-api';
import { callCompanyApp } from '$lib/host-bridge';
import { isSupabaseConfigured } from '$lib/supabase';
import type { MemoryChange } from './memory-change';

export type MemoryGraphHealth = {
	configured?: boolean;
	reachable?: boolean;
	hasSearchFailure?: boolean;
	hasIngestionFailure?: boolean;
	hasGraphFailure?: boolean;
};

export type MemoryGraphNamespace = {
	namespaceID: string;
	scopeType: string;
	scopePersonID?: string;
	scopeConversationID?: string;
	scopeCircleID?: string;
	episodeCount?: number;
};

export type MemoryGraphFact = {
	factID: string;
	scopeType: string;
	namespaceID: string;
	content: string;
	score?: number | null;
	sourceEpisodeID?: string;
	sourceKind?: string;
	validAt?: string;
};

export type MemoryGraphNode = {
	nodeID: string;
	label: string;
	kind: string;
	scopeType?: string;
	status?: string;
};

export type MemoryGraphEdge = {
	sourceID: string;
	targetID: string;
	weight?: number;
};

export type MemoryGraphEpisode = {
	episodeID: string;
	platform?: string;
	prompt?: string;
	namespaceIDs?: string[];
	ingestionStatus?: string;
	ingestionError?: string;
	occurredAt?: string;
};

export type MemoryGraphResponse = {
	health?: MemoryGraphHealth;
	namespaces?: MemoryGraphNamespace[];
	episodes?: MemoryGraphEpisode[];
	facts?: MemoryGraphFact[];
	nodes?: MemoryGraphNode[];
	edges?: MemoryGraphEdge[];
};

export async function fetchMemoryGraph(memoryGraphQuery: string): Promise<MemoryGraphResponse> {
	const asked = memoryGraphQuery.trim();
	if (isSupabaseConfigured()) return normalizeMemoryGraphResponse(await askTheCompanyApp(asked));

	const urlParameters = new URLSearchParams({ limit: '120' });
	if (asked) urlParameters.set('query', asked);
	const response = await adminApiFetch(`/memory/api/graph?${urlParameters.toString()}`);
	if (!response.ok) {
		throw new Error(`Memory graph request returned ${response.status}`);
	}
	const document: unknown = await response.json();
	return normalizeMemoryGraphResponse(document);
}

export function episodeDeleteRequest(episodeID: string, namespaceIDs: string[]): MemoryChange {
	return {
		capability: 'person.memory.episode_delete',
		path: '/memory/api/episodes/delete',
		body: { episodeID, namespaceIDs }
	};
}

export function pinnedUpdateRequest(content: string): MemoryChange {
	return { capability: 'person.memory.pinned_update', path: '/memory/api/pinned/update', body: { content } };
}

export function pinnedDeleteRequest(): MemoryChange {
	return { capability: 'person.memory.pinned_delete', path: '/memory/api/pinned/delete', body: {} };
}

export async function deleteMemoryEpisode(episodeID: string, namespaceIDs: string[]): Promise<void> {
	await changeMemory(episodeDeleteRequest(episodeID, namespaceIDs));
}

export async function savePinnedMemory(content: string): Promise<void> {
	await changeMemory(pinnedUpdateRequest(content));
}

export async function deletePinnedMemory(): Promise<void> {
	await changeMemory(pinnedDeleteRequest());
}

export function normalizeMemoryGraphResponse(document: unknown): MemoryGraphResponse {
	const record = readRecord(document);
	if (!record) return {};

	const health = normalizeMemoryGraphHealth(record.health);
	const namespaces = readArray(record.namespaces, normalizeMemoryGraphNamespace);
	const episodes = readArray(record.episodes, normalizeMemoryGraphEpisode);
	const facts = readArray(record.facts, normalizeMemoryGraphFact);
	const nodes = readArray(record.nodes, normalizeMemoryGraphNode);
	const edges = readArray(record.edges, normalizeMemoryGraphEdge);

	return {
		...(health ? { health } : {}),
		...(namespaces ? { namespaces } : {}),
		...(episodes ? { episodes } : {}),
		...(facts ? { facts } : {}),
		...(nodes ? { nodes } : {}),
		...(edges ? { edges } : {})
	};
}

async function changeMemory(change: MemoryChange): Promise<void> {
	if (isSupabaseConfigured()) {
		const answer = await callCompanyApp({ capability: change.capability, body: change.body });
		if (answer.status >= 400) throw new Error(`Memory graph request returned ${answer.status}`);
		return;
	}
	const response = await adminApiFetch(change.path, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(change.body)
	});
	if (!response.ok) {
		throw new Error(`Memory graph request returned ${response.status}`);
	}
}

function normalizeMemoryGraphHealth(document: unknown): MemoryGraphHealth | undefined {
	const record = readRecord(document);
	if (!record) return undefined;

	const health = {
		...(typeof record.configured === 'boolean' ? { configured: record.configured } : {}),
		...(typeof record.reachable === 'boolean' ? { reachable: record.reachable } : {}),
		...(hasFailure(record.lastSearchError) ? { hasSearchFailure: true } : {}),
		...(hasFailure(record.lastIngestionError) ? { hasIngestionFailure: true } : {}),
		...(hasFailure(record.error) ? { hasGraphFailure: true } : {})
	};
	return Object.keys(health).length > 0 ? health : undefined;
}

function hasFailure(value: unknown): boolean {
	if (typeof value === 'boolean') return value;
	return typeof value === 'string' && value.trim().length > 0;
}

function normalizeMemoryGraphNamespace(document: unknown): MemoryGraphNamespace | undefined {
	const record = readRecord(document);
	if (!record) return undefined;

	const namespaceID = readString(record.namespaceID);
	const scopeType = readString(record.scopeType);
	if (!namespaceID || !scopeType) return undefined;

	const scopePersonID = readString(record.scopePersonID);
	const scopeConversationID = readString(record.scopeConversationID);
	const scopeCircleID = readString(record.scopeCircleID);
	const episodeCount = readNumber(record.episodeCount);

	return {
		namespaceID,
		scopeType,
		...(scopePersonID ? { scopePersonID } : {}),
		...(scopeConversationID ? { scopeConversationID } : {}),
		...(scopeCircleID ? { scopeCircleID } : {}),
		...(typeof episodeCount === 'number' ? { episodeCount } : {})
	};
}

function normalizeMemoryGraphFact(document: unknown): MemoryGraphFact | undefined {
	const record = readRecord(document);
	if (!record) return undefined;

	const factID = readString(record.factID);
	const scopeType = readString(record.scopeType);
	const namespaceID = readString(record.namespaceID);
	const content = readString(record.content);
	if (!factID || !scopeType || !namespaceID || !content) return undefined;

	const score = readNullableNumber(record.score);
	const sourceEpisodeID = readString(record.sourceEpisodeID);
	const sourceKind = readString(record.sourceKind);
	const validAt = readTimestamp(record.validAt);

	return {
		factID,
		scopeType,
		namespaceID,
		content,
		...(typeof score === 'number' || score === null ? { score } : {}),
		...(sourceEpisodeID ? { sourceEpisodeID } : {}),
		...(sourceKind ? { sourceKind } : {}),
		...(validAt ? { validAt } : {})
	};
}

function normalizeMemoryGraphEpisode(document: unknown): MemoryGraphEpisode | undefined {
	const record = readRecord(document);
	if (!record) return undefined;

	const episodeID = readString(record.episodeID);
	if (!episodeID) return undefined;

	const platform = readString(record.platform);
	const prompt = readString(record.prompt);
	const namespaceIDs = readStringArray(record.namespaceIDs);
	const ingestionStatus = readString(record.ingestionStatus);
	const ingestionError = readString(record.ingestionError);
	const occurredAt = readString(record.occurredAt);

	return {
		episodeID,
		...(platform ? { platform } : {}),
		...(prompt ? { prompt } : {}),
		...(namespaceIDs ? { namespaceIDs } : {}),
		...(ingestionStatus ? { ingestionStatus } : {}),
		...(ingestionError ? { ingestionError } : {}),
		...(occurredAt ? { occurredAt } : {})
	};
}

function normalizeMemoryGraphNode(document: unknown): MemoryGraphNode | undefined {
	const record = readRecord(document);
	if (!record) return undefined;

	const nodeID = readString(record.nodeID);
	const label = readString(record.label);
	const kind = readString(record.kind);
	if (!nodeID || !label || !kind) return undefined;

	const scopeType = readString(record.scopeType);
	const status = readString(record.status);

	return {
		nodeID,
		label,
		kind,
		...(scopeType ? { scopeType } : {}),
		...(status ? { status } : {})
	};
}

function normalizeMemoryGraphEdge(document: unknown): MemoryGraphEdge | undefined {
	const record = readRecord(document);
	if (!record) return undefined;

	const sourceID = readString(record.sourceID);
	const targetID = readString(record.targetID);
	if (!sourceID || !targetID) return undefined;

	const weight = readNumber(record.weight);

	return {
		sourceID,
		targetID,
		...(typeof weight === 'number' ? { weight } : {})
	};
}

function readArray<T>(value: unknown, normalizeItem: (document: unknown) => T | undefined): T[] | undefined {
	if (!Array.isArray(value)) return undefined;
	return value.flatMap((item) => {
		const normalizedItem = normalizeItem(item);
		return normalizedItem ? [normalizedItem] : [];
	});
}

function readRecord(document: unknown): Record<string, unknown> | undefined {
	return isRecord(document) ? document : undefined;
}

function isRecord(document: unknown): document is Record<string, unknown> {
	return Boolean(document) && typeof document === 'object' && !Array.isArray(document);
}

function readString(value: unknown): string | undefined {
	return typeof value === 'string' ? value : undefined;
}

function readTimestamp(value: unknown): string | undefined {
	const timestamp = readString(value);
	if (!timestamp || timestamp.startsWith('0001-01-01')) return undefined;
	return timestamp;
}

function readStringArray(value: unknown): string[] | undefined {
	if (!Array.isArray(value)) return undefined;
	const values = value.flatMap((item) => {
		if (typeof item !== 'string') return [];
		const trimmedItem = item.trim();
		return trimmedItem ? [trimmedItem] : [];
	});
	return values.length > 0 ? values : undefined;
}

function readNumber(value: unknown): number | undefined {
	return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

function readNullableNumber(value: unknown): number | null | undefined {
	if (value === null) return null;
	return readNumber(value);
}

async function askTheCompanyApp(memoryGraphQuery: string): Promise<unknown> {
	const answer = await callCompanyApp({
		capability: 'person.memory.graph',
		body: memoryGraphQuery ? { limit: 120, query: memoryGraphQuery } : { limit: 120 }
	});
	if (answer.status >= 400) throw new Error(`Memory graph request returned ${answer.status}`);
	return answer.body;
}
