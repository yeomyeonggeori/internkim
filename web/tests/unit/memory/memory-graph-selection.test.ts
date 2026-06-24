import { describe, expect, test } from 'bun:test';
import { factsForMemoryGraphSelection } from '../../../src/routes/memory/memory-graph-selection';
import type { MemoryGraphFact, MemoryGraphNode } from '../../../src/routes/memory/memory-graph-api';

describe('memory graph selection', () => {
	test('shows facts extracted from the selected episode when source episode ids match', () => {
		const facts = [
			memoryFact('fact-1', 'workspace-memory', 'episode-1'),
			memoryFact('fact-2', 'workspace-memory', 'episode-2')
		];

		const selectedFacts = factsForMemoryGraphSelection(facts, episodeNode('episode-1'), {
			episodeID: 'episode-1',
			namespaceIDs: ['workspace-memory']
		});

		expect(selectedFacts.map((fact) => fact.factID)).toEqual(['fact-1']);
	});

	test('does not show unrelated namespace facts for old episode rows without source links', () => {
		const facts = [
			memoryFact('fact-1', 'workspace-memory'),
			memoryFact('fact-2', 'private-memory')
		];

		const selectedFacts = factsForMemoryGraphSelection(facts, episodeNode('episode-1'), {
			episodeID: 'episode-1',
			namespaceIDs: ['workspace-memory']
		});

		expect(selectedFacts).toEqual([]);
	});
});

function episodeNode(episodeID: string): MemoryGraphNode {
	return { nodeID: `episode:${episodeID}`, label: 'mattermost 05-06 03:37', kind: 'episode' };
}

function memoryFact(factID: string, namespaceID: string, sourceEpisodeID?: string): MemoryGraphFact {
	return {
		factID,
		scopeType: 'workspace',
		namespaceID,
		content: factID,
		...(sourceEpisodeID ? { sourceEpisodeID } : {})
	};
}
