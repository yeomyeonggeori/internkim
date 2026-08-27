import { describe, expect, test } from 'bun:test';
import type { SupabaseClient } from '@supabase/supabase-js';
import {
	connectMessengerAccount,
	memberCredential
} from '../../../src/lib/server/member-credential';

type Written = { table: string; row: Record<string, unknown> };

function aRecordThatRemembers(held: Record<string, string> = {}) {
	const written: Written[] = [];
	const vaulted: { member: string; kind: string; secret: string }[] = [];
	const client = {
		from(table: string) {
			return {
				upsert: (row: Record<string, unknown>) => {
					written.push({ table, row });
					return Promise.resolve({ error: null });
				},
				select: () => ({
					eq: () => ({
						single: () => Promise.resolve({ data: { messenger: held }, error: null })
					})
				}),
				update: (row: Record<string, unknown>) => ({
					eq: () => {
						written.push({ table, row });
						return Promise.resolve({ error: null });
					}
				})
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
	platform: 'mattermost',
	kind: 'mattermost-token',
	externalID: 'U-one',
	name: '이샘플',
	secret: 'a-durable-token'
};

describe('connecting a member to their own messenger account', () => {
	test('the secret goes to the vault rather than into a row', async () => {
		const { client, written, vaulted } = aRecordThatRemembers();

		await connectMessengerAccount(client, 'company-1', account);

		expect(vaulted).toEqual([
			{ member: 'member-1', kind: 'mattermost-token', secret: 'a-durable-token' }
		]);
		expect(JSON.stringify(written)).not.toContain('a-durable-token');
	});

	test('the account lands on the member, so the member is addressable at once', async () => {
		const { client, written } = aRecordThatRemembers();

		await connectMessengerAccount(client, 'company-1', account);

		expect(written.map((entry) => entry.table)).toEqual(['credential', 'member']);
		expect(written[1]?.row).toEqual({ messenger: { mattermost: 'U-one' } });
	});

	test('connecting one messenger leaves the others the member already had', async () => {
		const { client, written } = aRecordThatRemembers({ buzz: 'pubkey-1' });

		await connectMessengerAccount(client, 'company-1', account);

		expect(written[1]?.row).toEqual({ messenger: { buzz: 'pubkey-1', mattermost: 'U-one' } });
	});

	test('the credential is keyed to the member, never to a company-wide account', async () => {
		const { client, written } = aRecordThatRemembers();

		await connectMessengerAccount(client, 'company-1', account);

		expect(written[0]?.row).toEqual({
			member_id: 'member-1',
			company_id: null,
			kind: 'mattermost-token',
			external_id: 'U-one',
			vault_secret_id: 'vault-secret-1',
			name: ''
		});
	});
});

function aRecordHolding(credentials: { kind: string; external_id: string; secret: string }[]) {
	const asked: string[] = [];
	const client = {
		from() {
			return {
				select: () => ({
					eq: () => ({
						eq: (_column: string, kind: string) => ({
							maybeSingle: () => {
								asked.push(kind);
								const held = credentials.find((credential) => credential.kind === kind);
								return Promise.resolve({
									data: held ? { external_id: held.external_id } : null,
									error: null
								});
							}
						})
					})
				})
			};
		},
		rpc(_name: string, parameters: { target_kind: string }) {
			const held = credentials.find((credential) => credential.kind === parameters.target_kind);
			return Promise.resolve({ data: held?.secret ?? null, error: null });
		}
	} as unknown as SupabaseClient;
	return { client, asked };
}

describe('reading the credential a messenger will accept', () => {
	const held = [
		{ kind: 'api_key', external_id: 'key-one', secret: 'an-api-key' },
		{ kind: 'buzz-secret', external_id: 'pubkey-1', secret: 'a-buzz-secret' }
	];

	test('the kind asked for is the kind that comes back', async () => {
		const { client, asked } = aRecordHolding(held);

		expect(await memberCredential(client, 'member-1', 'buzz-secret')).toEqual({
			kind: 'buzz-secret',
			externalID: 'pubkey-1',
			secret: 'a-buzz-secret'
		});
		expect(asked).toEqual(['buzz-secret']);
	});

	test('a kind the member does not hold is nothing, never another kind', async () => {
		const { client } = aRecordHolding(held);

		expect(await memberCredential(client, 'member-1', 'mattermost-token')).toBeNull();
	});
});
