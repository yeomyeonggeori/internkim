export type ArrivedMessage = {
	conversationID: string;
	messageID: string;
	authorExternalID: string;
	authorName: string;
	recipientExternalIDs: string[];
	preview: string;
};

export type Told = {
	title: string;
	body: string;
	openPath: string;
	tag: string;
};

const previewLimit = 140;

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
		preview: text(held.preview).slice(0, previewLimit)
	};
}

export type NotifyRequest = Told & {
	platform: string;
	externalIDs: string[];
	senderExternalID: string;
	category: 'message';
	conversationID: string;
	senderPicturePath: string;
};

export function tellingOf(arrived: ArrivedMessage, authorName: string): Told {
	return {
		title: authorName || 'internkim',
		body: arrived.preview,
		openPath: conversationPathOf(arrived.conversationID),
		tag: `message:${arrived.conversationID}`
	};
}

function conversationPathOf(conversationID: string): string {
	if (!conversationID) return '/messenger/';
	return `/messenger/?channel=${encodeURIComponent(conversationID)}`;
}

export function notifyRequestOf(
	arrived: ArrivedMessage,
	authorName: string,
	platform: string,
	senderPicturePath: string
): NotifyRequest {
	return {
		platform,
		externalIDs: arrived.recipientExternalIDs,
		senderExternalID: arrived.authorExternalID,
		category: 'message',
		conversationID: arrived.conversationID,
		senderPicturePath,
		...tellingOf(arrived, authorName)
	};
}

function text(offered: unknown): string {
	return typeof offered === 'string' ? offered.trim() : '';
}

function externalIDs(offered: unknown, author: string): string[] {
	if (!Array.isArray(offered)) return [];
	const named = offered.filter((entry): entry is string => typeof entry === 'string' && entry.trim() !== '');
	return [...new Set(named.map((entry) => entry.trim()))].filter((entry) => entry !== author);
}
