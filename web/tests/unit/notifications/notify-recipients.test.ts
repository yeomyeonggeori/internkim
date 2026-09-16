import { describe, expect, test } from 'bun:test';
import { addressedIn } from '../../../../supabase/functions/_shared/notify-recipients.ts';

describe('who a notify request addresses', () => {
	test('a device names people by address and no messenger', () => {
		expect(addressedIn({ platform: '', externalIDs: null, emails: [' Member1@Example.com ', ''] })).toEqual({
			by: 'email',
			keys: ['member1@example.com']
		});
	});

	test('a messenger arrival names people by their account there', () => {
		expect(addressedIn({ platform: 'buzz', externalIDs: ['pubkey-one', 7] })).toEqual({
			by: 'messenger',
			platform: 'buzz',
			keys: ['pubkey-one']
		});
	});

	test('addresses decide when both are given', () => {
		expect(addressedIn({ platform: 'buzz', externalIDs: ['pubkey-one'], emails: ['member1@example.com'] })).toEqual({
			by: 'email',
			keys: ['member1@example.com']
		});
	});

	test('a request naming nobody it can resolve is refused', () => {
		expect(addressedIn({ platform: '', externalIDs: ['pubkey-one'] })).toBeNull();
		expect(addressedIn({ platform: 'buzz' })).toBeNull();
		expect(addressedIn({})).toBeNull();
	});
});
