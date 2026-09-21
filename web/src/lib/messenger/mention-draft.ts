export type MentionFragment = {
	start: number;
	query: string;
};

export type ChosenMention = {
	key: string;
	label: string;
	externalID?: string;
	isEveryone: boolean;
};

export type DraftMentions = {
	externalIDs: string[];
	isEveryone: boolean;
};

export function mentionFragmentAt(text: string, cursor: number): MentionFragment | undefined {
	const before = text.slice(0, cursor);
	const start = before.lastIndexOf('@');
	if (start < 0) return undefined;
	if (start > 0 && !/\s/.test(before[start - 1])) return undefined;
	const query = before.slice(start + 1);
	if (/\s/.test(query)) return undefined;
	return { start, query };
}

export function textWithMention(
	text: string,
	cursor: number,
	fragment: MentionFragment,
	label: string
): { text: string; cursor: number } {
	const head = text.slice(0, fragment.start);
	const written = `@${label} `;
	return { text: head + written + text.slice(cursor), cursor: head.length + written.length };
}

export function mentionsToSend(text: string, chosen: ChosenMention[]): DraftMentions {
	const kept = chosen.filter((mention) => text.includes(`@${mention.label}`));
	const externalIDs = kept
		.filter((mention) => !mention.isEveryone)
		.map((mention) => mention.externalID)
		.filter((externalID): externalID is string => externalID !== undefined);
	return { externalIDs: [...new Set(externalIDs)], isEveryone: kept.some((mention) => mention.isEveryone) };
}

export type MentionKeyAction = 'down' | 'up' | 'take' | 'close';

export function mentionKeyAction(pressed: string, isComposing: boolean): MentionKeyAction | undefined {
	if (isComposing) return undefined;
	if (pressed === 'ArrowDown') return 'down';
	if (pressed === 'ArrowUp') return 'up';
	if (pressed === 'Enter' || pressed === 'Tab') return 'take';
	if (pressed === 'Escape') return 'close';
	return undefined;
}
