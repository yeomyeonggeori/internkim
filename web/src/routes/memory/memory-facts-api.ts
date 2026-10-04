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

export const memoryLayerSchema = z.object({
	scopeType: z.enum(memoryScopeTypes),
	scopeID: z.string().optional()
});

export const memoryFactsResponseSchema = z.object({
	personID: z.string(),
	layers: z.array(memoryLayerSchema),
	index: memoryIndexSchema,
	facts: z.array(memoryFactSchema)
});

export const memoryRecalledSchema = z.object({
	factID: z.string().min(1),
	scopeType: z.enum(memoryScopeTypes),
	scopeID: z.string().optional(),
	content: z.string().min(1)
});

export const memoryRecallResponseSchema = z.object({
	facts: z.array(memoryRecalledSchema),
	degradedReason: z.string().optional()
});

export type MemoryFact = z.infer<typeof memoryFactSchema>;
export type MemoryLayer = z.infer<typeof memoryLayerSchema>;
export type MemoryFactsResponse = z.infer<typeof memoryFactsResponseSchema>;
export type MemoryRecalled = z.infer<typeof memoryRecalledSchema>;
export type MemoryRecallResponse = z.infer<typeof memoryRecallResponseSchema>;

const defaultLimit = 200;

export async function fetchMemoryFacts(): Promise<MemoryFactsResponse> {
	const document = isSupabaseConfigured() ? await askTheCompanyApp() : await askTheDevice();
	return memoryFactsResponseSchema.parse(document);
}

export async function fetchMemoryRecall(query: string): Promise<MemoryRecallResponse> {
	const document = isSupabaseConfigured() ? await askTheCompanyAppToRecall(query) : await askTheDeviceToRecall(query);
	return memoryRecallResponseSchema.parse(document);
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

async function askTheDeviceToRecall(query: string): Promise<unknown> {
	const response = await adminApiFetch(`/memory/api/recall?${new URLSearchParams({ query })}`);
	if (!response.ok) throw new Error(`Memory recall request returned ${response.status}`);
	return response.json();
}

async function askTheCompanyAppToRecall(query: string): Promise<unknown> {
	const answer = await callCompanyApp({ capability: 'person.memory.recall', body: { query } });
	if (answer.status >= 400) throw new Error(`Memory recall request returned ${answer.status}`);
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
