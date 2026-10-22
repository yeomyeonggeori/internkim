import type { SelectablePerson } from '$lib/components/person-multi-select.svelte';
import type { MessengerDirectoryPerson } from './messenger-api';
import { fetchMessengerDirectory, type MessengerDirectory } from './messenger-directory';
import { fetchMessengerAccounts } from './messenger-accounts';

export type ChannelCandidate = SelectablePerson & { externalID: string };

export type ChannelAgent = { externalID: string; name: string };

export async function fetchChannelCandidates(agentName: string): Promise<ChannelCandidate[]> {
	const [accounts, directory] = await Promise.all([fetchMessengerAccounts(), fetchMessengerDirectory()]);
	const agent = accounts.agentExternalID ? { externalID: accounts.agentExternalID, name: agentName } : undefined;
	return channelCandidatesOf(accounts.people, directory, agent);
}

export function channelCandidatesOf(
	people: MessengerDirectoryPerson[],
	directory: MessengerDirectory,
	agent: ChannelAgent | undefined
): ChannelCandidate[] {
	const emailOfMember = new Map([...directory.memberOfEmail].map(([email, memberID]) => [memberID, email]));
	const members = people
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
	if (!agent) return members;
	return [{ memberID: agent.externalID, externalID: agent.externalID, name: agent.name, email: '' }, ...members];
}
