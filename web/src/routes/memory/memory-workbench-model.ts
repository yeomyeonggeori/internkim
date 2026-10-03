import type { Circle } from '$lib/data-room/model';
import type { MemoryFact } from './memory-facts-api';
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

export function memoryScopeLabel(fact: MemoryFact, circles: Circle[], text: MemoryText, locale: string): string {
	if (fact.scopeType === 'person') return text.myMemory;
	if (fact.scopeType === 'workspace') return text.companyMemory;
	const circle = circles.find((candidate) => candidate.id === fact.scopeID);
	const name = circle ? (locale === 'ko' ? circle.nameKO || circle.name : circle.name) : fact.scopeID;
	return `${text.circleMemory} · ${name ?? ''}`;
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
