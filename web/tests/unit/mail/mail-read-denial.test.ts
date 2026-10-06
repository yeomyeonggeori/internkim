import { beforeEach, expect, test } from 'bun:test';
import { MailReadError } from '../../../src/routes/mail/mail-read-error';
import { mailText } from '../../../src/routes/mail/text';
import { createController, createDeferred, inboxMessage, loadMessagesPage, mailMessageListResponses, resetMailPageMessageActionTestState, type MailMessageListResult } from './mail-page-message-actions-test-helpers';

beforeEach(resetMailPageMessageActionTestState);

for (const status of [401, 403, 503]) {
	test(`a ${status} current list failure ${status === 503 ? 'retains' : 'clears'} the visible mail and detail cache`, async () => {
		const controller = createController({ messages: [inboxMessage], selectedMessage: inboxMessage });
		controller.messageDetailCache.set(`${inboxMessage.mailbox}:${inboxMessage.uid}`, inboxMessage);
		const gate = createDeferred<MailMessageListResult>();
		mailMessageListResponses.push(gate.promise);
		const loading = loadMessagesPage(controller, mailText.ko, false);
		gate.reject(new MailReadError('Read failed', status));
		await loading;
		expect(controller.messages).toEqual(status === 503 ? [inboxMessage] : []);
		expect(controller.selectedMessage).toEqual(status === 503 ? inboxMessage : null);
		expect(controller.messageDetailCache.size).toBe(status === 503 ? 1 : 0);
		expect(controller.errorMessage).toBe('Read failed');
	});
}

test('a late denied response cannot clear a newly selected mailbox', async () => {
	const controller = createController({ messages: [inboxMessage], selectedMessage: inboxMessage });
	const gate = createDeferred<MailMessageListResult>();
	mailMessageListResponses.push(gate.promise);
	const loading = loadMessagesPage(controller, mailText.ko, false);
	controller.selectedMailbox = 'Archive';
	gate.reject(new MailReadError('Old mailbox refused', 403));
	await loading;
	expect(controller.messages).toEqual([inboxMessage]);
	expect(controller.errorMessage).toBe('');
});
