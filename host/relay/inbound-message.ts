import type { Addressing, Requester } from './acp-session';

export type InboundMessage = {
	requester: Requester;
	addressing: Addressing;
	messageID: string;
	message: string;
};

export function readInboundMessage(offered: unknown): InboundMessage | null {
	if (typeof offered !== 'object' || offered === null) return null;
	const held = offered as Record<string, unknown>;

	const requesterEmail = text(held.requesterEmail).toLowerCase();
	const conversationID = text(held.conversationID);
	const message = text(held.message);
	if (!requesterEmail || !conversationID || !message) return null;

	return {
		requester: {
			email: requesterEmail,
			name: text(held.requesterName) || undefined,
			callingName: text(held.requesterCallingName) || undefined,
			handle: text(held.requesterHandle) || undefined
		},
		addressing: {
			platform: text(held.platform),
			conversationID,
			conversationType: text(held.conversationType) || undefined,
			replyTargetID: text(held.replyTargetID) || undefined,
			isThread: held.isThread === true,
			responseLanguage: text(held.responseLanguage) || undefined
		},
		messageID: text(held.messageID),
		message
	};
}

function text(offered: unknown): string {
	return typeof offered === 'string' ? offered.trim() : '';
}
