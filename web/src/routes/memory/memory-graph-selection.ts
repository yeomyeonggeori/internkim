import type { MemoryGraphEpisode, MemoryGraphFact, MemoryGraphNode } from './memory-graph-api';

export const mergedPersonalNodeID = 'namespace:personal';

export function factsForMemoryGraphSelection(
	allFacts: MemoryGraphFact[],
	node: MemoryGraphNode | null,
	episode: MemoryGraphEpisode | null
): MemoryGraphFact[] {
	if (!node) return allFacts;
	if (node.kind === 'fact') {
		return allFacts.filter((fact) => `fact:${fact.namespaceID}:${fact.factID}` === node.nodeID);
	}
	if (node.kind === 'episode') {
		return factsForEpisode(allFacts, episode);
	}
	if (node.kind === 'namespace') {
		return factsForNamespace(allFacts, node);
	}
	return [];
}

export function isPersonalScope(scopeType: string): boolean {
	return scopeType === 'user' || scopeType === 'private';
}

function factsForNamespace(allFacts: MemoryGraphFact[], node: MemoryGraphNode): MemoryGraphFact[] {
	if (node.nodeID === mergedPersonalNodeID) {
		return allFacts.filter((fact) => isPersonalScope(fact.scopeType));
	}
	const namespaceID = node.nodeID.slice('namespace:'.length);
	return allFacts.filter((fact) => fact.namespaceID === namespaceID);
}

function factsForEpisode(allFacts: MemoryGraphFact[], episode: MemoryGraphEpisode | null): MemoryGraphFact[] {
	if (!episode) return [];
	return allFacts.filter((fact) => fact.sourceEpisodeID === episode.episodeID);
}
