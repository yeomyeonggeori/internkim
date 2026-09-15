import { describe, expect, test } from 'bun:test';
import type { SupabaseClient } from '@supabase/supabase-js';
import { colleagueWhoSent } from '../../../../supabase/functions/_shared/member-directory.ts';

type MemberRow = { id: string; name: string | null; company_id: string | null; profile_image: string | null };

function directoryHolding(members: MemberRow[]): { client: SupabaseClient; signed: string[] } {
	const signed: string[] = [];
	const client = {
		from: () => ({
			select: () => ({
				in: (_column: string, ids: string[]) => ({
					returns: async () => ({ data: members.filter((member) => ids.includes(member.id)), error: null })
				})
			})
		}),
		storage: {
			from: () => ({
				createSignedUrl: async (path: string) => {
					signed.push(path);
					return { data: { signedUrl: `https://storage.example.test/signed/${path}` }, error: null };
				}
			})
		}
	} as unknown as SupabaseClient;
	return { client, signed };
}

const recipient: MemberRow = { id: 'member-1', name: '최견본', company_id: 'company-1', profile_image: null };
const sender: MemberRow = { id: 'member-2', name: '이샘플', company_id: 'company-1', profile_image: 'company-1/shared/members/member-2.png' };

describe('the colleague a notification comes from', () => {
	test('a colleague in the recipient company lends their name and a signed picture', async () => {
		const { client, signed } = directoryHolding([recipient, sender]);

		expect(await colleagueWhoSent(client, 'member-2', 'member-1')).toEqual({
			senderName: '이샘플',
			icon: 'https://storage.example.test/signed/company-1/shared/members/member-2.png'
		});
		expect(signed).toEqual(['company-1/shared/members/member-2.png']);
	});

	test('a sender from another company lends nothing, and no picture is signed', async () => {
		const outsider = { ...sender, company_id: 'company-2' };
		const { client, signed } = directoryHolding([recipient, outsider]);

		expect(await colleagueWhoSent(client, 'member-2', 'member-1')).toBeNull();
		expect(signed).toEqual([]);
	});

	test('a sender the record does not hold lends nothing', async () => {
		const { client } = directoryHolding([recipient]);

		expect(await colleagueWhoSent(client, 'member-9', 'member-1')).toBeNull();
	});

	test('a colleague with no name lends no name, so the notification title stands in', async () => {
		const { client } = directoryHolding([recipient, { ...sender, name: '  ' }]);

		expect((await colleagueWhoSent(client, 'member-2', 'member-1'))?.senderName).toBe('');
	});

	test('a colleague with no picture kept lends their name and no picture', async () => {
		const { client, signed } = directoryHolding([recipient, { ...sender, profile_image: null }]);

		expect(await colleagueWhoSent(client, 'member-2', 'member-1')).toEqual({ senderName: '이샘플', icon: '' });
		expect(signed).toEqual([]);
	});
});
