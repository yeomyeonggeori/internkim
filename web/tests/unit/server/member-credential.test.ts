import { describe, expect, test } from 'bun:test';
import type { SupabaseClient } from '@supabase/supabase-js';
import { connectMessengerAccount } from '../../../src/lib/server/member-credential';

type Written = { table: string; row: Record<string, unknown> };

function aRecordThatRemembers() {
	const written: Written[] = [];
	const vaulted: { member: string; kind: string; secret: string }[] = [];
	const client = {
		from(table: string) {
			return {
				upsert: (row: Record<string, unknown>) => {
					written.push({ table, row });
					return Promise.resolve({ error: null });
				}
			};
		},
		rpc(_name: string, parameters: { target_member: string; target_kind: string; secret_value: string }) {
			vaulted.push({
				member: parameters.target_member,
				kind: parameters.target_kind,
				secret: parameters.secret_value
			});
			return Promise.resolve({ data: 'vault-secret-1', error: null });
		}
	} as unknown as SupabaseClient;
	return { client, written, vaulted };
}

const account = {
	memberID: 'member-1',
	kind: 'mattermost',
	externalID: 'U-one',
	name: '이샘플',
	secret: 'a-durable-token'
};

describe('connecting a member to their own messenger account', () => {
	test('the secret goes to the vault rather than into a row', async () => {
		const { client, written, vaulted } = aRecordThatRemembers();

		await connectMessengerAccount(client, 'company-1', account);

		expect(vaulted).toEqual([
			{ member: 'member-1', kind: 'mattermost', secret: 'a-durable-token' }
		]);
		expect(JSON.stringify(written)).not.toContain('a-durable-token');
	});

	test('the contact row is written in the same call, so the member is addressable at once', async () => {
		const { client, written } = aRecordThatRemembers();

		await connectMessengerAccount(client, 'company-1', account);

		expect(written.map((entry) => entry.table)).toEqual(['credential', 'contact']);
		expect(written[1]?.row).toEqual({
			company_id: 'company-1',
			platform: 'mattermost',
			external_id: 'U-one',
			name: '이샘플',
			member_id: 'member-1'
		});
	});

	test('the credential is keyed to the member, never to a company-wide account', async () => {
		const { client, written } = aRecordThatRemembers();

		await connectMessengerAccount(client, 'company-1', account);

		expect(written[0]?.row).toEqual({
			member_id: 'member-1',
			company_id: null,
			kind: 'mattermost',
			external_id: 'U-one',
			vault_secret_id: 'vault-secret-1'
		});
	});
});
