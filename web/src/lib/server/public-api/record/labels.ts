import { vocabularyOf } from '$lib/task/task-vocabulary';

export class LabelUnresolved extends Error {
	constructor(
		readonly asked: string,
		readonly registered: string[]
	) {
		super(
			registered.length === 0
				? `this company registers no label to put ${asked} under`
				: `${asked} is not one of ${registered.join(', ')}`
		);
		this.name = 'LabelUnresolved';
	}
}

export type CompanyLabels = {
	businesses: string[];
	types: string[];
	timezone: string;
};

export function labelsOfVocabulary(vocabulary: unknown, timezone: string | null): CompanyLabels {
	const read = vocabularyOf(vocabulary);
	return {
		businesses: (read.businesses ?? []).map((business) => business.name),
		types: (read.types ?? []).map((type) => type.name),
		timezone: timezone?.trim() || 'Asia/Seoul'
	};
}

// A label a company never registered would answer a board filter nobody set up,
// so an unregistered one is refused with the list rather than written.
export function labelOf(registered: string[], asked: string | undefined, fallback: string | null): string | null {
	if (asked === undefined) return fallback;
	const written = asked.trim();
	if (!written) return null;

	const exact = registered.find((label) => label.toLowerCase() === written.toLowerCase());
	if (exact) return exact;

	const contained = registered.filter((label) => label.includes(written));
	if (contained.length === 1) return contained[0];
	throw new LabelUnresolved(asked, registered);
}

export function firstRegistered(registered: string[]): string | null {
	return registered[0] ?? null;
}
