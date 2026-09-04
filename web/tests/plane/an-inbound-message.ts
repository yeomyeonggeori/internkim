import type { ACompanyPlane } from './a-company-plane';

export type AnInboundMessage = {
	conversationID: string;
	messageID: string;
	sender: { email: string; name?: string; handle?: string };
	message: string;
	/** 'direct' is one person writing to the agent; 'channel' is a room. */
	conversationType?: 'direct' | 'channel';
	botMentioned?: boolean;
	channelName?: string;
};

/**
 * The body chatd's forwardNormalizedEvent builds, field for field. A scenario
 * that posts a shape of its own proves the relay takes that shape and nothing
 * about the messages a company actually sends.
 */
export function aChatdInboundEvent(
	plane: ACompanyPlane,
	message: AnInboundMessage
): Record<string, unknown> {
	const conversationType = message.conversationType ?? 'direct';
	const senderID = message.sender.handle ?? message.sender.email;
	return {
		platform: plane.messengerPlatform,
		conversationID: message.conversationID,
		messageID: message.messageID,
		senderID,
		replyTargetID: message.conversationID,
		prompt: message.message,
		context: {
			messages: [],
			messagesOpenOtherExchanges: false,
			hasMoreBefore: false,
			historyCursor: message.conversationID,
			sender: {
				platform: plane.messengerPlatform,
				senderID,
				handle: message.sender.handle,
				email: message.sender.email,
				name: message.sender.name
			},
			conversationType,
			channelID: conversationType === 'channel' ? message.conversationID : undefined,
			channelName: message.channelName,
			inputAttachments: [],
			addressing: {
				botMentioned: message.botMentioned ?? conversationType === 'direct',
				otherPersonMentioned: false
			}
		}
	};
}

/** The relay answers 202 once the event is on disk; delivery comes after. */
export async function handToTheRelay(
	plane: ACompanyPlane,
	message: AnInboundMessage
): Promise<Response> {
	return fetch(plane.relayInboundURL, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(aChatdInboundEvent(plane, message))
	});
}

export async function until(
	what: string,
	ready: () => boolean | Promise<boolean>,
	seconds = 60
): Promise<void> {
	for (let attempt = 0; attempt < seconds * 4; attempt += 1) {
		if (await ready()) return;
		await Bun.sleep(250);
	}
	throw new Error(what);
}
