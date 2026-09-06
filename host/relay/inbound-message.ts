import type { Addressing, Requester } from './acp-session';
import { personName, personNameLocale } from '../../web/src/lib/person-name';
import type { Locale } from '../../web/src/lib/i18n/locale';

export type InboundMessage = {
	key: string;
	requester: Requester;
	addressing: Addressing;
	messageID: string;
	message: string;
	context: Record<string, unknown>;
};

/**
 * The body is the one chatd already builds for an inbound event, unchanged, so
 * the two paths carry the same facts and neither has a shape of its own.
 */
export function readInboundMessage(offered: unknown): InboundMessage | null {
	if (typeof offered !== 'object' || offered === null) return null;
	const held = offered as Record<string, unknown>;

	const context = record(held.context);
	const sender = record(context.sender);
	const requesterEmail = text(sender.email).toLowerCase();
	const platform = text(held.platform) || text(sender.platform);
	const conversationID = text(held.conversationID);
	const messageID = text(held.messageID);
	const message = text(held.prompt);
	if (!requesterEmail || !conversationID || !message) return null;

	return {
		key: inboundMessageKey(platform, conversationID, messageID),
		requester: {
			email: requesterEmail,
			name: text(sender.name) || undefined,
			handle: text(sender.handle) || undefined
		},
		addressing: {
			platform,
			conversationID,
			conversationType: text(context.conversationType) || undefined,
			replyTargetID: text(held.replyTargetID) || undefined,
			isThread: held.isThread === true,
			responseLanguage: text(context.responseLanguage) || undefined
		},
		messageID,
		message,
		context
	};
}

export function inboundMessageKey(
	platform: string,
	conversationID: string,
	messageID: string
): string {
	return `${platform}:${conversationID}:${messageID}`;
}

export function displayNameForRequester(
	recordedName: string,
	companyLocale: string,
	responseLanguage: string
): string {
	const primaryLanguage = responseLanguage.trim().toLowerCase().split('-')[0];
	const requestedLocale: Locale = primaryLanguage === 'ko' ? 'ko' : 'en';
	return personName(recordedName, personNameLocale(requestedLocale, companyLocale));
}

function record(offered: unknown): Record<string, unknown> {
	if (typeof offered !== 'object' || offered === null) return {};
	return offered as Record<string, unknown>;
}

function text(offered: unknown): string {
	return typeof offered === 'string' ? offered.trim() : '';
}
