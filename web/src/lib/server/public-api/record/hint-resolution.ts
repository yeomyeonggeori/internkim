import { Refusal } from '../refusal';

const approximateCandidateLimit = 8;

export type HintOutcome = 'resolved' | 'ambiguous' | 'approximate' | 'not_found';

export type HintMatcher<Item> = {
	identifiersOf: (item: Item) => string[];
	reachOf: (item: Item, asked: string) => number;
	nearnessTo: (item: Item, hint: string) => number;
	isPreferred?: (item: Item) => boolean;
};

const titleRank = { unreached: 0, exact: 1, contained: 2 } as const;

export function titleReachOf<Item>(titleOf: (item: Item) => string): (item: Item, asked: string) => number {
	return (item, asked) => {
		const title = titleOf(item);
		if (normalized(title) === normalized(asked)) return titleRank.exact;
		if (titleContains(title, asked)) return titleRank.contained;
		return titleRank.unreached;
	};
}

export type HintResolution<Item> =
	| { outcome: 'resolved'; match: Item }
	| { outcome: 'ambiguous' | 'approximate' | 'not_found'; candidates: Item[] };

export function resolveHint<Item>(
	hint: string,
	items: Item[],
	matcher: HintMatcher<Item>
): HintResolution<Item> {
	const asked = hint.trim();
	if (asked === '') return { outcome: 'not_found', candidates: [] };

	const identified = items.filter((item) => hasIdentifier(matcher.identifiersOf(item), asked));
	if (identified.length === 1) return { outcome: 'resolved', match: identified[0] };
	if (identified.length > 1) return { outcome: 'ambiguous', candidates: identified };

	const closest = itemsClosestTo(asked, items, matcher);
	const closestResolution = onlyOrPreferred(closest, matcher.isPreferred);
	if (closestResolution) return closestResolution;
	if (closest.length > 0) return { outcome: 'ambiguous', candidates: closest };

	const nearest = nearestItems(asked, items, matcher);
	if (nearest.length > 0) return { outcome: 'approximate', candidates: nearest };
	return { outcome: 'not_found', candidates: [] };
}

function itemsClosestTo<Item>(asked: string, items: Item[], matcher: HintMatcher<Item>): Item[] {
	const ranked = items
		.map((item) => ({ item, rank: matcher.reachOf(item, asked) }))
		.filter((entry) => entry.rank > 0);
	if (ranked.length === 0) return [];
	const closestRank = Math.min(...ranked.map((entry) => entry.rank));
	return ranked.filter((entry) => entry.rank === closestRank).map((entry) => entry.item);
}

function onlyOrPreferred<Item>(
	items: Item[],
	isPreferred: ((item: Item) => boolean) | undefined
): { outcome: 'resolved'; match: Item } | null {
	if (items.length === 1) return { outcome: 'resolved', match: items[0] };
	if (items.length === 0 || !isPreferred) return null;
	const preferred = items.filter(isPreferred);
	if (preferred.length !== 1) return null;
	return { outcome: 'resolved', match: preferred[0] };
}

function nearestItems<Item>(hint: string, items: Item[], matcher: HintMatcher<Item>): Item[] {
	return items
		.map((item, position) => ({ item, nearness: matcher.nearnessTo(item, hint), position }))
		.filter((scored) => scored.nearness > 0)
		.sort((first, second) => second.nearness - first.nearness || first.position - second.position)
		.slice(0, approximateCandidateLimit)
		.map((scored) => scored.item);
}

function hasIdentifier(identifiers: string[], asked: string): boolean {
	return identifiers.some((identifier) => identifier !== '' && normalized(identifier) === normalized(asked));
}

function titleContains(title: string, asked: string): boolean {
	const collapsedAsked = collapseWhitespace(normalized(asked));
	if (collapsedAsked === '') return false;
	return collapseWhitespace(normalized(title)).includes(collapsedAsked);
}

export function normalized(value: string): string {
	return value.trim().toLowerCase();
}

function collapseWhitespace(text: string): string {
	return text.split(/\s+/).filter(Boolean).join(' ');
}

const notFoundCodes = {
	holiday: 'company_holiday_not_found',
	person: 'person_not_found',
	participant: 'task_participant_not_found',
	supervisor: 'supervisor_not_found',
	task: 'task_not_found',
	event: 'calendar_event_not_found',
	organization: 'organization_not_found',
	crmOrganization: 'crm_organization_not_found',
	crmContact: 'crm_contact_not_found',
	crmOpportunity: 'crm_opportunity_not_found',
	record: 'company_record_not_found',
	document: 'company_document_not_found',
	equityEntry: 'company_equity_entry_not_found'
} as const;

export type HintSubject = keyof typeof notFoundCodes;

const subjectNouns: Partial<Record<HintSubject, string>> = {
	crmOrganization: 'organization',
	crmContact: 'contact',
	crmOpportunity: 'deal',
	equityEntry: 'equity line'
};

export type HintCandidate = {
	id: string;
	label: string;
	email?: string;
	handle?: string;
};

export class HintRefused extends Refusal {
	readonly status = 409;
	readonly errorCode: string;
	readonly failureStage = 'target_resolution';
	readonly retryable = true;
	readonly safeRetry = true;

	constructor(
		readonly subject: HintSubject,
		readonly hint: string,
		readonly outcome: 'ambiguous' | 'approximate' | 'not_found',
		readonly candidates: HintCandidate[]
	) {
		super(refusalOf(subject, hint, outcome));
		this.errorCode = outcome === 'not_found' ? notFoundCodes[subject] : 'interaction_required';
	}

	get details() {
		return { hint: this.hint, candidates: this.candidates };
	}
}

function refusalOf(subject: HintSubject, hint: string, outcome: HintOutcome): string {
	const noun = subjectNouns[subject] ?? subject;
	if (outcome === 'ambiguous') return `${hint} matches more than one ${noun}`;
	if (outcome === 'approximate') return `no ${noun} is called ${hint}; these are the nearest`;
	return `no ${noun} matches ${hint}`;
}
