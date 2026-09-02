import { describe, expect, test } from 'bun:test';
import type { MemoryFact } from '../../../src/routes/memory/memory-facts-api';
import {
	allFilterValue,
	emptyMemoryFactFilters,
	filterMemoryFacts,
	isExpiringFact,
	memoryFactKindsOf,
	memoryFactScopesOf,
	sortMemoryFactsByRecency
} from '../../../src/routes/memory/memory-fact-list-model';

const facts: MemoryFact[] = [
	{
		factID: 'fact-1',
		episodeID: 'episode-1',
		scopeType: 'private',
		scopeID: 'person-1',
		kind: 'preference',
		content: 'The user prefers terse release notes.',
		validFrom: '2026-07-01T09:00:00Z',
		reinforcementCount: 2
	},
	{
		factID: 'fact-2',
		episodeID: 'episode-2',
		scopeType: 'private',
		scopeID: 'person-1',
		kind: 'temporary',
		content: 'The user is away until Friday.',
		validFrom: '2026-07-03T09:00:00Z',
		validUntil: '2026-07-11T00:00:00Z',
		reinforcementCount: 1
	},
	{
		factID: 'fact-3',
		episodeID: 'episode-3',
		scopeType: 'circle',
		scopeID: 'hr',
		kind: 'fact',
		content: 'Compensation data belongs to HR.',
		validFrom: '2026-07-05T09:00:00Z',
		reinforcementCount: 1
	},
	{
		factID: 'fact-4',
		episodeID: 'episode-4',
		scopeType: 'workspace',
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
		const filtered = filterMemoryFacts(facts, { searchText: '', kind: 'temporary', scope: allFilterValue });
		expect(filtered.map((fact) => fact.factID)).toEqual(['fact-2']);
	});

	test('filters by scope', () => {
		const filtered = filterMemoryFacts(facts, { searchText: '', kind: allFilterValue, scope: 'private' });
		expect(filtered.map((fact) => fact.factID)).toEqual(['fact-1', 'fact-2']);
	});

	test('search text matches content case-insensitively', () => {
		const filtered = filterMemoryFacts(facts, { searchText: 'COMPENSATION', kind: allFilterValue, scope: allFilterValue });
		expect(filtered.map((fact) => fact.factID)).toEqual(['fact-3']);
	});

	test('search text matches the circle', () => {
		const filtered = filterMemoryFacts(facts, { searchText: 'hr', kind: allFilterValue, scope: allFilterValue });
		expect(filtered.map((fact) => fact.factID)).toEqual(['fact-3']);
	});

	test('combines filters', () => {
		const filtered = filterMemoryFacts(facts, { searchText: 'user', kind: 'preference', scope: 'private' });
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

	test('memoryFactScopesOf returns distinct sorted scopes', () => {
		expect(memoryFactScopesOf(facts)).toEqual(['circle', 'private', 'workspace']);
	});
});

describe('isExpiringFact', () => {
	test('is true only for temporary facts with an expiry', () => {
		expect(isExpiringFact(facts[1])).toBe(true);
		expect(isExpiringFact(facts[0])).toBe(false);
	});
});
