import { adminApiFetch } from '$lib/admin-api';
import { callCompanyApp } from '$lib/host-bridge';
import { isSupabaseConfigured } from '$lib/supabase';
import { SkillReadError } from './skill-read-error';

export type LoadedSkill = {
	name: string;
	description: string;
	path: string;
	toolReferences: string[];
};

export type UnavailableSkill = {
	name: string;
	description: string;
	path: string;
	missingEnvironmentVariables: string[];
	missingToolNames: string[];
};

export type SkillInventory = {
	skills: LoadedSkill[];
	unavailableSkills: UnavailableSkill[];
};

export type SkillRoot = {
	path: string;
	skills: LoadedSkill[];
};

export async function fetchSkillInventory(): Promise<SkillInventory> {
	return readSkillInventory(isSupabaseConfigured() ? await askTheCompanyApp() : await askTheDevice());
}

export function readSkillInventory(document: unknown): SkillInventory {
	const record = readRecord(document);
	if (!record) return { skills: [], unavailableSkills: [] };
	return {
		skills: readArray(record.skills, readLoadedSkill),
		unavailableSkills: readArray(record.unavailableSkills, readUnavailableSkill)
	};
}

export function skillRootOf(path: string): string {
	const trimmedPath = path.replace(/\/+$/, '');
	const lastSeparator = trimmedPath.lastIndexOf('/');
	return lastSeparator > 0 ? trimmedPath.slice(0, lastSeparator) : trimmedPath;
}

export function skillRootsOf(skills: LoadedSkill[]): SkillRoot[] {
	const rootPaths: string[] = [];
	const skillsByRoot = new Map<string, LoadedSkill[]>();
	for (const skill of skills) {
		const rootPath = skillRootOf(skill.path);
		const alreadyGrouped = skillsByRoot.get(rootPath);
		if (alreadyGrouped) {
			alreadyGrouped.push(skill);
			continue;
		}
		rootPaths.push(rootPath);
		skillsByRoot.set(rootPath, [skill]);
	}
	return rootPaths.map((path) => ({ path, skills: skillsByRoot.get(path) ?? [] }));
}

function readLoadedSkill(document: unknown): LoadedSkill | undefined {
	const record = readRecord(document);
	const name = readString(record?.name);
	if (!record || !name) return undefined;
	return {
		name,
		description: readString(record.description) ?? '',
		path: readString(record.path) ?? '',
		toolReferences: readStringArray(record.toolReferences)
	};
}

function readUnavailableSkill(document: unknown): UnavailableSkill | undefined {
	const record = readRecord(document);
	const name = readString(record?.name);
	if (!record || !name) return undefined;
	return {
		name,
		description: readString(record.description) ?? '',
		path: readString(record.path) ?? '',
		missingEnvironmentVariables: readStringArray(record.missingEnvironmentVariables),
		missingToolNames: readStringArray(record.missingToolNames)
	};
}

function readArray<Entry>(value: unknown, readEntry: (document: unknown) => Entry | undefined): Entry[] {
	if (!Array.isArray(value)) return [];
	return value.flatMap((document) => {
		const entry = readEntry(document);
		return entry ? [entry] : [];
	});
}

function readStringArray(value: unknown): string[] {
	if (!Array.isArray(value)) return [];
	return value.flatMap((entry) => {
		const text = readString(entry)?.trim();
		return text ? [text] : [];
	});
}

function readString(value: unknown): string | undefined {
	return typeof value === 'string' ? value : undefined;
}

function readRecord(value: unknown): Record<string, unknown> | undefined {
	if (typeof value !== 'object' || value === null || Array.isArray(value)) return undefined;
	return value as Record<string, unknown>;
}

async function askTheDevice(): Promise<unknown> {
	const response = await adminApiFetch('/skills/api');
	if (!response.ok) throw new SkillReadError(`Skill inventory request returned ${response.status}`, response.status);
	return response.json();
}

async function askTheCompanyApp(): Promise<unknown> {
	const answer = await callCompanyApp({ capability: 'person.skills.list' });
	if (answer.status >= 400) throw new SkillReadError(`Skill inventory request returned ${answer.status}`, answer.status);
	return answer.body;
}
