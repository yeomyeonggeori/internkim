import type { MemoryGraphEpisode, MemoryGraphFact } from './memory-graph-api';
import { isPersonalScope } from './memory-graph-selection';
import type { MemoryText } from './text';

export function memoryFactKey(fact: MemoryGraphFact): string {
	return JSON.stringify([fact.namespaceID, fact.factID]);
}

export function isCurrentMemoryFact(fact: MemoryGraphFact, now = Date.now()): boolean {
	if (fact.validAt && Date.parse(fact.validAt) > now) return false;
	return ![fact.invalidAt, fact.expiredAt].some((value) => value && Date.parse(value) <= now);
}

export function memoryAudience(fact: MemoryGraphFact, text: MemoryText): string {
	if (isPersonalScope(fact.scopeType)) return text.myMemory;
	if (fact.scopeType === 'circle') return text.factScopeCircle;
	if (fact.scopeType === 'conversation') return text.factScopeConversation;
	return text.factScopeWorkspace;
}

export function memoryDate(value: string | undefined, text: MemoryText, locale = 'ko'): string {
	if (!value || !Number.isFinite(Date.parse(value))) return text.dateUnavailable;
	return new Intl.DateTimeFormat(locale, { dateStyle: 'medium' }).format(new Date(value));
}

export function memorySources(fact: MemoryGraphFact, episodes: MemoryGraphEpisode[]): MemoryGraphEpisode[] {
	const sourceIDs = new Set(fact.sourceEpisodeIDs ?? (fact.sourceEpisodeID ? [fact.sourceEpisodeID] : []));
	return episodes.filter((episode) => sourceIDs.has(episode.episodeID));
}
