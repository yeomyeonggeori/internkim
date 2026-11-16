import { adminApiFetch } from '$lib/admin-api';
import { callCompanyApp } from '$lib/host-bridge';
import { isSupabaseConfigured } from '$lib/supabase';

export type SoulRevision = {
	version: number;
	document: unknown;
	reason: string;
	createdAt: string;
	origin: string;
	author: string;
};

export type SoulHistory = { current: SoulRevision; history: SoulRevision[] };

export async function fetchLearningSoul(): Promise<SoulHistory> {
	const [current, historyRecord] = await Promise.all([readCurrent(), readHistory()]);
	if (!current || !historyRecord) throw new Error('Learning soul response is incomplete');
	const history = readArray(historyRecord.history);
	if (history.length === 0) throw new Error('Learning soul response is incomplete');
	return { current, history };
}

async function readCurrent(): Promise<SoulRevision | undefined> {
	if (isSupabaseConfigured()) { const response = await callCompanyApp({ capability: 'person.agent_learning.soul.get' }); if (response.status >= 400) throw new Error(`Learning soul request returned ${response.status}`); return readRevision(response.body); }
	const response = await adminApiFetch('/agent-learning/api/soul');
	if (!response.ok) throw new Error(`Learning soul request returned ${response.status}`);
	return readRevision(await response.json());
}

async function readHistory(): Promise<Record<string, unknown> | undefined> {
	if (isSupabaseConfigured()) { const response = await callCompanyApp({ capability: 'person.agent_learning.soul.history' }); if (response.status >= 400) throw new Error(`Learning soul history returned ${response.status}`); return readRecord(response.body); }
	const response = await adminApiFetch('/agent-learning/api/soul/history');
	if (!response.ok) throw new Error(`Learning soul history returned ${response.status}`);
	return readRecord(await response.json());
}

export async function reviseLearningSoul(expectedVersion: number, document: unknown, reason: string): Promise<SoulRevision> {
	const body = { expectedVersion, document, reason };
	if (isSupabaseConfigured()) { const response = await callCompanyApp({ capability: 'person.agent_learning.soul.update', body }); if (response.status >= 400) throw new SoulRevisionError(response.status); return revisionOrThrow(response.body); }
	const response = await adminApiFetch('/agent-learning/api/soul', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
	if (!response.ok) throw new SoulRevisionError(response.status);
	return revisionOrThrow(await response.json());
}

export class SoulRevisionError extends Error {
	readonly status: number;
	constructor(status: number) {
		super(`Learning soul revision returned ${status}`);
		this.status = status;
	}
}

function revisionOrThrow(value: unknown): SoulRevision {
	const revision = readRevision(value);
	if (!revision) throw new Error('Learning soul revision response is incomplete');
	return revision;
}

function readRevision(value: unknown): SoulRevision | undefined {
	const record = readRecord(value);
	if (!record || typeof record.version !== 'number' || !record.document) return undefined;
	return { version: record.version, document: record.document, reason: typeof record.reason === 'string' ? record.reason : '', createdAt: typeof record.createdAt === 'string' ? record.createdAt : '', origin: typeof record.origin === 'string' ? record.origin : '', author: typeof record.author === 'string' ? record.author : '' };
}

function readArray(value: unknown): SoulRevision[] { return Array.isArray(value) ? value.flatMap((entry) => { const revision = readRevision(entry); return revision ? [revision] : []; }) : []; }
function readRecord(value: unknown): Record<string, unknown> | undefined { return typeof value === 'object' && value !== null && !Array.isArray(value) ? value as Record<string, unknown> : undefined; }
