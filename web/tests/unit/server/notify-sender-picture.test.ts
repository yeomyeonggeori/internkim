import { describe, expect, test } from 'bun:test';
import type { SupabaseClient } from '@supabase/supabase-js';
import {
	isCompanySharedPath,
	pictureURLOfSender
} from '../../../../supabase/functions/_shared/sender-picture.ts';

const company = '43000000-0000-0000-0000-0000000000a0';
const kept = `${company}/shared/sender-picture/digest.png`;

function aDirectoryHolding(profileImage: string | null) {
	const signed: string[] = [];
	const client = {
		from: () => ({
			select: () => ({
				eq: () => ({ maybeSingle: () => Promise.resolve({ data: { profile_image: profileImage }, error: null }) })
			})
		}),
		storage: {
			from: () => ({
				createSignedUrl: (path: string) => {
					signed.push(path);
					return Promise.resolve({ data: { signedUrl: `https://example.com/${path}` }, error: null });
				}
			})
		}
	} as unknown as SupabaseClient;
	return { client, signed };
}

describe('the picture a message notification shows', () => {
	test('is the copy the relay kept of the sender\'s messenger picture', async () => {
		const directory = aDirectoryHolding(`${company}/shared/profile/member.png`);
		expect(await pictureURLOfSender(directory.client, company, kept, 'member-1')).toBe(`https://example.com/${kept}`);
		expect(directory.signed).toEqual([kept]);
	});

	test('is the member row picture when the relay kept none', async () => {
		const directory = aDirectoryHolding(`${company}/shared/profile/member.png`);
		await pictureURLOfSender(directory.client, company, '', 'member-1');
		expect(directory.signed).toEqual([`${company}/shared/profile/member.png`]);
	});

	test('is never a path outside the calling company\'s shared scope', async () => {
		const directory = aDirectoryHolding(null);
		const other = '43000000-0000-0000-0000-0000000000b0/shared/sender-picture/digest.png';
		expect(await pictureURLOfSender(directory.client, company, other, 'member-1')).toBe('');
		expect(directory.signed).toEqual([]);
	});
});

describe('isCompanySharedPath', () => {
	test('refuses a path that climbs out of the shared scope', () => {
		expect(isCompanySharedPath(company, `${company}/shared/../private/x.png`)).toBe(false);
		expect(isCompanySharedPath(company, `${company}/private/member/x.png`)).toBe(false);
		expect(isCompanySharedPath(company, `${company}/shared`)).toBe(false);
		expect(isCompanySharedPath(company, 42)).toBe(false);
	});

	test('takes a path under the company\'s shared scope', () => {
		expect(isCompanySharedPath(company, kept)).toBe(true);
	});
});
