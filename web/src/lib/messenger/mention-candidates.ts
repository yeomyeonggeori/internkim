export type MentionPerson = {
	externalID: string;
	name: string;
	avatarURL?: string;
};

export type MentionCandidate = {
	key: string;
	label: string;
	person?: MentionPerson;
	isEveryone: boolean;
};

export const everyoneKey = 'everyone';
export const everyoneLabel = 'all';

export function mentionCandidates(people: MentionPerson[], isGroup: boolean): MentionCandidate[] {
	const named = people
		.filter((person) => person.externalID && person.name.trim() !== '')
		.map((person) => ({ key: person.externalID, label: person.name, person, isEveryone: false }));
	if (!isGroup) return named;
	return [{ key: everyoneKey, label: everyoneLabel, isEveryone: true }, ...named];
}

export function matchingMentions(
	candidates: MentionCandidate[],
	query: string
): MentionCandidate[] {
	const wanted = query.trim().toLowerCase();
	if (wanted === '') return candidates;
	return candidates
		.map((candidate, order) => ({ candidate, order, rank: rankOf(candidate.label, wanted) }))
		.filter((scored) => scored.rank > 0)
		.sort((left, right) => right.rank - left.rank || left.order - right.order)
		.map((scored) => scored.candidate);
}

function rankOf(label: string, wanted: string): number {
	const lowered = label.toLowerCase();
	if (lowered.startsWith(wanted)) return 3;
	if (lowered.split(/\s+/).some((word) => word.startsWith(wanted))) return 2;
	if (lowered.includes(wanted)) return 1;
	return 0;
}

export function mentionPeopleOf(
	channel: { kind: 'dm' | 'group'; members?: { externalID?: string; name: string }[] } | undefined
): MentionPerson[] {
	if (!channel || channel.kind !== 'group') return [];
	return (channel.members ?? [])
		.filter((member) => member.externalID !== undefined && member.externalID !== '')
		.map((member) => ({ externalID: member.externalID as string, name: member.name }));
}
