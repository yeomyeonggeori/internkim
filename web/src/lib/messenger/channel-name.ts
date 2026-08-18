import { personLabel, type MessengerDirectory } from './messenger-directory';
import type { Locale } from '$lib/i18n/locale.svelte';
import type { MessengerChannel, MessengerPerson } from './messenger-api';

// A direct conversation is called after the person on the other side, and the
// company's own record is what that person is called here: the messenger holds
// a name in whatever order it was typed into it, so 김여명 arrives as 여명 김.
// The messenger's name is the fallback, for somebody the record does not know.
//
// A conversation is never called after the person reading it, however few
// people are left in it once the viewer is taken out.
export function channelName(
	channel: MessengerChannel,
	people: MessengerDirectory,
	mine: string,
	canonicalKey: (person: MessengerPerson, people: MessengerDirectory) => string,
	locale: Locale
): string {
	if (!channel.isDirect) return channel.name;
	const others = channel.participants.filter((person) => canonicalKey(person, people) !== mine);
	const asRecorded = others.map((person) => personLabel(person, people, locale)).filter(Boolean).join(', ');
	return asRecorded || channel.name;
}
