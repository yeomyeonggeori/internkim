import { isSupabaseConfigured } from '$lib/supabase';
import {
	bridgeConversation,
	bridgeConversations,
	bridgeDirectMessage,
	bridgePeople,
	bridgeSendMessage
} from '$lib/messenger/channel-over-bridge';

import { publishBuzzMessage } from '$lib/buzz-relay-client';
import { imetaTag, uploadBlob } from '$lib/buzz-blossom';
import { shortcodePattern } from '$lib/messenger/custom-emoji-names';
import type { OutgoingAttachment } from '$lib/messenger/messenger-api';
import { buzzIdentity } from '$lib/stores/buzz-identity.svelte';

export type ChannelParticipant = {
	id: string;
	name: string;
	email?: string;
	avatarURL?: string;
	memberID?: string;
	externalID?: string;
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

// url is where the message says the file is; source is where this browser can
// open it, which is somewhere else whenever the message points at the company's
// own machine. Empty until that copy exists.
export type ChannelMessageAttachment = {
	kind: 'image' | 'file';
	url: string;
	source?: string;
	filename?: string;
	mimeType?: string;
	sizeBytes?: number;
	widthPixels?: number;
	heightPixels?: number;
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

const imageMarkdownPattern = /!\[[^\]]*\]\((\S+?)\)/g;
const linkMarkdownPattern = /\[[^\]]*\]\((\S+?)\)/g;

// An imported message names each file it carries in its body, so a client that
// reads nothing but text still has them. This one draws them itself, and a
// reference to something already on screen is not text.
export function messageTextBeside(text: string, attachmentURLs: string[]): string {
	const withoutImages = text.replace(imageMarkdownPattern, '');
	const withoutFiles = withoutImages.replace(linkMarkdownPattern, (link, url: string) =>
		attachmentURLs.includes(url) ? '' : link
	);
	return withoutFiles.trim();
}

export function applyCustomEmoji(
	text: string,
	perMessageEmoji?: ChannelCustomEmoji[],
	globalEmoji?: Map<string, string>
): string {
	const urlByName = new Map(globalEmoji ?? []);
	for (const emoji of perMessageEmoji ?? []) urlByName.set(emoji.name, emoji.url);
	if (urlByName.size === 0) return text;
	return text.replace(shortcodePattern(), (whole, name: string) => {
		const url = urlByName.get(name);
		return url ? `![${name}](${url})` : whole;
	});
}

export type ChannelSummary = {
	id: string;
	name: string;
	kind: 'dm' | 'group';
	isPrivate?: boolean;
	avatarURL?: string;
	counterpart?: { memberID?: string; externalID?: string };
	platform?: string;
	webURL?: string;
};


export async function fetchConversations(): Promise<ChannelSummary[]> {
	if (isSupabaseConfigured()) return bridgeConversations();
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
	if (isSupabaseConfigured()) return bridgePeople();
	const response = await fetch('/agent/api/people', { credentials: 'include', cache: 'no-store' });
	if (!response.ok) throw new Error(await response.text());
	const document: { people?: Person[] } = await response.json();
	return document.people ?? [];
}

export async function ensureDirectMessage(personID: string): Promise<string> {
	if (isSupabaseConfigured()) return bridgeDirectMessage(personID);
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
	if (isSupabaseConfigured()) return bridgeConversation(channelID, before);
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

export type ChannelOutgoingAttachment = OutgoingAttachment;

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
	if (isSupabaseConfigured()) {
		await bridgeSendMessage(message, channelID, replyToRootID, attachments);
		return;
	}
	await sendServerSignedMessage(message, attachments, channelID, replyToRootID);
}
