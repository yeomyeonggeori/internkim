import type { MemoryFact } from './memory-facts-api';

export const allFilterValue = 'all';
export const ownOnlyFilterValue = 'own';

export type MemoryFactFilters = {
	searchText: string;
	kind: string;
	circle: string;
};

export function emptyMemoryFactFilters(): MemoryFactFilters {
	return { searchText: '', kind: allFilterValue, circle: allFilterValue };
}

export function filterMemoryFacts(facts: MemoryFact[], filters: MemoryFactFilters): MemoryFact[] {
	const searchText = filters.searchText.trim().toLowerCase();
	return facts.filter(
		(fact) => matchesKind(fact, filters.kind) && matchesCircle(fact, filters.circle) && matchesSearchText(fact, searchText)
	);
}

export function sortMemoryFactsByRecency(facts: MemoryFact[]): MemoryFact[] {
	return [...facts].sort((left, right) => right.validFrom.localeCompare(left.validFrom));
}

export function memoryFactKindsOf(facts: MemoryFact[]): string[] {
	return [...new Set(facts.map((fact) => fact.kind))].sort();
}

export function memoryFactCirclesOf(facts: MemoryFact[]): string[] {
	return [...new Set(facts.flatMap((fact) => fact.circleIDs))].sort();
}

export function isOwnOnlyFact(fact: MemoryFact): boolean {
	return fact.circleIDs.length === 0;
}

export function isExpiringFact(fact: MemoryFact): boolean {
	return fact.kind === 'temporary' && Boolean(fact.validUntil);
}

function matchesKind(fact: MemoryFact, kind: string): boolean {
	return kind === allFilterValue || fact.kind === kind;
}

function matchesCircle(fact: MemoryFact, circle: string): boolean {
	if (circle === allFilterValue) return true;
	if (circle === ownOnlyFilterValue) return isOwnOnlyFact(fact);
	return fact.circleIDs.includes(circle);
}

function matchesSearchText(fact: MemoryFact, searchText: string): boolean {
	if (!searchText) return true;
	return fact.content.toLowerCase().includes(searchText) || fact.circleIDs.some((circleID) => circleID.toLowerCase().includes(searchText));
}
