import { plainPreviewOf } from './plain-preview';

export type ArrivedMessage = {
	conversationID: string;
	messageID: string;
	authorExternalID: string;
	authorName: string;
	recipientExternalIDs: string[];
	preview: string;
	attachmentKinds: string[];
	pictureDigest: string;
	pictureContentType: string;
};

export type Told = {
	title: string;
	body: string;
	openPath: string;
	tag: string;
};

const previewLimit = 140;
const cutMark = '…';

export function readArrivedMessage(offered: unknown): ArrivedMessage | null {
	if (typeof offered !== 'object' || offered === null) return null;
	const held = offered as Record<string, unknown>;

	const messageID = text(held.messageID);
	const authorExternalID = text(held.authorExternalID);
	if (!messageID || !authorExternalID) return null;

	return {
		conversationID: text(held.conversationID),
		messageID,
		authorExternalID,
		authorName: text(held.authorName),
		recipientExternalIDs: externalIDs(held.recipientExternalIDs, authorExternalID),
		preview: limitedPreviewOf(text(held.preview)),
		attachmentKinds: words(held.attachmentKinds),
		pictureDigest: text(held.pictureDigest),
		pictureContentType: text(held.pictureContentType)
	};
}

export type NotifyRequest = Told & {
	platform: string;
	externalIDs: string[];
	senderExternalID: string;
	category: 'message';
	conversationID: string;
	messageID: string;
	withdrawnMessageIDs: string[];
	senderPicturePath: string;
	attachmentKinds: string[];
	messagePicturePath: string;
};

export function tellingOf(arrived: ArrivedMessage, authorName: string): Told {
	return {
		title: authorName || 'internkim',
		body: arrived.preview,
		openPath: messagePathOf(arrived),
		tag: `message:${arrived.conversationID}`
	};
}

function messagePathOf(arrived: ArrivedMessage): string {
	if (!arrived.conversationID) return '/messenger/';
	return `/messenger/?channel=${encodeURIComponent(arrived.conversationID)}&message=${encodeURIComponent(arrived.messageID)}`;
}

export function notifyRequestOf(
	arrived: ArrivedMessage,
	authorName: string,
	platform: string,
	senderPicturePath: string,
	withdrawnMessageIDs: string[],
	messagePicturePath: string
): NotifyRequest {
	return {
		platform,
		externalIDs: arrived.recipientExternalIDs,
		senderExternalID: arrived.authorExternalID,
		category: 'message',
		conversationID: arrived.conversationID,
		messageID: arrived.messageID,
		withdrawnMessageIDs,
		senderPicturePath,
		attachmentKinds: arrived.attachmentKinds,
		messagePicturePath,
		...tellingOf(arrived, authorName)
	};
}

function limitedPreviewOf(message: string): string {
	const characters = Array.from(plainPreviewOf(message));
	if (characters.length <= previewLimit) return characters.join('');
	return characters.slice(0, previewLimit - 1).join('').trimEnd() + cutMark;
}

function text(offered: unknown): string {
	return typeof offered === 'string' ? offered.trim() : '';
}

function words(offered: unknown): string[] {
	if (!Array.isArray(offered)) return [];
	return offered.filter((entry): entry is string => typeof entry === 'string' && entry.trim() !== '');
}

export function externalIDs(offered: unknown, author: string): string[] {
	if (!Array.isArray(offered)) return [];
	const named = offered.filter((entry): entry is string => typeof entry === 'string' && entry.trim() !== '');
	return [...new Set(named.map((entry) => entry.trim()))].filter((entry) => entry !== author);
}
