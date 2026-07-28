import { moveMailMessage, updateMailMessageFlags } from './mail-api';
import { mailPageMessageKey, selectedVisibleMessage } from './mail-message-detail-cache';
import { removeMessageFromPageCache, updateMessageInPageCache } from './mail-message-page-cache';
import { mailboxNameByHint } from './mail-page-utils';
import type { MailPageControllerState, MailPageText } from './mail-page-controller-types';
import type { MailMessage } from './mail-types';
import { loadMessageDetail } from './mail-page-message-detail-actions';
import { loadPageMailboxes } from './mail-page-message-list-actions';

export async function moveSelectedMailMessage(controller: MailPageControllerState, text: MailPageText, targetHint: string) {
	if (!controller.selectedMessage) return;
	const movedMessage = controller.selectedMessage;
	const movedMessageKey = mailPageMessageKey(movedMessage);
	const targetMailbox = mailboxNameByHint(controller.pageMailboxes(), targetHint);
	if (!targetMailbox) {
		controller.errorMessage = `${targetHint} ${text.errors.mailboxNotFound}`;
		return;
	}
	try {
		await moveMailMessage(controller.mailActorEmail(), movedMessage, targetMailbox, controller.mailErrors(text.errors.moveMessage));
	} catch (error) {
		controller.errorMessage = error instanceof Error ? error.message : text.errors.moveMessage;
		return;
	}
	controller.errorMessage = '';
	controller.messages = controller.messages.filter((message) => mailPageMessageKey(message) !== movedMessageKey);
	removeMessageFromPageCache(controller.messageListCache, movedMessageKey);
	controller.messageDetailCache.delete(movedMessageKey);
	if (mailPageMessageKey(controller.selectedMessage) === movedMessageKey) {
		controller.selectedMessage = controller.visibleMessages()[0] ?? null;
		if (controller.selectedMessage) await loadMessageDetail(controller, text, controller.selectedMessage);
	}
	try {
		await loadPageMailboxes(controller, text);
	} catch (error) {
		controller.errorMessage = error instanceof Error ? error.message : text.errors.loadMailboxes;
	}
}

export async function markMailMessageRead(controller: MailPageControllerState, text: MailPageText, message: MailMessage) {
	if (message.isRead) return;
	const messageKey = mailPageMessageKey(message);
	try {
		await updateMailMessageFlags(controller.mailActorEmail(), message, true, controller.mailErrors(text.errors.updateMessage));
	} catch (error) {
		controller.errorMessage = error instanceof Error ? error.message : text.errors.updateMessage;
		return;
	}
	const cachedMessage = controller.messageDetailCache.get(messageKey);
	if (cachedMessage) controller.messageDetailCache.set(messageKey, { ...cachedMessage, isRead: true });
	if (mailPageMessageKey(controller.selectedMessage) === messageKey && controller.selectedMessage) {
		controller.selectedMessage = { ...controller.selectedMessage, isRead: true };
	}
	controller.messages = controller.messages.map((candidateMessage) => (mailPageMessageKey(candidateMessage) === messageKey ? { ...candidateMessage, isRead: true } : candidateMessage));
	updateMessageInPageCache(controller.messageListCache, messageKey, (candidateMessage) => ({ ...candidateMessage, isRead: true }));
	try {
		await loadPageMailboxes(controller, text);
	} catch (error) {
		controller.errorMessage = error instanceof Error ? error.message : text.errors.loadMailboxes;
	}
}

export function setMailPageUnreadOnly(controller: MailPageControllerState, text: MailPageText, isUnreadOnly: boolean) {
	if (controller.isUnreadOnly === isUnreadOnly) return Promise.resolve();
	controller.isUnreadOnly = isUnreadOnly;
	controller.selectedMessage = selectedVisibleMessage(controller.visibleMessages(), controller.selectedMessage, controller.canSelectFirstMessage);
	if (!controller.selectedMessage) return Promise.resolve();
	return loadMessageDetail(controller, text, controller.selectedMessage);
}
