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
	isPrivate: boolean;
	position: number;
	participants: MessengerPerson[];
	isWithTheAgent?: boolean;
	webURL?: string;
	description?: string;
	roleOfExternalID?: Record<string, string>;
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
	digest: string;
	widthPixels?: number;
	heightPixels?: number;
};

export type KeptAttachment = {
	address: string;
	sizeBytes: number;
	digest: string;
	filename: string;
	contentType: string;
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
	description?: string;
	kind: 'dm' | 'group';
	isPrivate?: boolean;
	avatarURL?: string;
	participantExternalIDs?: string[];
	isWithTheAgent?: boolean;
	webURL?: string;
	roleOfExternalID?: Record<string, string>;
};
type PersonalReaction = { emoji: string; imageURL?: string; byExternalIDs: string[] };
type PersonalAttachment = {
	id: string;
	filename: string;
	contentType: string;
	sizeBytes: number;
	digest?: string;
	widthPixels?: number;
	heightPixels?: number;
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

export class MessengerRefusal extends Error {
	constructor(
		message: string,
		readonly reason: string | undefined,
		readonly remaining?: number
	) {
		super(message);
		this.name = 'MessengerRefusal';
	}
}

async function ask<Value>(capability: string, body?: Record<string, unknown>): Promise<Value> {
	const answer = await callCompanyApp({ capability, body });
	if (answer.status >= 400) {
		throw new MessengerRefusal(
			messageOf(answer.body, `the app answered ${answer.status}`),
			reasonOf(answer.body),
			remainingOf(answer.body)
		);
	}
	return answer.body as Value;
}

function reasonOf(body: unknown): string | undefined {
	if (typeof body !== 'object' || body === null) return undefined;
	const { reason } = body as { reason?: unknown };
	return typeof reason === 'string' ? reason : undefined;
}

function remainingOf(body: unknown): number | undefined {
	if (typeof body !== 'object' || body === null) return undefined;
	const { remaining } = body as { remaining?: unknown };
	return typeof remaining === 'number' ? remaining : undefined;
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
		isPrivate: conversation.isPrivate ?? false,
		position,
		isWithTheAgent: conversation.isWithTheAgent,
		participants: (conversation.participantExternalIDs ?? []).map((externalID) => ({ externalID })),
		webURL: conversation.webURL,
		description: conversation.description,
		roleOfExternalID: conversation.roleOfExternalID
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
			sizeBytes: attachment.sizeBytes,
			digest: attachment.digest ?? '',
			widthPixels: attachment.widthPixels,
			heightPixels: attachment.heightPixels
		}))
	};
}

export type MessengerChannels = { channels: MessengerChannel[]; agentExternalID?: string };

export async function fetchChannels(): Promise<MessengerChannels> {
	const answer = await ask<{ conversations: PersonalConversation[]; agentExternalID?: string }>(
		'person.conversations.list'
	);
	return { channels: answer.conversations.map(asChannel), agentExternalID: answer.agentExternalID };
}

export type MessengerDirectoryPerson = { externalID: string; name: string; avatarURL?: string };

export async function fetchPeople(): Promise<MessengerDirectoryPerson[]> {
	const answer = await ask<{ people: MessengerDirectoryPerson[] }>('person.people.list');
	return answer.people;
}

export async function openDirectChannel(externalIDs: string[]): Promise<MessengerChannel> {
	const conversation = await ask<PersonalConversation>('person.dm.ensure', {
		counterpartExternalIDs: externalIDs
	});
	return asChannel(conversation, 0);
}

export type NewChannel = {
	name: string;
	description?: string;
	visibility: 'open' | 'private';
	memberExternalIDs: string[];
};

export type CreatedChannel = { channel: MessengerChannel; uninvitedExternalIDs: string[] };

export async function createChannel(channel: NewChannel): Promise<CreatedChannel> {
	const created = await ask<PersonalConversation & { uninvitedExternalIDs?: string[] }>(
		'person.channel.create',
		channel
	);
	return { channel: asChannel(created, 0), uninvitedExternalIDs: created.uninvitedExternalIDs ?? [] };
}

