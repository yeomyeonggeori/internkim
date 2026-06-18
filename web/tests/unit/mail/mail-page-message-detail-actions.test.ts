import { beforeEach, describe, expect, test } from 'bun:test';

import { mailText } from '../../../src/routes/mail/text';
import {
	archiveMessage,
	createController,
	createDeferred,
	fetchMailMessageCallCount,
	inboxMessage,
	loadMessageDetail,
	mailMessageDetailResponses,
	resetMailPageMessageActionTestState
} from './mail-page-message-actions-test-helpers';
import type { MailMessage } from '../../../src/routes/mail/mail-types';

describe('mail page message detail actions', () => {
	beforeEach(resetMailPageMessageActionTestState);

	test('ignores stale message details after selected message changes', async () => {
		const controller = createController({ messages: [inboxMessage, archiveMessage], selectedMessage: inboxMessage });
		const firstResponse = createDeferred<Partial<MailMessage>>();
		const secondResponse = createDeferred<Partial<MailMessage>>();
		mailMessageDetailResponses.push(firstResponse.promise, secondResponse.promise);

		const firstLoad = loadMessageDetail(controller, mailText.ko, inboxMessage);
		controller.selectedMessage = archiveMessage;
		const secondLoad = loadMessageDetail(controller, mailText.ko, archiveMessage);
		secondResponse.resolve({ body: 'Archive detail' });
		await secondLoad;

		firstResponse.resolve({ body: 'Inbox detail' });
		await firstLoad;

		expect(controller.selectedMessage).toEqual({ ...archiveMessage, body: 'Archive detail' });
		expect(controller.isLoadingMessage).toBe(false);
	});

	test('reuses loaded message detail when selecting the same message again', async () => {
		const controller = createController({ messages: [inboxMessage], selectedMessage: inboxMessage });
		mailMessageDetailResponses.push(Promise.resolve({ body: 'Inbox detail' }));

		await loadMessageDetail(controller, mailText.ko, inboxMessage);
		await loadMessageDetail(controller, mailText.ko, { ...inboxMessage, body: 'Inbox detail' });

		expect(fetchMailMessageCallCount).toBe(1);
		expect(controller.selectedMessage).toEqual({ ...inboxMessage, body: 'Inbox detail' });
	});
});
