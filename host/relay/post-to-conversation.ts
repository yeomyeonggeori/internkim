import type { Addressing } from './acp-session';

export type AskChatd = (
	capability: string,
	body: Record<string, unknown>
) => Promise<{ status: number; body: unknown }>;

export function replySendBody(addressing: Addressing, message: string): Record<string, unknown> {
	return {
		replyTargetID: addressing.replyTargetID ?? addressing.conversationID,
		...(addressing.answeringMessageID ? { answeringMessageID: addressing.answeringMessageID } : {}),
		message
	};
}

export async function postToConversation(
	askChatd: AskChatd,
	addressing: Addressing,
	message: string
): Promise<void> {
	const response = await askChatd('reply.send', replySendBody(addressing, message));
	if (response.status >= 200 && response.status < 300) return;
	throw new Error(`chatd reply.send returned HTTP ${response.status}: ${bodyText(response.body)}`);
}

function bodyText(body: unknown): string {
	if (typeof body === 'string') return body;
	if (body === undefined) return '';
	return JSON.stringify(body) ?? '';
}
