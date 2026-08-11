import { describe, expect, test } from 'bun:test';
import type { SupabaseClient } from '@supabase/supabase-js';
import { externalIDsWithACredential } from '../../../src/lib/server/member-credential';

type CredentialRow = { member_id: string; external_id: string | null };
type ContactRow = { member_id: string | null; external_id: string | null };

function aRecordHolding(contacts: ContactRow[], credentials: CredentialRow[], secrets: Record<string, string>) {
	const rpcCalls: string[] = [];
	const client = {
		from(table: string) {
			const rows = table === 'contact' ? contacts : credentials;
			const query = {
				select: () => query,
				eq: () => query,
				in: () => query,
				not: () => query,
				returns: () => Promise.resolve({ data: rows, error: null }),
				then: (resolve: (answer: { data: unknown; error: null }) => unknown) =>
					resolve({ data: rows, error: null })
			};
			return query;
		},
		rpc(_name: string, parameters: { target_member: string }) {
			rpcCalls.push(parameters.target_member);
			return Promise.resolve({ data: secrets[parameters.target_member] ?? '', error: null });
		}
	} as unknown as SupabaseClient;
	return { client, rpcCalls };
}

describe('externalIDsWithACredential', () => {
	const contacts: ContactRow[] = [
		{ member_id: 'member-1', external_id: 'U-one' },
		{ member_id: 'member-2', external_id: 'U-two' }
	];
	const credentials: CredentialRow[] = [
		{ member_id: 'member-1', external_id: 'U-one' },
		{ member_id: 'member-2', external_id: 'U-two' }
	];

	test('a row whose secret the vault never took does not count as held', async () => {
		const { client } = aRecordHolding(contacts, credentials, {});

		expect(await externalIDsWithACredential(client, 'company-1', 'mattermost')).toEqual([]);
	});

	test('only the rows the vault can answer for count', async () => {
		const { client } = aRecordHolding(contacts, credentials, { 'member-2': 'a-real-token' });

		expect(await externalIDsWithACredential(client, 'company-1', 'mattermost')).toEqual(['U-two']);
	});

	test('every row is asked about, so one readable secret does not vouch for the rest', async () => {
		const { client, rpcCalls } = aRecordHolding(contacts, credentials, { 'member-1': 'a-real-token' });

		await externalIDsWithACredential(client, 'company-1', 'mattermost');

		expect(rpcCalls).toEqual(['member-1', 'member-2']);
	});
});
