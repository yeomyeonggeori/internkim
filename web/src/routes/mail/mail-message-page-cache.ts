import type { MailMessagePageCacheEntry } from './mail-page-controller-types';
import type { MailMessage } from './mail-types';
import { mailMessageKey } from './mail-message-utils';

export const mailMessagePageSize = 15;
export const mailMessagePageCacheFreshMilliseconds = 30_000;

export function normalizeMailMessagePageCacheActorEmail(actorEmail: string) {
	return actorEmail.trim().toLowerCase();
}

export function mailMessagePageCacheKey(actorEmail: string, mailbox: string, searchText: string, pageIndex: number) {
	return `${normalizeMailMessagePageCacheActorEmail(actorEmail)}\u0000${mailbox}\u0000${searchText}\u0000${pageIndex}`;
}

export function createMailMessagePageCacheEntry(input: {
	actorEmail: string;
	mailbox: string;
	searchText: string;
	pageIndex: number;
	cursor: string;
	messages: MailMessage[];
	nextCursor: string;
}) {
	return {
		...input,
		actorEmail: normalizeMailMessagePageCacheActorEmail(input.actorEmail),
		fetchedAt: Date.now()
	};
}

export function isFreshMessagePage(page: MailMessagePageCacheEntry) {
	return Date.now() - page.fetchedAt < mailMessagePageCacheFreshMilliseconds;
}

export function mailMessagePageCursor(
	pages: Map<string, MailMessagePageCacheEntry>,
	actorEmail: string,
	mailbox: string,
	searchText: string,
	pageIndex: number,
	cachedPage: MailMessagePageCacheEntry | undefined
) {
	if (pageIndex === 0) return '';
	if (cachedPage) return cachedPage.cursor;
	const previousPage = pages.get(mailMessagePageCacheKey(actorEmail, mailbox, searchText, pageIndex - 1));
	return previousPage?.nextCursor ?? '';
}

export function messagesThroughPage(pages: Map<string, MailMessagePageCacheEntry>, actorEmail: string, mailbox: string, searchText: string, pageIndex: number) {
	const messages: MailMessage[] = [];
	const seenKeys = new Set<string>();
	for (let index = 0; index <= pageIndex; index += 1) {
		const page = pages.get(mailMessagePageCacheKey(actorEmail, mailbox, searchText, index));
		if (!page) continue;
		for (const message of page.messages) {
			const key = mailMessageKey(message);
			if (seenKeys.has(key)) continue;
			seenKeys.add(key);
			messages.push(message);
		}
	}
	return messages;
}

export function removeMessageFromPageCache(pages: Map<string, MailMessagePageCacheEntry>, messageKey: string) {
	for (const [cacheKey, page] of pages) {
		const messages = page.messages.filter((message) => mailMessageKey(message) !== messageKey);
		if (messages.length === page.messages.length) continue;
		pages.set(cacheKey, { ...page, messages });
	}
}

export function updateMessageInPageCache(pages: Map<string, MailMessagePageCacheEntry>, messageKey: string, updateMessage: (message: MailMessage) => MailMessage) {
	for (const [cacheKey, page] of pages) {
		const messages = page.messages.map((message) => (mailMessageKey(message) === messageKey ? updateMessage(message) : message));
		pages.set(cacheKey, { ...page, messages });
	}
}
