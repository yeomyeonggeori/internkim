import { describe, expect, test } from 'bun:test';
import {
	everyoneKey,
	everyoneLabel,
	matchingMentions,
	mentionCandidates,
	mentionPeopleOf,
	type MentionPerson
} from '$lib/messenger/mention-candidates';

const people: MentionPerson[] = [
	{ externalID: 'external-1', name: '이샘플' },
	{ externalID: 'external-2', name: '박예시' },
	{ externalID: 'external-3', name: 'Sample Choi' }
];

describe('mentionCandidates', () => {
	test('a channel can call on everyone, and offers it first', () => {
		const candidates = mentionCandidates(people, true);
		expect(candidates[0]).toEqual({ key: 'everyone', label: everyoneLabel, isEveryone: true });
		expect(candidates).toHaveLength(4);
	});

	test('a conversation between two people cannot call on everyone', () => {
		expect(mentionCandidates(people, false).some((candidate) => candidate.isEveryone)).toBe(false);
	});

	test('leaves out anyone without a name to write', () => {
		const unnamed = [...people, { externalID: 'external-4', name: '  ' }];
		expect(mentionCandidates(unnamed, false)).toHaveLength(3);
	});
});

describe('matchingMentions', () => {
	const candidates = mentionCandidates(people, true);

	test('an empty query offers everyone the channel holds', () => {
		const crowd = Array.from({ length: 12 }, (_, index) => ({ externalID: `external-${index}`, name: `박예시${index}` }));
		expect(matchingMentions(mentionCandidates(crowd, true), '')).toHaveLength(13);
	});

	test('offers what the name starts with before what it merely contains', () => {
		const matched = matchingMentions(candidates, 'sample');
		expect(matched.map((candidate) => candidate.label)).toEqual(['Sample Choi']);
	});

	test('finds a name by a word inside it, ignoring case', () => {
		expect(matchingMentions(candidates, 'CHOI').map((candidate) => candidate.label)).toEqual([
			'Sample Choi'
		]);
	});

	test('everyone answers to its own name', () => {
		expect(matchingMentions(candidates, 'al').map((candidate) => candidate.label)).toEqual([everyoneLabel]);
	});

	test('a query nobody answers to offers nothing', () => {
		expect(matchingMentions(candidates, '없는사람')).toEqual([]);
	});
});

describe('mentionPeopleOf', () => {
	test('a channel offers the people it holds', () => {
		const channel = { kind: 'group' as const, members: [{ externalID: 'external-1', name: '이샘플' }] };
		expect(mentionPeopleOf(channel)).toEqual([{ externalID: 'external-1', name: '이샘플' }]);
	});

	test('a conversation between two people offers nobody', () => {
		expect(mentionPeopleOf({ kind: 'dm' })).toEqual([]);
	});

	test('leaves out a member the messenger cannot address', () => {
		const channel = { kind: 'group' as const, members: [{ name: '이샘플' }] };
		expect(mentionPeopleOf(channel)).toEqual([]);
	});
});

describe('the agent leads the mention list', () => {
	test('in a channel the agent comes before everyone and the people', () => {
		const people = [
			{ externalID: 'pk-park', name: '박예시' },
			{ externalID: 'pk-agent', name: '김인턴', isAgent: true }
		];
		expect(mentionCandidates(people, true).map((candidate) => candidate.key)).toEqual(['pk-agent', everyoneKey, 'pk-park']);
	});

	test('a query the agent and a person match equally keeps the agent first', () => {
		const people = [
			{ externalID: 'pk-kim', name: '김샘플' },
			{ externalID: 'pk-agent', name: '김인턴', isAgent: true }
		];
		expect(matchingMentions(mentionCandidates(people, false), '김').map((candidate) => candidate.key)).toEqual(['pk-agent', 'pk-kim']);
	});
});
