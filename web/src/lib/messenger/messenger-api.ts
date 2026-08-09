import { callCompanyApp } from '$lib/host-bridge';

export type MessengerPerson = {
	memberID?: string;
	externalID?: string;
};

export type MessengerChannel = {
	id: string;
	platform: string;
	name: string;
	isDirect: boolean;
	position: number;
	participants: MessengerPerson[];
};

export type MessengerReaction = {
	emoji: string;
	people: MessengerPerson[];
};

export type MessengerPost = {
	id: string;
	channelID: string;
	parentID?: string;
	author: MessengerPerson;
	body: string;
	postedAt: string;
	editedAt?: string;
	reactions: MessengerReaction[];
};

export type LinkPreview = {
	url: string;
	title: string;
	description: string;
	siteName: string;
	imageDataURL: string;
};

type PersonalConversation = { id: string; name: string; kind: 'dm' | 'group'; avatarURL?: string };
type PersonalReaction = { emoji: string; byExternalIDs: string[] };
type PersonalMessage = {
	id: string;
	conversationID: string;
	parentID?: string;
	authorExternalID: string;
	body: string;
	postedAt: string;
	editedAt?: string;
	reactions: PersonalReaction[];
};

async function ask<Value>(capability: string, body?: Record<string, unknown>): Promise<Value> {
	const answer = await callCompanyApp({ capability, body });
	if (answer.status >= 400) throw new Error(messageOf(answer.body, `the app answered ${answer.status}`));
	return answer.body as Value;
}

function messageOf(body: unknown, fallback: string): string {
	if (typeof body === 'object' && body !== null && typeof (body as { error?: unknown }).error === 'string') {
		return (body as { error: string }).error;
	}
	return fallback;
}

function asChannel(conversation: PersonalConversation, position: number): MessengerChannel {
	return {
		id: conversation.id,
		platform: 'mattermost',
		name: conversation.name,
		isDirect: conversation.kind === 'dm',
		position,
		participants: []
	};
}

function asPost(message: PersonalMessage): MessengerPost {
	return {
		id: message.id,
		channelID: message.conversationID,
		parentID: message.parentID,
		author: { externalID: message.authorExternalID },
		body: message.body,
		postedAt: message.postedAt,
		editedAt: message.editedAt,
		reactions: message.reactions.map((reaction) => ({
			emoji: reaction.emoji,
			people: reaction.byExternalIDs.map((externalID) => ({ externalID }))
		}))
	};
}

export async function fetchChannels(): Promise<MessengerChannel[]> {
	const answer = await ask<{ conversations: PersonalConversation[] }>('person.conversations.list');
	return answer.conversations.map(asChannel);
}

export async function fetchPeople(): Promise<{ externalID: string; name: string }[]> {
	const answer = await ask<{ people: { externalID: string; name: string }[] }>('person.people.list');
	return answer.people;
}

export async function openDirectChannel(externalIDs: string[]): Promise<MessengerChannel> {
	const conversation = await ask<PersonalConversation>('person.dm.ensure', {
		counterpartExternalIDs: externalIDs
	});
	return asChannel(conversation, 0);
}

export async function fetchPosts(channelID: string, before?: string): Promise<MessengerPost[]> {
	const answer = await ask<{ messages: PersonalMessage[] }>('person.messages.list', {
		conversationID: channelID,
		before
	});
	return answer.messages.map(asPost);
}

export async function writePost(
	channelID: string,
	body: string,
	parentID?: string
): Promise<MessengerPost> {
	const message = await ask<PersonalMessage>('person.message.send', {
		conversationID: channelID,
		body,
		parentID
	});
	return asPost(message);
}

export function fetchCustomEmoji(): Promise<{ name: string; url: string }[]> {
	return ask<{ name: string; url: string }[]>('asset.emoji');
}

export function fetchProfilePicture(externalID: string): Promise<{ dataURL: string } | null> {
	return ask<{ dataURL: string } | null>('asset.picture', { externalID });
}

export function fetchLinkPreview(url: string): Promise<LinkPreview | null> {
	return ask<LinkPreview | null>('asset.link', { url });
}
