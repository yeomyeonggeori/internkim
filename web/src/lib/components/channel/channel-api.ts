import { publishBuzzMessage } from '$lib/buzz-relay-client';
import { imetaTag, uploadBlob } from '$lib/buzz-blossom';
import { buzzIdentity } from '$lib/stores/buzz-identity.svelte';

export type ChannelParticipant = {
	id: string;
	name: string;
	email?: string;
	avatarURL?: string;
};

export type ChannelChoiceOption = {
	key: string;
	label: string;
	value?: string;
};

export type ChannelInteraction = {
	kind: string;
	question: string;
	options: ChannelChoiceOption[];
	recommendedOptionKey?: string;
};

export type ChannelMessageReaction = {
	emoji: string;
	count: number;
	imageURL?: string;
};

export type ChannelMessageAttachment = {
	kind: 'image' | 'file';
	url: string;
	filename?: string;
	mimeType?: string;
	sizeBytes?: number;
};

export type ChannelCustomEmoji = {
	name: string;
	url: string;
};

export type ThreadSummary = {
	replyCount: number;
	lastReplyAt: string;
	participants: ChannelParticipant[];
};

export type ChannelMessage = {
	id: string;
	threadRootId?: string;
	sender: ChannelParticipant;
	text: string;
	sentAt: string;
	isError?: boolean;
	interaction?: ChannelInteraction;
	reactions?: ChannelMessageReaction[];
	attachments?: ChannelMessageAttachment[];
	customEmoji?: ChannelCustomEmoji[];
	thread?: ThreadSummary;
};

export type ChannelConversation = {
	conversationID: string;
	currentUserID: string;
	messages: ChannelMessage[];
	hasMoreBefore: boolean;
	historyCursor: string;
};

export type MessageContent = {
	text: string;
	imageURLs: string[];
};

const imageMarkdownPattern = /!\[[^\]]*\]\((\S+?)\)/g;

export function parseMessageContent(text: string): MessageContent {
	const imageURLs: string[] = [];
	const stripped = text.replace(imageMarkdownPattern, (_match, url: string) => {
		imageURLs.push(url);
		return '';
	});
	return { text: stripped.trim(), imageURLs };
}

const emojiShortcodePattern = /:([^:\s]+):/g;

export function applyCustomEmoji(text: string, customEmoji?: ChannelCustomEmoji[]): string {
	if (!customEmoji || customEmoji.length === 0) return text;
	const urlByName = new Map(customEmoji.map((emoji) => [emoji.name, emoji.url]));
	return text.replace(emojiShortcodePattern, (whole, name: string) => {
		const url = urlByName.get(name);
		return url ? `![${name}](${url})` : whole;
	});
}

export type ChannelSummary = {
	id: string;
	name: string;
	kind: 'dm' | 'group';
	avatarURL?: string;
};

export async function fetchConversations(): Promise<ChannelSummary[]> {
	const response = await fetch('/agent/api/channels', { credentials: 'include', cache: 'no-store' });
	if (!response.ok) throw new Error(await response.text());
	const document: { conversations?: ChannelSummary[] } = await response.json();
	return document.conversations ?? [];
}

export type Person = {
	id: string;
	name: string;
	avatarURL?: string;
};

export async function fetchPeople(): Promise<Person[]> {
	const response = await fetch('/agent/api/people', { credentials: 'include', cache: 'no-store' });
	if (!response.ok) throw new Error(await response.text());
	const document: { people?: Person[] } = await response.json();
	return document.people ?? [];
}

export async function ensureDirectMessage(personID: string): Promise<string> {
	const response = await fetch('/agent/api/dm/ensure', {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ personId: personID })
	});
	if (!response.ok) throw new Error(await response.text());
	const document: { channelId?: string } = await response.json();
	return document.channelId ?? '';
}

