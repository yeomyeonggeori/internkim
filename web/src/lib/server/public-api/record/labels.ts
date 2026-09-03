import { vocabularyOf } from '$lib/task/task-vocabulary';

export class LabelUnresolved extends Error {
	readonly errorCode: string;
	readonly failureStage = 'input_validation';
	readonly retryable = true;
	readonly safeRetry = true;

	constructor(
		readonly asked: string,
		readonly registered: string[],
		readonly outcome: 'ambiguous' | 'unregistered'
	) {
		super(
			outcome === 'ambiguous'
				? `${asked} matches more than one registered label`
				: `${asked} is not one of ${registered.join(', ')}`
		);
		this.name = 'LabelUnresolved';
		this.errorCode = outcome === 'ambiguous' ? 'interaction_required' : 'task_label_unregistered';
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
	if (registered.length === 0) return written;

	const exact = registered.find((label) => label.toLowerCase() === written.toLowerCase());
	if (exact) return exact;

	const contained = registered.filter((label) =>
		label.toLowerCase().includes(written.toLowerCase())
	);
	if (contained.length === 1) return contained[0];
	throw new LabelUnresolved(asked, registered, contained.length > 1 ? 'ambiguous' : 'unregistered');
}
