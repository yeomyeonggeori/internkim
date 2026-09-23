import { describe, expect, test } from 'bun:test';
import {
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

	test('an empty query offers the front of the list', () => {
		expect(matchingMentions(candidates, '', 2)).toHaveLength(2);
	});

	test('offers what the name starts with before what it merely contains', () => {
		const matched = matchingMentions(candidates, 'sample', 5);
		expect(matched.map((candidate) => candidate.label)).toEqual(['Sample Choi']);
	});

	test('finds a name by a word inside it, ignoring case', () => {
		expect(matchingMentions(candidates, 'CHOI', 5).map((candidate) => candidate.label)).toEqual([
			'Sample Choi'
		]);
	});

	test('everyone answers to its own name', () => {
		expect(matchingMentions(candidates, 'al', 5).map((candidate) => candidate.label)).toEqual([everyoneLabel]);
	});

	test('a query nobody answers to offers nothing', () => {
		expect(matchingMentions(candidates, '없는사람', 5)).toEqual([]);
	});

	test('never offers more rows than it was asked for', () => {
		expect(matchingMentions(candidates, '', 2)).toHaveLength(2);
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
