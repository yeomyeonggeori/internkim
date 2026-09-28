import {
	everyoneLabel,
	matchingMentions,
	mentionCandidates,
	type MentionCandidate,
	type MentionPerson
} from './mention-candidates';
import {
	mentionFragmentAt,
	mentionsToSend,
	textWithMention,
	type ChosenMention,
	type DraftMentions,
	type MentionFragment
} from './mention-draft';

export type MentionPicker = ReturnType<typeof createMentionPicker>;

export function createMentionPicker(people: () => MentionPerson[], isGroup: () => boolean) {
	let rows = $state<MentionCandidate[]>([]);
	let active = $state(0);
	let fragment = $state<MentionFragment | undefined>(undefined);
	let chosen = $state<ChosenMention[]>([]);

	function close(): void {
		rows = [];
		active = 0;
		fragment = undefined;
	}

	return {
		get rows(): MentionCandidate[] {
			return rows;
		},
		get active(): number {
			return active;
		},
		get isOpen(): boolean {
			return rows.length > 0;
		},
		close,
		forget(): void {
			chosen = [];
			close();
		},
		reopen(text: string, cursor: number): void {
			const opening = mentionFragmentAt(text, cursor);
			if (!opening) return close();
			const matched = matchingMentions(mentionCandidates(people(), isGroup()), opening.query);
			if (matched.length === 0) return close();
			fragment = opening;
			rows = matched;
			active = 0;
		},
		moveBy(step: number): void {
			if (rows.length === 0) return;
			active = (active + step + rows.length) % rows.length;
		},
		take(
			text: string,
			cursor: number,
			candidate?: MentionCandidate
		): { text: string; cursor: number } | undefined {
			const taken = candidate ?? rows[active];
			if (!taken || !fragment) return undefined;
			const written = textWithMention(text, cursor, fragment, taken.label);
			chosen = [...chosen, asChosen(taken)];
			close();
			return written;
		},
		mentionsIn(text: string): DraftMentions {
			return mentionsToSend(text, chosen);
		}
	};
}

export function mentionLabelsOf(
	mentions: DraftMentions | undefined,
	nameOf: (externalID: string) => string | undefined
): string[] {
	if (!mentions) return [];
	const named = mentions.externalIDs
		.map((externalID) => nameOf(externalID))
		.filter((name): name is string => name !== undefined && name.trim() !== '');
	return mentions.isEveryone ? [everyoneLabel, ...named] : named;
}

function asChosen(candidate: MentionCandidate): ChosenMention {
	return {
		key: candidate.key,
		label: candidate.label,
		externalID: candidate.isEveryone ? undefined : candidate.person?.externalID,
		isEveryone: candidate.isEveryone
	};
}
