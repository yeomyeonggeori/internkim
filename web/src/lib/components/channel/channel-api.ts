import { isSupabaseConfigured } from '$lib/supabase';
import {
	bridgeAddReaction,
	bridgeAgentConversation,
	bridgeConversation,
	bridgeConversations,
	bridgeDeleteMessage,
	bridgeEditMessage,
	bridgeDirectMessage,
	bridgePeople,
	bridgeRemoveReaction,
	bridgeSendMessage
} from '$lib/messenger/channel-over-bridge';

import { reactionsWithValues } from './channel-reactions';
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
	value: string;
	count: number;
	imageURL?: string;
	reactedByMe?: boolean;
	people?: ChannelParticipant[];
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

export type ChannelMentions = {
	externalIDs: string[];
	isEveryone: boolean;
};

export type ChannelMessage = {
	id: string;
	threadRootId?: string;
	sender: ChannelParticipant;
	text: string;
	sentAt: string;
	editedAt?: string;
	mentions?: ChannelMentions;
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

export type ChannelRole = 'owner' | 'admin' | 'member';

export type ChannelMember = { memberID?: string; externalID?: string; name: string; role: ChannelRole };

export type ChannelSummary = {
	id: string;
	name: string;
	kind: 'dm' | 'group';
	isPrivate?: boolean;
	avatarURL?: string;
	counterpart?: { memberID?: string; externalID?: string };
	members?: ChannelMember[];
	myRole?: ChannelRole;
	isWithTheAgent?: boolean;
	description?: string;
	platform?: string;
	webURL?: string;
	unreadCount?: number;
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

export async function ensureAgentConversation(): Promise<string> {
	if (isSupabaseConfigured()) return bridgeAgentConversation();
	return (await fetchChannelConversation()).conversationID;
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
		messages: (document.messages ?? []).map((message) => ({
			...message,
			reactions: reactionsWithValues(message.reactions)
		})),
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
	const carried = await Promise.all(attachments.map(carriedInTheRequest));
	const response = await fetch(conversationURL(channelID), {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ message, attachments: carried, replyToRootId: replyToRootID })
	});
	if (!response.ok) throw new Error(await response.text());
}

function isImageType(contentType: string): boolean {
	return contentType.startsWith('image/');
}

async function carriedInTheRequest(
	attachment: ChannelOutgoingAttachment
): Promise<{ filename: string; contentType: string; contentBase64: string }> {
	const bytes = new Uint8Array(await attachment.content.arrayBuffer());
	let binary = '';
	for (let start = 0; start < bytes.length; start += 0x8000) {
		binary += String.fromCharCode(...bytes.subarray(start, start + 0x8000));
	}
	return { filename: attachment.filename, contentType: attachment.contentType, contentBase64: btoa(binary) };
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
		const blob = await uploadBlob(relayURL, secretHex, new Uint8Array(await attachment.content.arrayBuffer()), attachment.contentType);
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
	replyToRootID?: string,
	mentions?: ChannelMentions
): Promise<void> {
	if (isSupabaseConfigured()) {
		await bridgeSendMessage(message, channelID, replyToRootID, attachments, mentions);
		return;
	}
	await sendServerSignedMessage(message, attachments, channelID, replyToRootID);
}

export function canChangeMessages(): boolean {
	return isSupabaseConfigured();
}

export async function editChannelMessage(messageID: string, text: string, channelID?: string): Promise<void> {
	if (!isSupabaseConfigured()) throw new Error('this messenger cannot change a message here');
	await bridgeEditMessage(channelID, messageID, text);
}

export async function deleteChannelMessage(messageID: string, channelID?: string): Promise<void> {
	if (!isSupabaseConfigured()) throw new Error('this messenger cannot change a message here');
	await bridgeDeleteMessage(channelID, messageID);
}

export async function addChannelReaction(messageID: string, value: string, channelID?: string): Promise<void> {
	if (!isSupabaseConfigured()) throw new Error('this messenger cannot change a message here');
	await bridgeAddReaction(channelID, messageID, value);
}

export async function removeChannelReaction(messageID: string, value: string, channelID?: string): Promise<void> {
	if (!isSupabaseConfigured()) throw new Error('this messenger cannot change a message here');
	await bridgeRemoveReaction(channelID, messageID, value);
}
