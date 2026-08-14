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
	webURL?: string;
};

export type MessengerReaction = {
	emoji: string;
	imageURL?: string;
	people: MessengerPerson[];
};

export type MessengerAttachment = {
	url: string;
	filename: string;
	contentType: string;
	sizeBytes: number;
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
	attachments: MessengerAttachment[];
};

export type LinkPreview = {
	url: string;
	title: string;
	description: string;
	siteName: string;
	imageURL: string;
};

type PersonalConversation = {
	id: string;
	name: string;
	kind: 'dm' | 'group';
	avatarURL?: string;
	participantExternalIDs?: string[];
	webURL?: string;
};
type PersonalReaction = { emoji: string; imageURL?: string; byExternalIDs: string[] };
type PersonalAttachment = {
	id: string;
	filename: string;
	contentType: string;
	sizeBytes: number;
};
type PersonalMessage = {
	id: string;
	conversationID: string;
	parentID?: string;
	authorExternalID: string;
	body: string;
	postedAt: string;
	editedAt?: string;
	reactions: PersonalReaction[];
	attachments: PersonalAttachment[];
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

export function asChannel(conversation: PersonalConversation, position: number): MessengerChannel {
	return {
		id: conversation.id,
		platform: 'mattermost',
		name: conversation.name,
		isDirect: conversation.kind === 'dm',
		position,
		participants: (conversation.participantExternalIDs ?? []).map((externalID) => ({ externalID })),
		webURL: conversation.webURL
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
			imageURL: reaction.imageURL,
			people: reaction.byExternalIDs.map((externalID) => ({ externalID }))
		})),
		attachments: (message.attachments ?? []).map((attachment) => ({
			url: attachment.id,
			filename: attachment.filename,
			contentType: attachment.contentType,
			sizeBytes: attachment.sizeBytes
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

export type OutgoingAttachment = {
	filename: string;
	contentType: string;
	contentBase64: string;
};

export async function writePost(
	channelID: string,
	body: string,
	parentID?: string,
	attachments: OutgoingAttachment[] = []
): Promise<MessengerPost> {
	const message = await ask<PersonalMessage>('person.message.send', {
		conversationID: channelID,
		body,
		parentID,
		attachments
	});
	return asPost(message);
}

export async function fetchCustomEmojiNames(): Promise<string[]> {
	const answer = await ask<{ emoji: { name: string }[] }>('person.emoji.list');
	return answer.emoji.map((emoji) => emoji.name);
}

export async function fetchCustomEmojiImage(name: string): Promise<{ dataURL: string } | null> {
	const answer = await ask<{ image: { dataURL: string } | null }>('person.emoji.image', { name });
	return answer.image;
}

export async function fetchProfilePicture(externalID: string): Promise<{ dataURL: string } | null> {
	const answer = await ask<{ image: { dataURL: string } | null }>('person.picture', { externalID });
	return answer.image;
}

export function fetchLinkPreview(url: string): Promise<LinkPreview | null> {
	return ask<LinkPreview | null>('asset.link', { url });
}
