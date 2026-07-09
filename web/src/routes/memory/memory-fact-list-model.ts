import type { MemoryGraphEpisode, MemoryGraphFact } from './memory-graph-api';
import { isPersonalScope } from './memory-graph-selection';

export const allFilterValue = 'all';
export const personalScopeFilterValue = 'personal';

export type MemoryFactFilters = {
	searchText: string;
	sourceKind: string;
	scope: string;
};

export function emptyMemoryFactFilters(): MemoryFactFilters {
	return { searchText: '', sourceKind: allFilterValue, scope: allFilterValue };
}

export function filterMemoryFacts(facts: MemoryGraphFact[], filters: MemoryFactFilters): MemoryGraphFact[] {
	const searchText = filters.searchText.trim().toLowerCase();
	return facts.filter(
		(fact) =>
			matchesSourceKind(fact, filters.sourceKind) &&
			matchesScope(fact, filters.scope) &&
			matchesSearchText(fact, searchText)
	);
}

export function sortMemoryFactsByRecency(facts: MemoryGraphFact[]): MemoryGraphFact[] {
	return [...facts].sort((left, right) => (right.validAt ?? '').localeCompare(left.validAt ?? ''));
}

export function memoryFactSourceKinds(facts: MemoryGraphFact[]): string[] {
	const sourceKinds = new Set<string>();
	for (const fact of facts) {
		if (fact.sourceKind) sourceKinds.add(fact.sourceKind);
	}
	return [...sourceKinds].sort();
}

export function memoryFactScopes(facts: MemoryGraphFact[]): string[] {
	const scopes = new Set<string>();
	for (const fact of facts) {
		scopes.add(isPersonalScope(fact.scopeType) ? personalScopeFilterValue : fact.scopeType);
	}
	return [...scopes].sort();
}

export function episodeForFact(
	episodes: MemoryGraphEpisode[],
	fact: MemoryGraphFact
): MemoryGraphEpisode | undefined {
	if (!fact.sourceEpisodeID) return undefined;
	return episodes.find((episode) => episode.episodeID === fact.sourceEpisodeID);
}

function matchesSourceKind(fact: MemoryGraphFact, sourceKind: string): boolean {
	return sourceKind === allFilterValue || (fact.sourceKind ?? '') === sourceKind;
}

function matchesScope(fact: MemoryGraphFact, scope: string): boolean {
	if (scope === allFilterValue) return true;
	if (scope === personalScopeFilterValue) return isPersonalScope(fact.scopeType);
	return fact.scopeType === scope;
}

function matchesSearchText(fact: MemoryGraphFact, searchText: string): boolean {
	if (!searchText) return true;
	return fact.content.toLowerCase().includes(searchText) || fact.namespaceID.toLowerCase().includes(searchText);
}