function conversationURL(channelID?: string, before?: string): string {
	const params = new URLSearchParams();
	if (channelID) params.set('channelId', channelID);
	if (before) params.set('before', before);
	const query = params.toString();
	return query ? `/agent/api/dm?${query}` : '/agent/api/dm';
}

export async function fetchChannelConversation(
	channelID?: string,
	before?: string
): Promise<ChannelConversation> {
	const response = await fetch(conversationURL(channelID, before), {
		credentials: 'include',
		cache: 'no-store'
	});
	if (!response.ok) throw new Error(await response.text());
	const document: {
		conversationID?: string;
		currentUserId?: string;
		messages?: ChannelMessage[];
		hasMoreBefore?: boolean;
		historyCursor?: string;
	} = await response.json();
	return {
		conversationID: document.conversationID ?? '',
		currentUserID: document.currentUserId ?? '',
		messages: document.messages ?? [],
		hasMoreBefore: document.hasMoreBefore ?? false,
		historyCursor: document.historyCursor ?? ''
	};
}

export type ChannelOutgoingAttachment = {
	filename: string;
	contentType: string;
	contentBase64: string;
};

let cachedRelayURL: string | null | undefined;

async function buzzRelayURL(): Promise<string | null> {
	if (cachedRelayURL !== undefined) return cachedRelayURL;
	try {
		const response = await fetch('/agent/api/buzz-relay-config', { credentials: 'include' });
		const document: { relayURL?: string } = await response.json();
		cachedRelayURL = document.relayURL?.trim() || null;
	} catch {
		cachedRelayURL = null;
	}
	return cachedRelayURL;
}

async function sendServerSignedMessage(
	message: string,
	attachments: ChannelOutgoingAttachment[],
	channelID?: string,
	replyToRootID?: string
): Promise<void> {
	const response = await fetch(conversationURL(channelID), {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ message, attachments, replyToRootId: replyToRootID })
	});
	if (!response.ok) throw new Error(await response.text());
}

function isImageType(contentType: string): boolean {
	return contentType.startsWith('image/');
}

function base64ToBytes(value: string): Uint8Array {
	const binary = atob(value);
	const bytes = new Uint8Array(binary.length);
	for (let index = 0; index < binary.length; index++) bytes[index] = binary.charCodeAt(index);
	return bytes;
}

async function clientSignChannelMessage(
	relayURL: string,
	secretHex: string,
	channelID: string,
	message: string,
	attachments: ChannelOutgoingAttachment[],
	replyToRootID?: string
): Promise<void> {
	const bodyParts = message.trim() === '' ? [] : [message];
	const imetaTags: string[][] = [];
	for (const attachment of attachments) {
		const blob = await uploadBlob(relayURL, secretHex, base64ToBytes(attachment.contentBase64), attachment.contentType);
		const label = attachment.filename.trim() || (isImageType(attachment.contentType) ? 'image' : 'file');
		bodyParts.push(isImageType(attachment.contentType) ? `![${label}](${blob.url})` : `[${label}](${blob.url})`);
		imetaTags.push(imetaTag(blob));
	}
	await publishBuzzMessage(relayURL, secretHex, {
		channelId: channelID,
		content: bodyParts.join('\n'),
		replyToRootId: replyToRootID,
		extraTags: imetaTags
	});
}

// Text and attachments are both authored in the browser with the person's own
// key: attachments upload to the relay's Blossom store (client-signed) and the
// message publishes straight to Buzz with their imeta tags, so nothing is signed
// server-side on their behalf. The server path is only a fallback before the
// identity is unlocked or when the relay is unknown.
export async function sendChannelMessage(
	message: string,
	attachments: ChannelOutgoingAttachment[] = [],
	channelID?: string,
	replyToRootID?: string
): Promise<void> {
	// The relay lives on the device's loopback, unreachable from the browser, so
	// the message and its attachments are published through the on-device bridge
	// (chatd), which signs with the person's own derived key.
	await sendServerSignedMessage(message, attachments, channelID, replyToRootID);
}
