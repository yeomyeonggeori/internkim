import type { MemoryFact, MemoryFactKind } from './memory-facts-api';
import type { MemoryText } from './text';

export function isLiveMemoryFact(fact: MemoryFact, now = Date.now()): boolean {
	if (Date.parse(fact.validFrom) > now) return false;
	return !fact.validUntil || Date.parse(fact.validUntil) > now;
}

export function filterMemoryFacts(facts: MemoryFact[], query: string): MemoryFact[] {
	const needle = query.trim().toLocaleLowerCase();
	if (!needle) return facts;
	return facts.filter((fact) => searchableMemoryText(fact).some((value) => value.toLocaleLowerCase().includes(needle)));
}

function searchableMemoryText(fact: MemoryFact): string[] {
	return [fact.content, fact.kind, ...fact.circleIDs, ...fact.triggerPhrases];
}

export function memoryAudience(fact: MemoryFact, text: MemoryText): string {
	if (fact.circleIDs.length === 0) return text.myMemory;
	return `${text.sharedWithCircle} · ${fact.circleIDs.join(', ')}`;
}

export function memoryKindLabel(kind: MemoryFactKind, text: MemoryText): string {
	const labels: Record<MemoryFactKind, string> = {
		identity: text.factKindIdentity,
		preference: text.factKindPreference,
		fact: text.factKindFact,
		episode: text.factKindEpisode,
		temporary: text.factKindTemporary
	};
	return labels[kind];
}

export function memoryDate(value: string | undefined, text: MemoryText, locale = 'ko'): string {
	if (!value || !Number.isFinite(Date.parse(value))) return text.dateUnavailable;
	return new Intl.DateTimeFormat(locale, { dateStyle: 'medium' }).format(new Date(value));
}