export type OpenChannel = { id: string; name: string; description?: string };

export async function fetchOpenChannels(): Promise<OpenChannel[]> {
	const answer = await ask<{ channels: PersonalConversation[] }>('person.channels.open.list');
	return answer.channels.map(({ id, name, description }) => ({ id, name, description }));
}

export async function joinChannel(channelID: string): Promise<void> {
	await ask('person.channel.join', { conversationID: channelID });
}

export async function addChannelMembers(channelID: string, memberExternalIDs: string[]): Promise<string[]> {
	const answer = await ask<{ uninvitedExternalIDs?: string[] }>('person.channel.members.add', {
		conversationID: channelID,
		memberExternalIDs
	});
	return answer.uninvitedExternalIDs ?? [];
}

export async function leaveChannel(channelID: string): Promise<void> {
	await ask('person.channel.leave', { conversationID: channelID });
}

export async function handOverChannel(channelID: string, newOwnerExternalID: string): Promise<void> {
	await ask('person.channel.owner.set', { conversationID: channelID, externalID: newOwnerExternalID });
}

export async function addChannelOwner(channelID: string, externalID: string): Promise<void> {
	await ask('person.channel.owner.add', { conversationID: channelID, externalID });
}

export async function removeChannelMember(channelID: string, externalID: string): Promise<void> {
	await ask('person.channel.member.remove', { conversationID: channelID, externalID });
}

export async function deleteChannel(channelID: string): Promise<void> {
	await ask('person.channel.delete', { conversationID: channelID });
}

export async function fetchPosts(channelID: string, before?: string): Promise<MessengerPost[]> {
	const answer = await ask<{ messages: PersonalMessage[] }>('person.messages.list', {
		conversationID: channelID,
		before
	});
	return answer.messages.map(asPost);
}

// The messenger holds the file on the company's own machine, which a browser
// somewhere else cannot reach. The relay puts a copy in the company's bucket
// and names it; the reader signs for that with their own session.
export async function keepAttachmentForReading(
	attachment: MessengerAttachment
): Promise<KeptAttachment | null> {
	const answer = await ask<{ attachment: KeptAttachment | null }>('person.message.attachment', {
		messageID: attachment.url,
		filename: attachment.filename,
		contentType: attachment.contentType,
		digest: attachment.digest
	});
	return answer.attachment;
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

export async function deletePost(channelID: string, messageID: string): Promise<void> {
	await ask('person.message.delete', { conversationID: channelID, messageID });
}

export async function addReaction(channelID: string, messageID: string, emoji: string): Promise<void> {
	await ask('person.reaction.add', { conversationID: channelID, messageID, emoji });
}

export async function removeReaction(channelID: string, messageID: string, emoji: string): Promise<void> {
	await ask('person.reaction.remove', { conversationID: channelID, messageID, emoji });
}

export async function fetchCustomEmojiNames(): Promise<string[]> {
	const answer = await ask<{ emoji: { name: string }[] }>('person.emoji.list');
	return answer.emoji.map((emoji) => emoji.name);
}

export async function fetchCustomEmojiImage(name: string): Promise<{ dataURL: string } | null> {
	const answer = await ask<{ image: { dataURL: string } | null }>('person.emoji.image', { name });
	return answer.image;
}

export type KeptPicture = { address: string };

// A face is answered the way a file is: as an object in the company's bucket
// the reader signs for. The avatar URL names the bytes, so one the company has
// kept before is answered without the messenger being read at all.
export async function keepPersonPictureForReading(person: {
	externalID: string;
	avatarURL: string;
}): Promise<KeptPicture | null> {
	const answer = await ask<{ picture: KeptPicture | null }>('person.picture', person);
	return answer.picture;
}

export function fetchLinkPreview(url: string): Promise<LinkPreview | null> {
	return ask<LinkPreview | null>('asset.link', { url });
}
