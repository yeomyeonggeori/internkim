import { mailMessageKey } from './mail-message-utils';
import type { MailMessage } from './mail-types';

export function mailPageMessageKey(message: MailMessage | null) {
	if (!message) return '';
	return mailMessageKey(message);
}

export function selectedVisibleMessage(messages: MailMessage[], selectedMessage: MailMessage | null, canSelectFirstMessage = true) {
	const selectedMessageKey = mailPageMessageKey(selectedMessage);
	const visibleMessage = messages.find((message) => mailPageMessageKey(message) === selectedMessageKey);
	if (visibleMessage) return visibleMessage;
	return canSelectFirstMessage ? messages[0] ?? null : null;
}

export function mergeMessageDetailCache(messageDetailCache: Map<string, MailMessage>, messages: MailMessage[]) {
	for (const message of messages) {
		const messageKey = mailPageMessageKey(message);
		const cachedMessage = messageDetailCache.get(messageKey);
		if (!cachedMessage) continue;
		messageDetailCache.set(messageKey, mergeMessageDetailEnvelope(cachedMessage, message));
	}
}

export function mergeMessageDetailEnvelope(cachedMessage: MailMessage, message: MailMessage) {
	const mergedMessage = { ...cachedMessage, ...message };
	if (cachedMessage.to !== undefined) mergedMessage.to = cachedMessage.to;
	if (cachedMessage.cc !== undefined) mergedMessage.cc = cachedMessage.cc;
	if (cachedMessage.body !== undefined) mergedMessage.body = cachedMessage.body;
	if (cachedMessage.bodyHTML !== undefined) mergedMessage.bodyHTML = cachedMessage.bodyHTML;
	return mergedMessage;
}
