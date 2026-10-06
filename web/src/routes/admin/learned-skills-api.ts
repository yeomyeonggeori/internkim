import { adminApiFetch } from '$lib/admin-api';
import { callCompanyApp } from '$lib/host-bridge';
import { isSupabaseConfigured } from '$lib/supabase';
import { SkillReadError } from './skill-read-error';

export type LearnedSkill = {
	id: string;
	version: number;
	audience: string;
	description: string;
	instruction: string;
	evidenceIDs: string[];
	reason: string;
	verification: string;
	status: string;
	protected: boolean;
	createdAt: string;
	updatedAt: string;
	retiredAt?: string;
};

export type LearnedSkillInventory = {
	skills: LearnedSkill[];
	invalidSkillCount: number;
	activeCount?: number;
	activeLimit?: number;
	enabled?: boolean;
};

export async function fetchLearnedSkills(): Promise<LearnedSkillInventory> {
	const document = isSupabaseConfigured() ? await askCompanyApp() : await askDevice();
	return readLearnedSkillInventory(document);
}

export async function updateLearnedSkill(id: string, action: 'protect' | 'retire' | 'restore', protectedValue = action === 'protect'): Promise<void> {
	const path = `/agent-learning/api/skills/${action}`;
	const response = isSupabaseConfigured()
		? await callCompanyAction(id, action, protectedValue)
		: await adminApiFetch(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ id, protected: protectedValue }) });
	if (response.status >= 400) throw new Error(`Learned skill ${action} returned ${response.status}`);
}

export async function fetchLearningSettings(): Promise<{ enabled: boolean; activeLimit: number }> {
	if (isSupabaseConfigured()) {
		const response = await callCompanyApp({ capability: 'person.agent_learning.settings.get' });
		if (response.status >= 400) throw new SkillReadError(`Learning settings request returned ${response.status}`, response.status);
		return readSettings(response.body);
	}
	const response = await adminApiFetch('/agent-learning/api/settings');
	if (!response.ok) throw new SkillReadError(`Learning settings request returned ${response.status}`, response.status);
	return readSettings(await response.json());
}

export async function updateLearningSettings(settings: { enabled: boolean; activeLimit: number }): Promise<void> {
	const response = isSupabaseConfigured() ? await callCompanyApp({ capability: 'person.agent_learning.settings.update', body: settings }) : await adminApiFetch('/agent-learning/api/settings', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(settings) });
	if (response.status >= 400) throw new Error(`Learning settings update returned ${response.status}`);
}

export async function fetchLearnedSkillHistory(id: string): Promise<LearnedSkill[]> {
	if (isSupabaseConfigured()) {
		const response = await callCompanyApp({ capability: 'person.agent_learning.skills.get', body: { id, includeHistory: true } });
		if (response.status >= 400) throw new SkillReadError(`Learned skill history request returned ${response.status}`, response.status);
		return readArray(readRecord(response.body)?.skills, readLearnedSkill);
	}
	const response = await adminApiFetch(`/agent-learning/api/skills/${encodeURIComponent(id)}?includeHistory=true`);
	if (!response.ok) throw new SkillReadError(`Learned skill history request returned ${response.status}`, response.status);
	return readArray(readRecord(await response.json())?.skills, readLearnedSkill);
}

async function askDevice(): Promise<unknown> {
	const response = await adminApiFetch('/agent-learning/api/skills?includeRetired=true');
	if (!response.ok) throw new SkillReadError(`Learned skills request returned ${response.status}`, response.status);
	return response.json();
}

async function askCompanyApp(): Promise<unknown> {
	const response = await callCompanyApp({ capability: 'person.agent_learning.skills.list', body: { includeRetired: true } });
	if (response.status >= 400) throw new SkillReadError(`Learned skills request returned ${response.status}`, response.status);
	return response.body;
}

async function callCompanyAction(id: string, action: 'protect' | 'retire' | 'restore', protectedValue: boolean) {
		return callCompanyApp({ capability: 'person.agent_learning.skills.action', body: { id, action, protected: protectedValue } });
}

export function readLearnedSkillInventory(document: unknown): LearnedSkillInventory {
	const record = readRecord(document);
	if (!record || !Array.isArray(record.skills)) throw new Error('Learned skills response is malformed');
	const skills = readArray(record?.skills, readLearnedSkill);
	return {
		skills,
		invalidSkillCount: record.skills.length - skills.length,
		activeCount: readNumber(record?.activeCount) ?? skills.filter((skill) => skill.status === 'active').length,
		activeLimit: readNumber(record?.activeLimit) ?? readNumber(readRecord(record?.settings)?.activeLimit),
		enabled: readRecord(record?.settings)?.enabled === true
	};
}

function readLearnedSkill(value: unknown): LearnedSkill | undefined {
	const record = readRecord(value);
	const id = readString(record?.id);
	if (!id || !isPositiveInteger(record?.version)) return undefined;
	if (!isNonEmptyString(record?.audience) || !isString(record?.description) || !isString(record?.instruction)) return undefined;
	if (!Array.isArray(record?.evidenceIDs) || !record.evidenceIDs.every((entry) => typeof entry === 'string' && entry.trim() !== '')) return undefined;
	if (!isString(record?.verification) || !isString(record?.status) || typeof record?.protected !== 'boolean') return undefined;
	if (!isString(record?.createdAt) || !isString(record?.updatedAt)) return undefined;
	return {
		id,
		version: record.version,
		audience: record.audience,
		description: record.description,
		instruction: record.instruction,
		evidenceIDs: readStringArray(record?.evidenceIDs),
		status: record.status,
		reason: readString(record?.reason) ?? '',
		verification: record.verification,
		protected: record.protected,
		createdAt: record.createdAt,
		updatedAt: record.updatedAt,
		retiredAt: readString(record?.retiredAt)
	};
}

function readSettings(value: unknown): { enabled: boolean; activeLimit: number } {
	const record = readRecord(value);
	return { enabled: record?.enabled === true, activeLimit: readNumber(record?.activeLimit) ?? 20 };
}

function readArray<T>(value: unknown, read: (value: unknown) => T | undefined): T[] {
	if (!Array.isArray(value)) return [];
	return value.flatMap((entry) => {
		const result = read(entry);
		return result ? [result] : [];
	});
}

function readStringArray(value: unknown): string[] {
	return Array.isArray(value) ? value.filter((entry): entry is string => typeof entry === 'string' && entry.trim() !== '') : [];
}

function readString(value: unknown): string | undefined { return typeof value === 'string' ? value : undefined; }
function isString(value: unknown): value is string { return typeof value === 'string'; }
function isNonEmptyString(value: unknown): value is string { return isString(value) && value.trim() !== ''; }
function isPositiveInteger(value: unknown): value is number { return typeof value === 'number' && Number.isInteger(value) && value > 0; }
function readNumber(value: unknown): number | undefined { return typeof value === 'number' && Number.isFinite(value) ? value : undefined; }
function readRecord(value: unknown): Record<string, unknown> | undefined {
	return typeof value === 'object' && value !== null && !Array.isArray(value) ? value as Record<string, unknown> : undefined;
}
