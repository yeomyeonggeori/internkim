import type { SelectablePerson } from '$lib/components/person-multi-select.svelte';
import { fetchChannels, fetchPeople, type MessengerDirectoryPerson } from './messenger-api';
import { fetchMessengerDirectory, type MessengerDirectory } from './messenger-directory';

export type ChannelCandidate = SelectablePerson & { externalID: string };

export type ChannelAgent = { externalID: string; name: string };

export async function fetchChannelCandidates(agentName: string): Promise<ChannelCandidate[]> {
	const [people, directory, channels] = await Promise.all([fetchPeople(), fetchMessengerDirectory(), fetchChannels()]);
	const agent = channels.agentExternalID ? { externalID: channels.agentExternalID, name: agentName } : undefined;
	return channelCandidatesOf(people, directory, agent);
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
