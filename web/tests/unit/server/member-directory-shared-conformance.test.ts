import { describe, expect, test } from 'bun:test';
import type { SupabaseClient } from '@supabase/supabase-js';
import { membersOfCompanyByExternalID as webMembersByExternalID } from '../../../src/lib/server/member-credential';
import { pictureURLOfMember as webPictureURL } from '../../../src/lib/server/member-picture-url';
import {
	membersOfCompanyByExternalID as sharedMembersByExternalID,
	pictureURLOfMember as sharedPictureURL
} from '../../../../supabase/functions/_shared/member-directory.ts';

type Asked = { table?: string; columns?: string; column?: string; value?: string; bucket?: string; path?: string; seconds?: number };

const memberRows = [
	{ id: 'member-one', messenger: { mattermost: 'U-one', slack: 'S-one' } },
	{ id: 'member-two', messenger: { slack: 'S-two' } },
	{ id: 'member-three', messenger: null },
	{ id: 'member-four', messenger: { mattermost: '' } }
];

function aDirectoryThatAnswers(profileImage: string | null) {
	const asked: Asked[] = [];
	const client = {
		from(table: string) {
			return {
				select: (columns: string) => ({
					eq: (column: string, value: string) => {
						asked.push({ table, columns, column, value });
						return {
							returns: () => Promise.resolve({ data: memberRows, error: null }),
							maybeSingle: () => Promise.resolve({ data: { profile_image: profileImage }, error: null })
						};
					}
				})
			};
		},
		storage: {
			from(bucket: string) {
				return {
					createSignedUrl: (path: string, seconds: number) => {
						asked.push({ bucket, path, seconds });
						return Promise.resolve({ data: { signedUrl: `https://example.com/${path}` }, error: null });
					}
				};
			}
		}
	} as unknown as SupabaseClient;
	return { client, asked };
}

describe('the shared member directory stays interchangeable with the web one', () => {
	test('both read the same rows and keep the same external identities', async () => {
		const web = aDirectoryThatAnswers(null);
		const shared = aDirectoryThatAnswers(null);

		const webFound = await webMembersByExternalID(web.client, 'company-one', 'mattermost');
		const sharedFound = await sharedMembersByExternalID(shared.client, 'company-one', 'mattermost');

		expect([...sharedFound]).toEqual([...webFound]);
		expect([...webFound]).toEqual([['U-one', 'member-one']]);
		expect(shared.asked).toEqual(web.asked);
	});

	test('both sign the same picture out of the same bucket', async () => {
		const web = aDirectoryThatAnswers('member/one.png');
		const shared = aDirectoryThatAnswers('member/one.png');

		expect(await sharedPictureURL(shared.client, 'member-one')).toBe(await webPictureURL(web.client, 'member-one'));
		expect(shared.asked).toEqual(web.asked);
		expect(web.asked.at(-1)?.bucket).toBe('asset');
	});

	test('both answer with no picture for a member who has none', async () => {
		const web = aDirectoryThatAnswers(null);
		const shared = aDirectoryThatAnswers(null);

		expect(await sharedPictureURL(shared.client, 'member-one')).toBe(await webPictureURL(web.client, 'member-one'));
		expect(await sharedPictureURL(shared.client, '')).toBe(await webPictureURL(web.client, ''));
		expect(shared.asked).toEqual(web.asked);
	});
});
