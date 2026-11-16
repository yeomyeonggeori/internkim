import type { MentionPerson } from '$lib/messenger/mention-candidates';
import type { Typist } from '$lib/messenger/typing-signal';
import type { ChannelMessage, ChannelParticipant } from './channel-api';

export function typingPeople(
	typerExternalIDs: string[],
	messages: ChannelMessage[],
	participants: MentionPerson[]
): ChannelParticipant[] {
	return typerExternalIDs.map(
		(externalID) => latestSenderOf(externalID, messages) ?? participantOf(externalID, participants)
	);
}

function latestSenderOf(externalID: string, messages: ChannelMessage[]): ChannelParticipant | undefined {
	return messages.findLast((message) => message.sender.externalID === externalID)?.sender;
}

function participantOf(externalID: string, participants: MentionPerson[]): ChannelParticipant {
	const person = participants.find((participant) => participant.externalID === externalID);
	return {
		id: externalID,
		name: person?.name ?? '',
		avatarURL: person?.avatarURL,
		memberID: person?.memberID,
		externalID
	};
}

export function typistsOf(people: ChannelParticipant[], agentExternalID: string | undefined): Typist[] {
	return people.map((person) => ({
		name: person.name,
		isAgent: person.externalID !== undefined && person.externalID === agentExternalID
	}));
}
