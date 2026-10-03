import { z } from 'zod';
import { adminApiFetch } from '$lib/admin-api';
import { callCompanyApp } from '$lib/host-bridge';
import { invokeTool } from '$lib/public-api-call';
import { isSupabaseConfigured } from '$lib/supabase';
import { circleListResultSchema } from '$lib/data-room/schemas';
import type { Circle } from '$lib/data-room/model';
import type { MemoryChange } from './memory-change';

export const memoryScopeTypes = ['person', 'circle', 'workspace'] as const;
export type MemoryScopeType = (typeof memoryScopeTypes)[number];

const timestamp = z.iso.datetime({ offset: true });

export const memoryFactSchema = z.object({
	factID: z.string().min(1),
	originID: z.string(),
	scopeType: z.enum(memoryScopeTypes),
	scopeID: z.string().optional(),
	isStatic: z.boolean(),
	content: z.string().min(1),
	occurredAt: timestamp.optional(),
	occurredUntil: timestamp.optional(),
	validUntil: timestamp.optional(),
	importance: z.number().int(),
	storageStrength: z.number(),
	createdAt: timestamp,
	lastRecalledAt: timestamp.optional(),
	coldSince: timestamp.optional(),
	triggerPhrases: z.array(z.string())
});

export const memoryIndexSchema = z.object({
	embeddingModel: z.string(),
	current: z.number().int(),
	stale: z.number().int()
});

export const memoryFactsResponseSchema = z.object({
	personID: z.string(),
	index: memoryIndexSchema,
	facts: z.array(memoryFactSchema)
});

export type MemoryFact = z.infer<typeof memoryFactSchema>;
export type MemoryFactsResponse = z.infer<typeof memoryFactsResponseSchema>;

const defaultLimit = 200;

export async function fetchMemoryFacts(): Promise<MemoryFactsResponse> {
	const document = isSupabaseConfigured() ? await askTheCompanyApp() : await askTheDevice();
	return memoryFactsResponseSchema.parse(document);
}

export async function fetchCircles(): Promise<Circle[]> {
	if (!isSupabaseConfigured()) return [];
	return circleListResultSchema.parse(await invokeTool('circle_list', {})).circles;
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
