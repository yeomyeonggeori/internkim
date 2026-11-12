import { describe, expect, test } from 'bun:test';
import { createClient } from '@supabase/supabase-js';
import { attendancePage } from '$lib/server/public-api/record/attendance-page';
import { taskPage } from '$lib/server/public-api/record/task-page';
import { RecordReadFailed } from '$lib/server/public-api/record/tasks';

const statementTimeout = { code: '57014', message: 'canceling statement due to statement timeout', details: null, hint: null };
const everyone = { everyone: true, personIDs: [] };

function isCountRequest(init?: RequestInit): boolean {
	return init?.method === 'HEAD' || new Headers(init?.headers).get('Prefer')?.includes('count=exact') === true;
}

function recordWhoseCountsTimeOut() {
	const request: typeof fetch = Object.assign(async (_input: Parameters<typeof fetch>[0], init?: RequestInit) => {
		if (!isCountRequest(init)) return Response.json([]);
		const body = init?.method === 'HEAD' ? null : JSON.stringify(statementTimeout);
		return new Response(body, { status: 500, statusText: 'Internal Server Error' });
	}, { preconnect: fetch.preconnect });
	return createClient('http://record.test', 'key', { global: { fetch: request } });
}

describe('a page the record cannot count', () => {
	const answered = 'the record answered 500 Internal Server Error (57014): canceling statement due to statement timeout';

	test('names the Postgres code on a task page', async () => {
		const reading = taskPage(recordWhoseCountsTimeOut(), { whose: everyone, carriesLinks: false }, {}, 'task_list');
		await expect(reading).rejects.toBeInstanceOf(RecordReadFailed);
		await expect(reading).rejects.toThrow(answered);
	});

	test('names the Postgres code on an attendance page', async () => {
		const scope = { whose: everyone, from: '2026-11-01T00:00:00Z', to: '2026-11-30T00:00:00Z', handWrittenOnly: false };
		const reading = attendancePage(recordWhoseCountsTimeOut(), scope, {});
		await expect(reading).rejects.toBeInstanceOf(RecordReadFailed);
		await expect(reading).rejects.toThrow(answered);
	});
});
