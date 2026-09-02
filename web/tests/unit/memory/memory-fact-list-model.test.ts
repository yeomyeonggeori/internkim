import { describe, expect, test } from 'bun:test';
import type { MemoryFact } from '../../../src/routes/memory/memory-facts-api';
import {
	allFilterValue,
	emptyMemoryFactFilters,
	filterMemoryFacts,
	isExpiringFact,
	isOwnOnlyFact,
	memoryFactCirclesOf,
	memoryFactKindsOf,
	ownOnlyFilterValue,
	sortMemoryFactsByRecency
} from '../../../src/routes/memory/memory-fact-list-model';

const facts: MemoryFact[] = [
	{
		factID: 'fact-1',
		episodeID: 'episode-1',
		ownerPersonID: 'person-1',
		circleIDs: [],
		kind: 'preference',
		content: 'The user prefers terse release notes.',
		validFrom: '2026-07-01T09:00:00Z',
		reinforcementCount: 2
	},
	{
		factID: 'fact-2',
		episodeID: 'episode-2',
		ownerPersonID: 'person-1',
		circleIDs: [],
		kind: 'temporary',
		content: 'The user is away until Friday.',
		validFrom: '2026-07-03T09:00:00Z',
		validUntil: '2026-07-11T00:00:00Z',
		reinforcementCount: 1
	},
	{
		factID: 'fact-3',
		episodeID: 'episode-3',
		ownerPersonID: 'person-2',
		circleIDs: ['hr'],
		kind: 'fact',
		content: 'Compensation data belongs to HR.',
		validFrom: '2026-07-05T09:00:00Z',
		reinforcementCount: 1
	},
	{
		factID: 'fact-4',
		episodeID: 'episode-4',
		ownerPersonID: 'person-3',
		circleIDs: ['member'],
		kind: 'episode',
		content: 'Quarterly launch review happened on Friday.',
		validFrom: '2026-06-20T09:00:00Z',
		reinforcementCount: 1
	}
];

describe('filterMemoryFacts', () => {
	test('returns all facts with empty filters', () => {
		expect(filterMemoryFacts(facts, emptyMemoryFactFilters()).length).toBe(4);
	});

	test('filters by kind', () => {
		const filtered = filterMemoryFacts(facts, { searchText: '', kind: 'temporary', circle: allFilterValue });
		expect(filtered.map((fact) => fact.factID)).toEqual(['fact-2']);
	});

	test('filters to facts kept for the owner alone', () => {
		const filtered = filterMemoryFacts(facts, { searchText: '', kind: allFilterValue, circle: ownOnlyFilterValue });
		expect(filtered.map((fact) => fact.factID)).toEqual(['fact-1', 'fact-2']);
	});

	test('filters by a circle', () => {
		const filtered = filterMemoryFacts(facts, { searchText: '', kind: allFilterValue, circle: 'hr' });
		expect(filtered.map((fact) => fact.factID)).toEqual(['fact-3']);
	});

	test('search text matches content case-insensitively', () => {
		const filtered = filterMemoryFacts(facts, { searchText: 'COMPENSATION', kind: allFilterValue, circle: allFilterValue });
		expect(filtered.map((fact) => fact.factID)).toEqual(['fact-3']);
	});

	test('search text matches the circle', () => {
		const filtered = filterMemoryFacts(facts, { searchText: 'hr', kind: allFilterValue, circle: allFilterValue });
		expect(filtered.map((fact) => fact.factID)).toEqual(['fact-3']);
	});

	test('combines filters', () => {
		const filtered = filterMemoryFacts(facts, { searchText: 'user', kind: 'preference', circle: ownOnlyFilterValue });
		expect(filtered.map((fact) => fact.factID)).toEqual(['fact-1']);
	});
});

describe('sortMemoryFactsByRecency', () => {
	test('sorts by validFrom descending', () => {
		expect(sortMemoryFactsByRecency(facts).map((fact) => fact.factID)).toEqual(['fact-3', 'fact-2', 'fact-1', 'fact-4']);
	});

	test('does not mutate the input array', () => {
		const input = [...facts];
		sortMemoryFactsByRecency(input);
		expect(input.map((fact) => fact.factID)).toEqual(['fact-1', 'fact-2', 'fact-3', 'fact-4']);
	});
});

describe('filter options', () => {
	test('memoryFactKindsOf returns distinct sorted kinds', () => {
		expect(memoryFactKindsOf(facts)).toEqual(['episode', 'fact', 'preference', 'temporary']);
	});

	test('memoryFactCirclesOf returns distinct sorted circles', () => {
		expect(memoryFactCirclesOf(facts)).toEqual(['hr', 'member']);
	});
});

describe('isOwnOnlyFact', () => {
	test('is true only without circles', () => {
		expect(isOwnOnlyFact(facts[0])).toBe(true);
		expect(isOwnOnlyFact(facts[2])).toBe(false);
	});
});

describe('isExpiringFact', () => {
	test('is true only for temporary facts with an expiry', () => {
		expect(isExpiringFact(facts[1])).toBe(true);
		expect(isExpiringFact(facts[0])).toBe(false);
	});
});
