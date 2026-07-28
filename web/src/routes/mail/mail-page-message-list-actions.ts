import { fetchMailboxes, fetchMailMessages } from './mail-api';
import { mergeMessageDetailCache, selectedVisibleMessage } from './mail-message-detail-cache';
import {
	createMailMessagePageCacheEntry,
	isFreshMessagePage,
	mailMessagePageCacheKey,
	mailMessagePageCursor,
	mailMessagePageSize,
	messagesThroughPage,
	normalizeMailMessagePageCacheActorEmail
} from './mail-message-page-cache';
import type { MailMessagePageCacheEntry, MailPageControllerState, MailPageText } from './mail-page-controller-types';
import { loadMessageDetail } from './mail-page-message-detail-actions';

type MailMessagePageLoadOptions = {
	mode: 'cache-first' | 'force';
	pageIndex?: number;
};

export async function loadPageMailboxes(controller: MailPageControllerState, text: MailPageText) {
	controller.isLoadingMailboxes = true;
	try {
		controller.mailboxes = await fetchMailboxes(controller.mailActorEmail(), controller.mailErrors(text.errors.loadMailboxes));
	} finally {
		controller.isLoadingMailboxes = false;
	}
}

export async function loadMessagesPage(controller: MailPageControllerState, text: MailPageText, loadOptions: boolean | MailMessagePageLoadOptions = { mode: 'force' }) {
	if (!controller.account.isConfigured) return;
	const options = normalizeMessagePageLoadOptions(controller, loadOptions);
	const actorEmail = controller.mailActorEmail();
	const mailbox = controller.selectedMailbox;
	const searchText = controller.searchText.trim();
	const pageIndex = resolvedMessagePageIndex(controller, searchText, options.pageIndex);
	const cacheKey = mailMessagePageCacheKey(actorEmail, mailbox, searchText, pageIndex);
	const cachedPage = controller.messageListCache.get(cacheKey);
	if (options.mode === 'cache-first' && cachedPage) {
		const isFreshPage = isFreshMessagePage(cachedPage);
		await applyMessagePage(controller, text, cachedPage, isFreshPage);
		if (isFreshPage) return;
	}
	const cursor = mailMessagePageCursor(controller.messageListCache, actorEmail, mailbox, searchText, pageIndex, cachedPage);
	if (pageIndex > 0 && !cursor) return;
	const requestID = nextMessageListRequestID(controller);
	prepareMessagePageLoading(controller, mailbox, searchText, pageIndex, cachedPage);
	const query = new URLSearchParams({ mailbox, limit: String(mailMessagePageSize) });
	if (searchText) query.set('query', searchText);
	if (cursor) query.set('cursor', cursor);
	controller.errorMessage = '';
	try {
		const result = await fetchMailMessages(actorEmail, query, controller.mailErrors(text.errors.loadMessages));
		if (!isCurrentMessageListRequest(controller, requestID, actorEmail, mailbox, searchText, pageIndex)) return;
		const page = createMailMessagePageCacheEntry({
			actorEmail,
			mailbox,
			searchText,
			pageIndex,
			cursor,
			messages: result.messages ?? [],
			nextCursor: result.nextCursor ?? ''
		});
		controller.messageListCache.set(cacheKey, page);
		await applyMessagePage(controller, text, page, true);
	} catch (error) {
		if (!isCurrentMessageListRequest(controller, requestID, actorEmail, mailbox, searchText, pageIndex)) return;
		controller.nextCursor = cachedPage?.nextCursor ?? '';
		controller.hasMoreMessages = Boolean(cachedPage?.nextCursor);
		controller.isLoadingMessages = false;
		controller.errorMessage = error instanceof Error ? error.message : text.errors.loadMessages;
	}
}

