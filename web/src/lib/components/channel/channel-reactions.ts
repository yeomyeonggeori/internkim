import type { ChannelMessage, ChannelMessageReaction, ChannelParticipant } from './channel-api';

export function reactionValueFor(pickedGlyph: string, existing: ChannelMessageReaction[]): string {
	const match = existing.find((reaction) => reaction.emoji === pickedGlyph && !reaction.imageURL);
	return match ? match.value : pickedGlyph;
}

export function reactionPeopleLabel(
	names: string[],
	templates: { reactedBy: string; reactedByMore: string },
	shown = 3
): string {
	if (names.length === 0) return '';
	if (names.length <= shown) {
		return templates.reactedBy.replace('{names}', names.join(', '));
	}
	const remainingCount = names.length - shown;
	return templates.reactedByMore
		.replace('{names}', names.slice(0, shown).join(', '))
		.replace('{count}', String(remainingCount));
}

export type ReactionChange = {
	value: string;
	glyph: string;
	isAdding: boolean;
	person: ChannelParticipant;
};

export function reactionsAfter(
	reactions: ChannelMessageReaction[],
	change: ReactionChange
): ChannelMessageReaction[] {
	const existing = reactions.find((reaction) => reaction.value === change.value);
	if (!existing) {
		if (!change.isAdding) return reactions;
		return [
			...reactions,
			{ emoji: change.glyph, value: change.value, count: 1, reactedByMe: true, people: [change.person] }
		];
	}
	if (change.isAdding === (existing.reactedByMe ?? false)) return reactions;
	const people = (existing.people ?? []).filter((person) => person.id !== change.person.id);
	const updated: ChannelMessageReaction = {
		...existing,
		count: Math.max(existing.count + (change.isAdding ? 1 : -1), 0),
		reactedByMe: change.isAdding,
		people: change.isAdding ? [...people, change.person] : people
	};
	return reactions
		.map((reaction) => (reaction.value === change.value ? updated : reaction))
		.filter((reaction) => reaction.count > 0);
}

export function messagesWithReactions(
	messages: ChannelMessage[],
	messageID: string,
	reactions: ChannelMessageReaction[]
): ChannelMessage[] {
	return messages.map((message) => (message.id === messageID ? { ...message, reactions } : message));
}

type ReactionAsSent = Omit<ChannelMessageReaction, 'value'> & { value?: string };

export function reactionsWithValues(reactions: ReactionAsSent[] | undefined): ChannelMessageReaction[] | undefined {
	return reactions?.map((reaction) => ({ ...reaction, value: reaction.value ?? reaction.emoji }));
}
