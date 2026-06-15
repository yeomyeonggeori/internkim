import { fetchMailboxes, fetchMailMessage, fetchMailMessages, moveMailMessage, updateMailMessageFlags } from './mail-api';
import { mergeMailMessages } from './mail-message-utils';
import { mailboxNameByHint } from './mail-page-utils';
import type { MailMessage } from './mail-types';
import type { MailPageControllerState, MailPageText } from './mail-page-controller-types';

export async function loadPageMailboxes(controller: MailPageControllerState, text: MailPageText) {
	controller.isLoadingMailboxes = true;
	try {
		controller.mailboxes = await fetchMailboxes(controller.mailActorEmail(), controller.mailErrors(text.errors.loadMailboxes));
	} finally {
		controller.isLoadingMailboxes = false;
	}
}

export async function loadMessagesPage(controller: MailPageControllerState, text: MailPageText, isAppending: boolean) {
	if (!controller.account.isConfigured) return;
	if (isAppending && (!controller.hasMoreMessages || !controller.nextCursor || controller.isLoadingMore)) return;
	const requestID = nextMessageListRequestID(controller);
	const mailbox = controller.selectedMailbox;
	if (!isAppending) {
		controller.activeSearchText = controller.searchText.trim();
		controller.nextCursor = '';
		controller.hasMoreMessages = false;
		controller.isLoadingMessages = true;
	}
	const searchText = controller.activeSearchText;
	const cursor = controller.nextCursor;
	const query = new URLSearchParams({ mailbox, limit: '30' });
	if (searchText) query.set('query', searchText);
	if (isAppending) query.set('cursor', cursor);
	controller.errorMessage = '';
	if (isAppending) controller.isLoadingMore = true;
	try {
		const result = await fetchMailMessages(controller.mailActorEmail(), query, controller.mailErrors(text.errors.loadMessages));
		if (!isCurrentMessageListRequest(controller, requestID, mailbox, searchText, isAppending ? cursor : '')) return;
		if (!isAppending) controller.messageDetailCache.clear();
		controller.messages = isAppending ? mergeMailMessages(controller.messages, result.messages ?? []) : (result.messages ?? []);
		controller.nextCursor = result.nextCursor ?? '';
		controller.hasMoreMessages = controller.nextCursor !== '';
		if (!isAppending) controller.isLoadingMessages = false;
		if (!isAppending) {
			controller.selectedMessage = controller.visibleMessages()[0] ?? null;
			if (controller.selectedMessage) await loadMessageDetail(controller, text, controller.selectedMessage);
		}
	} catch (error) {
		if (!isCurrentMessageListRequest(controller, requestID, mailbox, searchText, isAppending ? cursor : '')) return;
		controller.nextCursor = '';
		controller.hasMoreMessages = false;
		if (!isAppending) controller.isLoadingMessages = false;
		controller.errorMessage = error instanceof Error ? error.message : text.errors.loadMessages;
	} finally {
		if (isAppending) controller.isLoadingMore = false;
	}
}

export function selectMailPageMailbox(controller: MailPageControllerState, mailboxName: string) {
	if (controller.selectedMailbox === mailboxName) return;
	controller.selectedMailbox = mailboxName;
	controller.loadMessages();
}

export function selectMailPageMessage(controller: MailPageControllerState, text: MailPageText, message: MailMessage) {
	controller.selectedMessage = message;
	loadMessageDetail(controller, text, message);
}

export async function loadMessageDetail(controller: MailPageControllerState, text: MailPageText, message: MailMessage) {
	const requestID = nextMessageDetailRequestID(controller);
	const messageKey = mailPageMessageKey(message);
	const cachedMessage = controller.messageDetailCache.get(messageKey);
	if (cachedMessage) {
		controller.selectedMessage = cachedMessage;
		controller.isLoadingMessage = false;
		controller.errorMessage = '';
		return;
	}
	controller.isLoadingMessage = true;
	controller.errorMessage = '';
	try {
		const detail = await fetchMailMessage(controller.mailActorEmail(), message, controller.mailErrors(text.errors.loadMessage));
		if (!isCurrentMessageDetailRequest(controller, requestID, messageKey)) return;
		controller.selectedMessage = { ...message, ...detail };
		controller.messageDetailCache.set(messageKey, controller.selectedMessage);
	} catch (error) {
		if (!isCurrentMessageDetailRequest(controller, requestID, messageKey)) return;
		controller.errorMessage = error instanceof Error ? error.message : text.errors.loadMessage;
	} finally {
		if (isCurrentMessageDetailRequest(controller, requestID, messageKey)) controller.isLoadingMessage = false;
	}
}

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

export async function toggleSelectedMailMessageRead(controller: MailPageControllerState, text: MailPageText) {
	if (!controller.selectedMessage) return;
	const messageKey = mailPageMessageKey(controller.selectedMessage);
	const seen = !controller.selectedMessage.isRead;
	try {
		await updateMailMessageFlags(controller.mailActorEmail(), controller.selectedMessage, seen, controller.mailErrors(text.errors.updateMessage));
	} catch (error) {
		controller.errorMessage = error instanceof Error ? error.message : text.errors.updateMessage;
		return;
	}
	const cachedMessage = controller.messageDetailCache.get(messageKey);
	if (cachedMessage) controller.messageDetailCache.set(messageKey, { ...cachedMessage, isRead: seen });
	if (mailPageMessageKey(controller.selectedMessage) === messageKey) {
		controller.selectedMessage = { ...controller.selectedMessage, isRead: seen };
	}
	controller.messages = controller.messages.map((message) => (mailPageMessageKey(message) === messageKey ? { ...message, isRead: seen } : message));
}

export function handleMailMessageListScroll(controller: MailPageControllerState, text: MailPageText, event: Event) {
	const element = event.currentTarget;
	if (!(element instanceof HTMLElement)) return;
	const remainingPixels = element.scrollHeight - element.scrollTop - element.clientHeight;
	if (remainingPixels < 240) loadMessagesPage(controller, text, true);
}

function nextMessageListRequestID(controller: MailPageControllerState) {
	controller.messageListRequestID += 1;
	return controller.messageListRequestID;
}

function nextMessageDetailRequestID(controller: MailPageControllerState) {
	controller.messageDetailRequestID += 1;
	return controller.messageDetailRequestID;
}

function isCurrentMessageListRequest(controller: MailPageControllerState, requestID: number, mailbox: string, searchText: string, cursor: string) {
	return controller.messageListRequestID === requestID && controller.selectedMailbox === mailbox && controller.activeSearchText === searchText && controller.nextCursor === cursor;
}

function isCurrentMessageDetailRequest(controller: MailPageControllerState, requestID: number, messageKey: string) {
	return controller.messageDetailRequestID === requestID && mailPageMessageKey(controller.selectedMessage) === messageKey;
}

function mailPageMessageKey(message: MailMessage | null) {
	if (!message) return '';
	return `${message.mailbox}:${message.uid}`;
}
