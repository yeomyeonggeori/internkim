import { describe, expect, test } from 'bun:test';
import { encryptForSubscription } from '../../src/lib/server/web-push-encrypt';
import { encodeBase64URL } from '../../src/lib/notifications/base64url';
import { aBrowserThatSubscribed, readAsTheBrowserWould } from '../support/read-as-the-browser-would';

describe('encryptForSubscription', () => {
	test('the browser that subscribed can read what we sent it', async () => {
		const recipient = await aBrowserThatSubscribed();
		const notification = JSON.stringify({ title: '이샘플', body: '오늘 회의 30분 미뤄도 될까요' });

		const sealed = await encryptForSubscription(notification, recipient.keys);

		expect(await readAsTheBrowserWould(sealed, recipient)).toBe(notification);
	});

	test('a browser that did not subscribe cannot', async () => {
		const recipient = await aBrowserThatSubscribed();
		const eavesdropper = await aBrowserThatSubscribed();

		const sealed = await encryptForSubscription('a private thing', recipient.keys);

		await expect(readAsTheBrowserWould(sealed, eavesdropper)).rejects.toThrow();
	});

	test('the header says how to read the rest of it', async () => {
		const recipient = await aBrowserThatSubscribed();

		const sealed = await encryptForSubscription('x', recipient.keys);

		expect(new DataView(sealed.buffer).getUint32(16)).toBe(4096);
		expect(sealed[20]).toBe(65);
		expect(sealed.length > 21 + 65).toBe(true);
	});

	test('two sends of the same words look nothing alike', async () => {
		const recipient = await aBrowserThatSubscribed();

		const first = await encryptForSubscription('same words', recipient.keys);
		const second = await encryptForSubscription('same words', recipient.keys);

		expect(encodeBase64URL(first.buffer)).not.toBe(encodeBase64URL(second.buffer));
	});
});
