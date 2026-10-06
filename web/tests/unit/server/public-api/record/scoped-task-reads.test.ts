import { describe, expect, test } from 'bun:test';
import { createClient } from '@supabase/supabase-js';
import { tasksOfCompany, savedTaskByID, taskFromHint, WriteNotReadBack, type TaskRow } from '$lib/server/public-api/record/tasks';
import { HintRefused } from '$lib/server/public-api/record/hint-resolution';

function task(index: number, organizationID: string | null = null): TaskRow {
	return {
		id: String(index).padStart(6, '0'), parent_task_id: null, title: `Task ${index}`,
		status: 'planned', note: null, location: null, business: null, type: null, size: null,
		is_event: false, is_whole_day: false, is_open_to_company: false, notify_minutes_before: null, starts_at: null,
		ends_at: null, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z',
		organization_id: organizationID, opportunity_id: null, contact_id: null, due_at: null,
		requester_id: null, task_participant: []
	};
}

function record(rows: TaskRow[], denied = false) {
	const queries: URL[] = [];
	const request: typeof fetch = Object.assign(async (input: Parameters<typeof fetch>[0]) => {
		const url = new URL(typeof input === 'string' ? input : input instanceof URL ? input.href : input.url);
		queries.push(url);
		if (denied) return Response.json({ message: 'permission denied', code: '42501' }, { status: 403 });
		let matching = rows.filter(row => String(row.is_event) === url.searchParams.get('is_event')?.slice(3));
		const organization = url.searchParams.get('organization_id');
		if (organization === 'not.is.null') matching = matching.filter(row => row.organization_id !== null);
		if (organization?.startsWith('eq.')) matching = matching.filter(row => row.organization_id === organization.slice(3));
		const status = url.searchParams.get('status');
		if (status?.startsWith('eq.')) matching = matching.filter(row => row.status === status.slice(3));
		const identifier = url.searchParams.get('id');
		if (identifier) matching = matching.filter(row => row.id === identifier.slice(3));
		const from = Number(url.searchParams.get('offset') ?? 0);
		const limit = Number(url.searchParams.get('limit') ?? matching.length);
		return Response.json(matching.slice(from, from + limit));
	}, { preconnect() {} });
	const caller = createClient('https://record.example.com', 'test-key', {
		auth: { persistSession: false, autoRefreshToken: false }, global: { fetch: request }
	});
	return { caller, queries };
}

describe('task query scope before pagination', () => {
	test('CRM reads one page despite 1500 unrelated tasks', async () => {
		const { caller, queries } = record([...Array.from({ length: 1500 }, (_, index) => task(index)), task(1501, 'customer')]);
		const found = await tasksOfCompany(caller, false, { linkedToOrganization: true });
		expect(found.map(row => row.id)).toEqual(['001501']);
		expect(queries).toHaveLength(1);
		expect(queries[0].searchParams.get('organization_id')).toBe('not.is.null');
	});

	test('keeps every matching row past three page boundaries', async () => {
		const { caller, queries } = record(Array.from({ length: 1501 }, (_, index) => task(index, 'customer')));
		const found = await tasksOfCompany(caller, false, { linkedToOrganization: true, organizationID: 'customer' });
		expect(found).toHaveLength(1501);
		expect(new Set(found.map(row => row.id)).size).toBe(1501);
		expect(queries).toHaveLength(4);
		expect(queries.every(url => url.searchParams.get('organization_id') === 'eq.customer')).toBe(true);
	});

	test('does not narrow ordinary task history to CRM rows', async () => {
		const { caller } = record([task(1), task(2, 'customer')]);
		expect(await tasksOfCompany(caller, false)).toHaveLength(2);
	});

	test('a requested-task count does not read completed history', async () => {
		const requested = { ...task(1600), status: 'requested' };
		const { caller, queries } = record([...Array.from({ length: 1501 }, (_, index) => ({ ...task(index), status: 'completed' })), requested]);
		expect((await tasksOfCompany(caller, false, { status: 'requested' })).map(row => row.id)).toEqual(['001600']);
		expect(queries).toHaveLength(1);
		expect(queries[0].searchParams.get('status')).toBe('eq.requested');
	});

	test('readback asks for the saved identifier and event kind only', async () => {
		const { caller, queries } = record(Array.from({ length: 1501 }, (_, index) => task(index)));
		expect((await savedTaskByID(caller, '001500', false)).id).toBe('001500');
		expect(queries).toHaveLength(1);
		expect(queries[0].searchParams.get('id')).toBe('eq.001500');
		await expect(savedTaskByID(caller, '001500', true)).rejects.toBeInstanceOf(WriteNotReadBack);
	});
});

describe('caller-scoped task and event hint resolution', () => {
	const identifier = 'abcdef01-1234-5678-9012-abcdef012345';

	for (const areEvents of [false, true]) {
		test(`exact ${areEvents ? 'event' : 'task'} UUID needs one row read despite 1501 unrelated records`, async () => {
			const wanted = { ...task(1600), id: identifier, is_event: areEvents };
			const { caller, queries } = record([
				...Array.from({ length: 1501 }, (_, index) => ({ ...task(index), is_event: areEvents })),
				{ ...task(1601), title: identifier, is_event: areEvents }, wanted
			]);
			expect(await taskFromHint(caller, ` ${identifier.toUpperCase()} `, areEvents)).toEqual(wanted);
			expect(queries).toHaveLength(1);
			expect(queries[0].searchParams.get('id')).toBe(`eq.${identifier}`);
			expect(queries[0].searchParams.get('is_event')).toBe(`eq.${areEvents}`);
			expect(queries[0].searchParams.has('limit')).toBe(false);
		});
	}

	test('missing or wrong-kind UUID falls back to the existing title resolver', async () => {
		const titleMatch = { ...task(1), title: identifier };
		const { caller, queries } = record([{ ...task(2), id: identifier, is_event: true }, titleMatch]);
		expect(await taskFromHint(caller, identifier, false)).toEqual(titleMatch);
		expect(queries).toHaveLength(2);
		expect(queries[1].searchParams.has('id')).toBe(false);
	});

	test('title resolution keeps preferred owner and ambiguity behavior without an ID query', async () => {
		const preferred = { ...task(1), title: 'Shared task', task_participant: [{ member_id: 'viewer' }] };
		const { caller, queries } = record([preferred, { ...task(2), title: 'Shared task' }]);
		expect(await taskFromHint(caller, 'Shared task', false, 'viewer')).toEqual(preferred);
		await expect(taskFromHint(caller, 'Shared task', false)).rejects.toBeInstanceOf(HintRefused);
		expect(queries.every(query => !query.searchParams.has('id'))).toBe(true);
	});

	test('a UUID absent from visible records preserves not-found refusal', async () => {
		const { caller, queries } = record([]);
		await expect(taskFromHint(caller, identifier, true)).rejects.toBeInstanceOf(HintRefused);
		expect(queries).toHaveLength(2);
	});

	test('a failed exact read propagates refusal without a broad fallback', async () => {
		const { caller, queries } = record([], true);
		await expect(taskFromHint(caller, identifier, false)).rejects.toThrow('permission denied');
		expect(queries).toHaveLength(1);
	});
});
