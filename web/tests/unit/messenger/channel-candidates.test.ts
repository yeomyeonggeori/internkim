import { describe, expect, test } from 'bun:test';
import { channelCandidatesOf } from '../../../src/lib/messenger/channel-candidates';
import type { MessengerDirectory } from '../../../src/lib/messenger/messenger-directory';

const agentExternalID = 'a'.repeat(64);

function directory(): MessengerDirectory {
	return {
		nameOfMember: new Map([
			['member1', '이샘플'],
			['member2', '박예시']
		]),
		nameOfExternal: new Map(),
		memberOfExternal: new Map([
			['U1', 'member1'],
			['U2', 'member2']
		]),
		externalsOfMember: new Map([
			['member1', ['U1']],
			['member2', ['U2']]
		]),
		memberOfEmail: new Map([
			['sample@example.com', 'member1'],
			['example@example.com', 'member2']
		]),
		adminMemberIDs: new Set()
	};
}

const people = [
	{ externalID: 'U1', name: 'sample' },
	{ externalID: 'U2', name: 'example' },
	{ externalID: 'U9', name: 'nobody the company knows' }
];

describe('channelCandidatesOf', () => {
	test('offers the agent first, beside the members a channel may hold', () => {
		const candidates = channelCandidatesOf(people, directory(), { externalID: agentExternalID, name: '김인턴' });

		expect(candidates.map((candidate) => candidate.externalID)).toEqual([agentExternalID, 'U2', 'U1']);
		expect(candidates[0]).toEqual({
			memberID: agentExternalID,
			externalID: agentExternalID,
			name: '김인턴',
			email: ''
		});
	});

	test('offers only members when the messenger names no agent', () => {
		const candidates = channelCandidatesOf(people, directory(), undefined);

		expect(candidates.map((candidate) => candidate.externalID)).toEqual(['U2', 'U1']);
		expect(candidates.map((candidate) => candidate.email)).toEqual(['example@example.com', 'sample@example.com']);
	});
});
