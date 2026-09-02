const approximateCandidateLimit = 8;

export type HintOutcome = 'resolved' | 'ambiguous' | 'approximate' | 'not_found';

export type HintMatcher<Item> = {
	identifiersOf: (item: Item) => string[];
	titleOf: (item: Item) => string;
	nearnessTo: (item: Item, hint: string) => number;
	isPreferred?: (item: Item) => boolean;
};

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

	const titled = items.filter((item) => normalized(matcher.titleOf(item)) === normalized(asked));
	const titledResolution = onlyOrPreferred(titled, matcher.isPreferred);
	if (titledResolution) return titledResolution;
	if (titled.length > 0) return { outcome: 'ambiguous', candidates: titled };

	const containing = items.filter((item) => titleContains(matcher.titleOf(item), asked));
	const containingResolution = onlyOrPreferred(containing, matcher.isPreferred);
	if (containingResolution) return containingResolution;
	if (containing.length > 0) return { outcome: 'ambiguous', candidates: containing };

	const nearest = nearestItems(asked, items, matcher);
	if (nearest.length > 0) return { outcome: 'approximate', candidates: nearest };
	return { outcome: 'not_found', candidates: [] };
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
	person: 'person_not_found',
	participant: 'task_participant_not_found',
	task: 'task_not_found',
	event: 'calendar_event_not_found'
} as const;

export type HintSubject = keyof typeof notFoundCodes;

export type HintCandidate = {
	id: string;
	label: string;
	email?: string;
	handle?: string;
};

export class HintRefused extends Error {
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
		this.name = 'HintRefused';
		this.errorCode = outcome === 'not_found' ? notFoundCodes[subject] : 'interaction_required';
	}
}

function refusalOf(subject: HintSubject, hint: string, outcome: HintOutcome): string {
	if (outcome === 'ambiguous') return `${hint} matches more than one ${subject}`;
	if (outcome === 'approximate') return `no ${subject} is called ${hint}; these are the nearest`;
	return `no ${subject} matches ${hint}`;
}
