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
	thread?: ThreadSummary;
};

export type ChannelConversation = {
	conversationID: string;
	currentUserID: string;
	messages: ChannelMessage[];
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

function conversationURL(channelID?: string): string {
	return channelID ? `/agent/api/dm?channelId=${encodeURIComponent(channelID)}` : '/agent/api/dm';
}

export async function fetchChannelConversation(channelID?: string): Promise<ChannelConversation> {
	const response = await fetch(conversationURL(channelID), { credentials: 'include', cache: 'no-store' });
	if (!response.ok) throw new Error(await response.text());
	const document: {
		conversationID?: string;
		currentUserId?: string;
		messages?: ChannelMessage[];
	} = await response.json();
	return {
		conversationID: document.conversationID ?? '',
		currentUserID: document.currentUserId ?? '',
		messages: document.messages ?? []
	};
}

export type ChannelOutgoingAttachment = {
	filename: string;
	contentType: string;
	contentBase64: string;
};

export async function sendChannelMessage(
	message: string,
	attachments: ChannelOutgoingAttachment[] = [],
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
