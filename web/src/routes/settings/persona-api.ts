import { adminApiFetch } from '$lib/admin-api';
import { callCompanyApp } from '$lib/host-bridge';
import { isSupabaseConfigured } from '$lib/supabase';
import type { AgentSoul, AgentUser } from '../admin/admin-types';

export type AgentIdentity = {
	schemaVersion: 1;
	names: string[];
	handle?: string;
	role?: string;
	creature?: string;
	emoji?: string;
	introduction?: string;
};

export function readAgentIdentity(value: unknown): AgentIdentity {
	if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('identity response is invalid');
	const record = value as Record<string, unknown>;
	if (record.schemaVersion !== 1 || !Array.isArray(record.names) || record.names.length === 0 || record.names.some((name) => typeof name !== 'string' || !name.trim())) throw new Error('identity response is invalid');
	return { schemaVersion: 1, names: [...record.names], ...readOptionalIdentityFields(record) };
}

function readOptionalIdentityFields(record: Record<string, unknown>): Omit<AgentIdentity, 'schemaVersion' | 'names'> {
	const fields = ['handle', 'role', 'creature', 'emoji', 'introduction'] as const;
	return Object.fromEntries(fields.filter((field) => record[field] === undefined || typeof record[field] === 'string').map((field) => [field, record[field]])) as Omit<AgentIdentity, 'schemaVersion' | 'names'>;
}

export async function fetchAgentIdentity(): Promise<AgentIdentity> {
	if (isSupabaseConfigured()) { const answer = await callCompanyApp({ capability: 'person.persona.identity' }); if (answer.status >= 400) throw new Error(`persona identity request returned ${answer.status}`); return readAgentIdentity(answer.body); }
	return readAgentIdentity(await (await adminApiFetch('/persona/api/identity')).json());
}

export async function updateAgentIdentity(identity: AgentIdentity): Promise<AgentIdentity> {
	if (isSupabaseConfigured()) { const answer = await callCompanyApp({ capability: 'person.persona.identity.update', body: identity }); if (answer.status >= 400) throw new Error(`persona identity save returned ${answer.status}`); return readAgentIdentity(answer.body); }
	return readAgentIdentity(await (await adminApiFetch('/persona/api/identity', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(identity) })).json());
}

export function fetchMyAgentDocument(): Promise<AgentUser> {
	return readPersona<AgentUser>('user', 'person.persona.user');
}

export function updateMyAgentDocument(user: AgentUser): Promise<AgentUser> {
	return writePersona<AgentUser>('user', 'person.persona.user.update', user);
}

export function fetchAgentSoul(): Promise<AgentSoul> {
	return readPersona<AgentSoul>('soul', 'person.persona.soul');
}

export function updateAgentSoul(soul: AgentSoul): Promise<AgentSoul> {
	return writePersona<AgentSoul>('soul', 'person.persona.soul.update', soul);
}

async function readPersona<T>(name: string, capability: string): Promise<T> {
	if (isSupabaseConfigured()) {
		const answer = await callCompanyApp({ capability });
		if (answer.status >= 400) throw new Error(`persona ${name} request returned ${answer.status}`);
		return answer.body as T;
	}
	const response = await fetch(`/persona/api/${name}`, { credentials: 'include' });
	if (!response.ok) throw new Error(`persona ${name} request returned ${response.status}`);
	return (await response.json()) as T;
}

async function writePersona<T>(name: string, capability: string, document: T): Promise<T> {
	if (isSupabaseConfigured()) {
		const answer = await callCompanyApp({ capability, body: document as Record<string, unknown> });
		if (answer.status >= 400) throw new Error(personaErrorText(answer.body) ?? `persona ${name} save returned ${answer.status}`);
		return answer.body as T;
	}
	const response = await fetch(`/persona/api/${name}`, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(document)
	});
	if (!response.ok) throw new Error((await response.text()).trim() || `persona ${name} save returned ${response.status}`);
	return (await response.json()) as T;
}

function personaErrorText(body: unknown): string | undefined {
	if (typeof body === 'string' && body.trim()) return body.trim();
	if (body && typeof body === 'object' && typeof (body as { error?: unknown }).error === 'string') {
		return (body as { error: string }).error;
	}
	return undefined;
}
