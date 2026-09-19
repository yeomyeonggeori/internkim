import { describe, expect, test } from 'bun:test';
import { accountsHeldBy, externalIDFor, type MessengerDirectory } from '$lib/messenger/messenger-directory';

// Somebody the company moved from one messenger to another holds an account on
// each. Only the one the company runs today answers with a picture, and which
// that is belongs to the host, so a caller here asks after all of them.
const directory: MessengerDirectory = {
	nameOfMember: new Map([['member-1', '예시 김']]),
	nameOfExternal: new Map(),
	memberOfExternal: new Map([
		['mattermost-account', 'member-1'],
		['buzz-account', 'member-1']
	]),
	externalsOfMember: new Map([['member-1', ['mattermost-account', 'buzz-account']]]),
	memberOfEmail: new Map([['sample@example.com', 'member-1']]),
	adminMemberIDs: new Set<string>()
};

describe('accountsHeldBy', () => {
	test('gives every account a member holds', () => {
		expect(accountsHeldBy({ memberID: 'member-1' }, directory)).toEqual(['mattermost-account', 'buzz-account']);
	});

	test('finds them by email as well as by member', () => {
		expect(accountsHeldBy({ email: 'Sample@Example.com ' }, directory)).toEqual([
			'mattermost-account',
			'buzz-account'
		]);
	});

	test('gives nothing for somebody the record does not place', () => {
		expect(accountsHeldBy({ email: 'stranger@example.com' }, directory)).toEqual([]);
		expect(accountsHeldBy({}, directory)).toEqual([]);
	});

	test('externalIDFor still answers with one, for callers that want one', () => {
		expect(externalIDFor({ memberID: 'member-1' }, directory)).toBe('mattermost-account');
	});
});
