import type { SelectablePerson } from '$lib/components/person-multi-select.svelte';
import { fetchPeople } from './messenger-api';
import { fetchMessengerDirectory } from './messenger-directory';

export type ChannelCandidate = SelectablePerson & { externalID: string };

export async function fetchChannelCandidates(): Promise<ChannelCandidate[]> {
	const [people, directory] = await Promise.all([fetchPeople(), fetchMessengerDirectory()]);
	const emailOfMember = new Map([...directory.memberOfEmail].map(([email, memberID]) => [memberID, email]));
	return people
		.flatMap((person) => {
			const memberID = directory.memberOfExternal.get(person.externalID);
			if (!memberID) return [];
			return [
				{
					memberID,
					externalID: person.externalID,
					name: directory.nameOfMember.get(memberID) || person.name,
					email: emailOfMember.get(memberID) ?? '',
					image: person.avatarURL
				}
			];
		})
		.sort((left, right) => left.name.localeCompare(right.name));
}
