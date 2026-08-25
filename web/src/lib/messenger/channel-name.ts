import { personLabel, type MessengerDirectory } from './messenger-directory';
import type { Locale } from '$lib/i18n/locale.svelte';
import type { MessengerChannel, MessengerPerson } from './messenger-api';

// A direct conversation is called after the person on the other side, and the
// company's own record is what that person is called here: the messenger holds
// a name in whatever order it was typed into it, so 김예시 arrives as 예시 김.
// The messenger's name is the fallback, for somebody the record does not know.
//
// A conversation is never called after the person reading it, however few
// people are left in it once the viewer is taken out.
export function channelName(
	channel: MessengerChannel,
	people: MessengerDirectory,
	mine: string,
	canonicalKey: (person: MessengerPerson, people: MessengerDirectory) => string,
	locale: Locale,
	agentName: string
): string {
	if (!channel.isDirect) return channel.name;
	// The agent is the product, not a person on the messenger. Its account is
	// called whatever was typed into that messenger's profile — "Intern Kim" —
	// while the product is 김인턴 to a Korean reader and internkim to an English
	// one. chatd says which conversation is its own; the name comes from here.
	if (channel.isWithTheAgent) return agentName;
	const others = channel.participants.filter((person) => canonicalKey(person, people) !== mine);
	const asRecorded = others.map((person) => personLabel(person, people, locale)).filter(Boolean).join(', ');
	return asRecorded || channel.name;
}
