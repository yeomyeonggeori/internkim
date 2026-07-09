import { describe, expect, test } from 'bun:test';
import type { MemoryGraphEpisode, MemoryGraphFact } from '../../../src/routes/memory/memory-graph-api';
import {
	allFilterValue,
	emptyMemoryFactFilters,
	episodeForFact,
	filterMemoryFacts,
	memoryFactScopes,
	memoryFactSourceKinds,
	personalScopeFilterValue,
	sortMemoryFactsByRecency
} from '../../../src/routes/memory/memory-fact-list-model';

const facts: MemoryGraphFact[] = [
	{
		factID: 'fact-1',
		scopeType: 'user',
		namespaceID: 'user:person-1',
		content: 'The user prefers terse release notes.',
		sourceKind: 'fact',
		sourceEpisodeID: 'episode-1',
		validAt: '2026-07-01T09:00:00Z'
	},
	{
		factID: 'fact-2',
		scopeType: 'private',
		namespaceID: 'private:person-1',
		content: '# Memory\n- Pinned personal memory.',
		sourceKind: 'pinned'
	},
	{
		factID: 'fact-3',
		scopeType: 'circle',
		namespaceID: 'circle:default:staff',
		content: 'Compensation data belongs to HR.',
		sourceKind: 'fact',
		validAt: '2026-07-05T09:00:00Z'
	},
	{
		factID: 'fact-4',
		scopeType: 'workspace',
		namespaceID: 'workspace:default',
		content: 'Quarterly launch review happens every Friday.',
		sourceKind: 'node',
		validAt: '2026-06-20T09:00:00Z'
	}
];

describe('filterMemoryFacts', () => {
	test('returns all facts with empty filters', () => {
		expect(filterMemoryFacts(facts, emptyMemoryFactFilters()).length).toBe(4);
	});

	test('filters by source kind', () => {
		const filtered = filterMemoryFacts(facts, { searchText: '', sourceKind: 'pinned', scope: allFilterValue });
		expect(filtered.map((fact) => fact.factID)).toEqual(['fact-2']);
	});

	test('personal scope filter matches user and private scopes', () => {
		const filtered = filterMemoryFacts(facts, {
			searchText: '',
			sourceKind: allFilterValue,
			scope: personalScopeFilterValue
		});
		expect(filtered.map((fact) => fact.factID)).toEqual(['fact-1', 'fact-2']);
	});

	test('filters by explicit scope', () => {
		const filtered = filterMemoryFacts(facts, { searchText: '', sourceKind: allFilterValue, scope: 'circle' });
		expect(filtered.map((fact) => fact.factID)).toEqual(['fact-3']);
	});

	test('search text matches content case-insensitively', () => {
		const filtered = filterMemoryFacts(facts, { searchText: 'COMPENSATION', sourceKind: allFilterValue, scope: allFilterValue });
		expect(filtered.map((fact) => fact.factID)).toEqual(['fact-3']);
	});

	test('search text matches namespace', () => {
		const filtered = filterMemoryFacts(facts, { searchText: 'workspace:default', sourceKind: allFilterValue, scope: allFilterValue });
		expect(filtered.map((fact) => fact.factID)).toEqual(['fact-4']);
	});

	test('combines filters', () => {
		const filtered = filterMemoryFacts(facts, { searchText: 'memory', sourceKind: 'pinned', scope: personalScopeFilterValue });
		expect(filtered.map((fact) => fact.factID)).toEqual(['fact-2']);
	});
});

describe('sortMemoryFactsByRecency', () => {
	test('sorts by validAt descending with missing dates last', () => {
		expect(sortMemoryFactsByRecency(facts).map((fact) => fact.factID)).toEqual(['fact-3', 'fact-1', 'fact-4', 'fact-2']);
	});

	test('does not mutate the input array', () => {
		const input = [...facts];
		sortMemoryFactsByRecency(input);
		expect(input.map((fact) => fact.factID)).toEqual(['fact-1', 'fact-2', 'fact-3', 'fact-4']);
	});
});

describe('filter options', () => {
	test('memoryFactSourceKinds returns distinct sorted kinds', () => {
		expect(memoryFactSourceKinds(facts)).toEqual(['fact', 'node', 'pinned']);
	});

	test('memoryFactScopes folds personal scopes into one value', () => {
		expect(memoryFactScopes(facts)).toEqual(['circle', personalScopeFilterValue, 'workspace']);
	});
});

describe('episodeForFact', () => {
	const episodes: MemoryGraphEpisode[] = [{ episodeID: 'episode-1', namespaceIDs: ['user:person-1'] }];

	test('finds the source episode', () => {
		expect(episodeForFact(episodes, facts[0])?.episodeID).toBe('episode-1');
	});

	test('returns undefined without a source episode', () => {
		expect(episodeForFact(episodes, facts[1])).toBe(undefined);
	});
});