export function selectMailPageMailbox(controller: MailPageControllerState, text: MailPageText, mailboxName: string) {
	if (controller.selectedMailbox === mailboxName) return Promise.resolve();
	controller.selectedMailbox = mailboxName;
	return loadMessagesPage(controller, text, { mode: 'cache-first', pageIndex: 0 });
}

export function hasCachedNextMessagePage(controller: MailPageControllerState) {
	const nextPageIndex = controller.messagePageIndex + 1;
	return controller.messageListCache.has(mailMessagePageCacheKey(controller.mailActorEmail(), controller.selectedMailbox, controller.activeSearchText, nextPageIndex));
}

function nextMessageListRequestID(controller: MailPageControllerState) {
	controller.messageListRequestID += 1;
	return controller.messageListRequestID;
}

function isCurrentMessageListRequest(controller: MailPageControllerState, requestID: number, actorEmail: string, mailbox: string, searchText: string, pageIndex: number) {
	return (
		controller.messageListRequestID === requestID &&
		normalizeMailMessagePageCacheActorEmail(controller.mailActorEmail()) === normalizeMailMessagePageCacheActorEmail(actorEmail) &&
		controller.selectedMailbox === mailbox &&
		controller.activeSearchText === searchText &&
		controller.messagePageIndex === pageIndex
	);
}

function normalizeMessagePageLoadOptions(controller: MailPageControllerState, loadOptions: boolean | MailMessagePageLoadOptions): MailMessagePageLoadOptions {
	if (typeof loadOptions !== 'boolean') return loadOptions;
	return loadOptions ? { mode: 'cache-first', pageIndex: controller.messagePageIndex + 1 } : { mode: 'force' };
}

function resolvedMessagePageIndex(controller: MailPageControllerState, searchText: string, pageIndex: number | undefined) {
	if (searchText !== controller.activeSearchText) return 0;
	return Math.max(0, pageIndex ?? controller.messagePageIndex);
}

function prepareMessagePageLoading(controller: MailPageControllerState, mailbox: string, searchText: string, pageIndex: number, cachedPage: MailMessagePageCacheEntry | undefined) {
	const shouldKeepMessages = pageIndex > 0 || Boolean(cachedPage) || isCurrentVisiblePage(controller, mailbox, searchText, pageIndex);
	controller.activeSearchText = searchText;
	controller.messagePageIndex = pageIndex;
	controller.nextCursor = cachedPage?.nextCursor ?? '';
	controller.hasMoreMessages = Boolean(cachedPage?.nextCursor);
	controller.isLoadingMessages = true;
	if (shouldKeepMessages) return;
	controller.messages = [];
	controller.selectedMessage = null;
}

function isCurrentVisiblePage(controller: MailPageControllerState, mailbox: string, searchText: string, pageIndex: number) {
	if (controller.messagePageIndex !== pageIndex) return false;
	if (controller.activeSearchText !== searchText) return false;
	if (!controller.messages.length) return true;
	return controller.messages.every((message) => message.mailbox === mailbox);
}

async function applyMessagePage(controller: MailPageControllerState, text: MailPageText, page: MailMessagePageCacheEntry, shouldAwaitDetail: boolean) {
	controller.activeSearchText = page.searchText;
	controller.messagePageIndex = page.pageIndex;
	controller.nextCursor = page.nextCursor;
	controller.hasMoreMessages = page.nextCursor !== '';
	mergeMessageDetailCache(controller.messageDetailCache, page.messages);
	controller.messages = messagesThroughPage(controller.messageListCache, page.actorEmail, page.mailbox, page.searchText, page.pageIndex);
	controller.selectedMessage = selectedVisibleMessage(controller.visibleMessages(), controller.selectedMessage, controller.canSelectFirstMessage);
	controller.errorMessage = '';
	controller.isLoadingMessages = false;
	if (!controller.selectedMessage) return;
	const loadSelectedMessage = loadMessageDetail(controller, text, controller.selectedMessage);
	if (shouldAwaitDetail) await loadSelectedMessage;
}
