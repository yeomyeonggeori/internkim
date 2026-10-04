import { describe, expect, test } from 'bun:test';
import { createClient } from '@supabase/supabase-js';
import { organizationsOfCompany, contactsOfCompany, opportunitiesOfCompany, activityCountByOpportunity } from '$lib/server/public-api/record/crm';

function record() {
	const queries: URL[] = [];
	const request: typeof fetch = Object.assign(async (input: Parameters<typeof fetch>[0]) => {
		const url = new URL(typeof input === 'string' ? input : input instanceof URL ? input.href : input.url);
		queries.push(url);
		const from = Number(url.searchParams.get('offset') ?? 0);
		const limit = Number(url.searchParams.get('limit') ?? 1000);
		return Response.json(Array.from({ length: Math.max(0, Math.min(limit, 1501 - from)) }, (_, index) => ({
			id: String(from + index), name: 'Same name', stage_position: 0,
			opportunity_id: 'deal', archived_at: null
		})));
	}, { preconnect() {} });
	return {
		queries,
		caller: createClient('https://record.example.com', 'test-key', {
			auth: { persistSession: false, autoRefreshToken: false }, global: { fetch: request }
		})
	};
}

describe('CRM record pagination', () => {
	for (const read of [organizationsOfCompany, contactsOfCompany, opportunitiesOfCompany]) {
		test(`${read.name} filters archived records before stable pagination`, async () => {
			const { caller, queries } = record();
			const found = await read(caller);
			expect(found).toHaveLength(1501);
			expect(new Set(found.map(row => row.id)).size).toBe(1501);
			expect(queries).toHaveLength(4);
			expect(queries.every(url => url.searchParams.get('archived_at') === 'is.null')).toBe(true);
			expect(queries.every(url => url.searchParams.get('order')?.endsWith(',id.asc'))).toBe(true);
		});
	}

	test('explicit archived reads retain archived records', async () => {
		const { caller, queries } = record();
		await organizationsOfCompany(caller, true);
		expect(queries.every(url => !url.searchParams.has('archived_at'))).toBe(true);
	});

	test('activity counts do not stop at the server row cap', async () => {
		const { caller, queries } = record();
		expect((await activityCountByOpportunity(caller)).get('deal')).toBe(1501);
		expect(queries).toHaveLength(4);
		expect(queries.every(url => url.searchParams.get('opportunity_id') === 'not.is.null')).toBe(true);
	});
});
