import { adminApiFetch } from '$lib/admin-api';
import { callCompanyApp } from '$lib/host-bridge';
import { isSupabaseConfigured } from '$lib/supabase';

export type SoulRevision = {
	version: number;
	document: unknown;
	reason: string;
	createdAt: string;
	origin: string;
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

function readRevision(value: unknown): SoulRevision | undefined {
	const record = readRecord(value);
	if (!record || typeof record.version !== 'number' || !record.document) return undefined;
	return { version: record.version, document: record.document, reason: typeof record.reason === 'string' ? record.reason : '', createdAt: typeof record.createdAt === 'string' ? record.createdAt : '', origin: typeof record.origin === 'string' ? record.origin : '' };
}

function readArray(value: unknown): SoulRevision[] { return Array.isArray(value) ? value.flatMap((entry) => { const revision = readRevision(entry); return revision ? [revision] : []; }) : []; }
function readRecord(value: unknown): Record<string, unknown> | undefined { return typeof value === 'object' && value !== null && !Array.isArray(value) ? value as Record<string, unknown> : undefined; }
