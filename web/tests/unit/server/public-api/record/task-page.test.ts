import { describe, expect, test } from 'bun:test';
import { createClient } from '@supabase/supabase-js';
import { taskPage } from '$lib/server/public-api/record/task-page';
import { RecordReadFailed } from '$lib/server/public-api/record/tasks';

function recordWhoseCountsFail() {
	const request: typeof fetch = Object.assign(async (_input: Parameters<typeof fetch>[0], init?: RequestInit) => {
		if (init?.method === 'HEAD') return new Response(null, { status: 500, statusText: 'Internal Server Error' });
		return Response.json([]);
	}, { preconnect: fetch.preconnect });
	return createClient('http://record.test', 'key', { global: { fetch: request } });
}

describe('a task page the record cannot count', () => {
	test('names the status the record answered instead of an empty message', async () => {
		const reading = taskPage(recordWhoseCountsFail(), { whose: { everyone: true, personIDs: [] }, carriesLinks: false }, {}, 'task_list');
		await expect(reading).rejects.toBeInstanceOf(RecordReadFailed);
		await expect(reading).rejects.toThrow('the record answered 500 Internal Server Error');
	});
});
