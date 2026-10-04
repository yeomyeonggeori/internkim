import type { Circle } from '$lib/data-room/model';
import type { MemoryFact, MemoryLayer } from './memory-facts-api';
import type { MemoryText } from './text';

export function isCurrentMemory(fact: MemoryFact, now = Date.now()): boolean {
	if (fact.coldSince) return false;
	return !fact.validUntil || Date.parse(fact.validUntil) > now;
}

export function filterMemoryFacts(facts: MemoryFact[], query: string, scopeLabel: (fact: MemoryFact) => string): MemoryFact[] {
	const needle = query.trim().toLocaleLowerCase();
	if (!needle) return facts;
	return facts.filter((fact) =>
		[fact.content, scopeLabel(fact), ...fact.triggerPhrases].some((value) => value.toLocaleLowerCase().includes(needle))
	);
}

export function memoryScopeLabel(layer: MemoryLayer, circles: Circle[], text: MemoryText, locale: string): string {
	if (layer.scopeType === 'person') return text.myMemory;
	if (layer.scopeType === 'workspace') return text.companyMemory;
	return circleName(layer.scopeID ?? '', circles, locale);
}

function circleName(circleID: string, circles: Circle[], locale: string): string {
	const circle = circles.find((candidate) => candidate.id === circleID);
	if (!circle) return circleID;
	return locale === 'ko' ? circle.nameKO || circle.name : circle.name;
}

export function memoryLayerKey(layer: MemoryLayer): string {
	return layer.scopeType === 'circle' ? `circle:${layer.scopeID ?? ''}` : layer.scopeType;
}

export const memoryLayerDepth: Record<MemoryLayer['scopeType'], number> = {
	person: 1,
	circle: 2,
	workspace: 3
};

export type MemoryLayerGroup = { layer: MemoryLayer; facts: MemoryFact[] };

export function groupFactsByLayer(layers: MemoryLayer[], facts: MemoryFact[]): MemoryLayerGroup[] {
	return layers
		.map((layer) => ({ layer, facts: facts.filter((fact) => memoryLayerKey(fact) === memoryLayerKey(layer)) }))
		.filter((group) => group.facts.length > 0);
}

export function memoryWhen(fact: MemoryFact, text: MemoryText, locale: string): string {
	if (fact.isStatic) return text.always;
	if (!fact.occurredAt) return text.happened;
	const start = memoryDate(fact.occurredAt, text, locale);
	const end = fact.occurredUntil ? memoryDate(fact.occurredUntil, text, locale) : start;
	return end === start ? start : `${start} → ${end}`;
}

export function memoryDate(value: string | undefined, text: MemoryText, locale = 'ko'): string {
	if (!value || !Number.isFinite(Date.parse(value))) return text.dateUnavailable;
	return new Intl.DateTimeFormat(locale, { dateStyle: 'medium' }).format(new Date(value));
}
