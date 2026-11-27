import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, test } from 'bun:test';
import type { SupabaseClient } from '@supabase/supabase-js';
import { issuePersonalAccessToken } from '../../src/lib/server/control-plane';

const kinds = readFileSync(join(import.meta.dir, '..', '..', '..', 'internal', 'personaldata', 'kinds.go'), 'utf8');

function maskedPersonalAccessTokenPattern(): RegExp {
	const declared = /`(ik_[^`]+)`/.exec(kinds);
	if (!declared) throw new Error('internal/personaldata/kinds.go no longer names the ik_ token among the API keys it masks');
	return new RegExp(`^${declared[1]}$`);
}

const acceptingClient = {
	from: () => ({ upsert: async () => ({ error: null }) }),
} as unknown as SupabaseClient;

describe('the personal access token the model route masks', () => {
	test('is the shape the web app issues', async () => {
		const issued = await issuePersonalAccessToken(acceptingClient, 'member-1', 'sample');
		expect(issued).toMatch(maskedPersonalAccessTokenPattern());
	});
});
