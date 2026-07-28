import { fetchMailMessage } from './mail-api';
import { mailPageMessageKey, mergeMessageDetailEnvelope } from './mail-message-detail-cache';
import { normalizeMailMessagePageCacheActorEmail } from './mail-message-page-cache';
import type { MailPageControllerState, MailPageText } from './mail-page-controller-types';
import type { MailMessage } from './mail-types';
import { markMailMessageRead } from './mail-page-message-mutation-actions';

export function selectMailPageMessage(controller: MailPageControllerState, text: MailPageText, message: MailMessage) {
	controller.selectedMessage = message;
	loadMessageDetail(controller, text, message);
	markMailMessageRead(controller, text, message);
}

export async function loadMessageDetail(controller: MailPageControllerState, text: MailPageText, message: MailMessage) {
	const requestID = nextMessageDetailRequestID(controller);
	const actorEmail = controller.mailActorEmail();
	const messageKey = mailPageMessageKey(message);
	const cachedMessage = controller.messageDetailCache.get(messageKey);
	if (cachedMessage) {
		const currentMessage = controller.messages.find((candidateMessage) => mailPageMessageKey(candidateMessage) === messageKey) ?? message;
		const mergedMessage = mergeMessageDetailEnvelope(cachedMessage, currentMessage);
		controller.messageDetailCache.set(messageKey, mergedMessage);
		controller.selectedMessage = mergedMessage;
		controller.isLoadingMessage = false;
		controller.errorMessage = '';
		return;
	}
	controller.isLoadingMessage = true;
	controller.errorMessage = '';
	try {
		const detail = await fetchMailMessage(actorEmail, message, controller.mailErrors(text.errors.loadMessage));
		if (!isCurrentMessageDetailRequest(controller, requestID, actorEmail, messageKey)) return;
		controller.selectedMessage = { ...message, ...detail };
		controller.messageDetailCache.set(messageKey, controller.selectedMessage);
	} catch (error) {
		if (!isCurrentMessageDetailRequest(controller, requestID, actorEmail, messageKey)) return;
		controller.errorMessage = error instanceof Error ? error.message : text.errors.loadMessage;
	} finally {
		if (isCurrentMessageDetailRequest(controller, requestID, actorEmail, messageKey)) controller.isLoadingMessage = false;
	}
}

function nextMessageDetailRequestID(controller: MailPageControllerState) {
	controller.messageDetailRequestID += 1;
	return controller.messageDetailRequestID;
}

function isCurrentMessageDetailRequest(controller: MailPageControllerState, requestID: number, actorEmail: string, messageKey: string) {
	return (
		controller.messageDetailRequestID === requestID &&
		normalizeMailMessagePageCacheActorEmail(controller.mailActorEmail()) === normalizeMailMessagePageCacheActorEmail(actorEmail) &&
		mailPageMessageKey(controller.selectedMessage) === messageKey
	);
}
