import { channelText } from '$lib/i18n/channel-text';
import { currentLocale } from '$lib/i18n/locale.svelte';
import type { Locale } from '$lib/i18n/locale';
import { currentPersonNameLocale } from '$lib/person-name.svelte';
import { fetchMessengerAccounts } from './messenger-accounts';
import type { MessengerPeopleList, MessengerPerson } from './messenger-api';
import { fetchMessengerDirectory, personLabel, type MessengerDirectory } from './messenger-directory';

export type MessengerPeople = MessengerDirectory & { accounts: MessengerPeopleList };

const noAccounts: MessengerPeopleList = { people: [], agentExternalID: '' };

export async function fetchMessengerPeople(): Promise<MessengerPeople> {
	const [directory, accounts] = await Promise.all([
		fetchMessengerDirectory(),
		fetchMessengerAccounts().catch((failure: unknown) => {
			console.warn('the messenger did not answer with its people', failure);
			return noAccounts;
		})
	]);
	return { ...directory, accounts };
}

export function agentName(locale: Locale = currentLocale.value): string {
	return channelText[locale].title;
}

export function isTheAgent(person: MessengerPerson, accounts: MessengerPeopleList): boolean {
	return accounts.agentExternalID !== '' && person.externalID === accounts.agentExternalID;
}

export function messengerPersonLabel(person: MessengerPerson, people: MessengerPeople): string {
	const member = personLabel(person, people, currentPersonNameLocale());
	if (member) return member;
	if (isTheAgent(person, people.accounts)) return agentName();
	return people.accounts.people.find((account) => account.externalID === person.externalID)?.name ?? '';
}
