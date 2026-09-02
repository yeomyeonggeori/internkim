import type { MemoryFact } from './memory-facts-api';

export const allFilterValue = 'all';

export type MemoryFactFilters = {
	searchText: string;
	kind: string;
	scope: string;
};

export function emptyMemoryFactFilters(): MemoryFactFilters {
	return { searchText: '', kind: allFilterValue, scope: allFilterValue };
}

export function filterMemoryFacts(facts: MemoryFact[], filters: MemoryFactFilters): MemoryFact[] {
	const searchText = filters.searchText.trim().toLowerCase();
	return facts.filter(
		(fact) => matchesKind(fact, filters.kind) && matchesScope(fact, filters.scope) && matchesSearchText(fact, searchText)
	);
}

export function sortMemoryFactsByRecency(facts: MemoryFact[]): MemoryFact[] {
	return [...facts].sort((left, right) => right.validFrom.localeCompare(left.validFrom));
}

export function memoryFactKindsOf(facts: MemoryFact[]): string[] {
	return [...new Set(facts.map((fact) => fact.kind))].sort();
}

export function memoryFactScopesOf(facts: MemoryFact[]): string[] {
	return [...new Set(facts.map((fact) => fact.scopeType))].sort();
}

export function isExpiringFact(fact: MemoryFact): boolean {
	return fact.kind === 'temporary' && Boolean(fact.validUntil);
}

function matchesKind(fact: MemoryFact, kind: string): boolean {
	return kind === allFilterValue || fact.kind === kind;
}

function matchesScope(fact: MemoryFact, scope: string): boolean {
	return scope === allFilterValue || fact.scopeType === scope;
}

function matchesSearchText(fact: MemoryFact, searchText: string): boolean {
	if (!searchText) return true;
	return fact.content.toLowerCase().includes(searchText) || (fact.scopeID ?? '').toLowerCase().includes(searchText);
}
