import type { Addressing, PostFileToConversation, PostToConversation } from './acp-session';
import type { KeptAttachment, WorkspaceFile } from './file-transfer';

export type ChatdAnswer = { status: number; body: unknown };

export type ConversationPostSettings = {
	askChatd: (capability: string, body: Record<string, unknown>) => Promise<ChatdAnswer>;
	tellBrowsers: (conversationID: string, messageID: string) => void;
};

export type AgentFilePostSettings = {
	keepForTheMessenger: (requesterEmail: string, file: WorkspaceFile) => Promise<KeptAttachment>;
	postToConversation: PostToConversation;
};

export function conversationPoster(settings: ConversationPostSettings): PostToConversation {
	return async (addressing, message, attachments = []) => {
		const threadID = replyThreadOf(addressing);
		const posted = await settings.askChatd('message.post', {
			threadID,
			message,
			...(attachments.length > 0 ? { attachments } : {})
		});
		if (posted.status >= 300) {
			throw new Error(
				`chatd refused the post to ${threadID} in ${addressing.conversationID} with ${posted.status}: ${JSON.stringify(posted.body)}`
			);
		}
		const messageID = postedMessageIDOf(posted.body);
		settings.tellBrowsers(addressing.conversationID, messageID);
		return messageID;
	};
}

export function agentFilePoster(settings: AgentFilePostSettings): PostFileToConversation {
	return async (addressing, requesterEmail, file) => {
		const kept = await settings.keepForTheMessenger(requesterEmail, file);
		return settings.postToConversation(addressing, '', [kept]);
	};
}

export function isAReplyInTheThreadOf(message: Addressing, thread: Addressing): boolean {
	if (!message.isThread) return false;
	if (message.conversationID !== thread.conversationID) return false;
	return replyThreadOf(message) === replyThreadOf(thread);
}

function replyThreadOf(addressing: Addressing): string {
	return addressing.replyTargetID ?? addressing.conversationID;
}

function postedMessageIDOf(body: unknown): string {
	if (typeof body !== 'object' || body === null || !('messageID' in body)) return '';
	return typeof body.messageID === 'string' ? body.messageID : '';
}
