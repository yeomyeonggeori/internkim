import { describe, expect, test } from 'bun:test';
import { apnsCollapseID, apnsPayload } from '../../../supabase/functions/_shared/apns.ts';
import { withdrawalPayload } from '../../../supabase/functions/_shared/notification-withdrawal.ts';

const message = { title: '이샘플', body: '보냈어요', openPath: '/messenger/', tag: 'message:channel-a' };
const messageID = 'a'.repeat(64);

describe('a message notification the phone can take back', () => {
	test('is shown under the message it tells about', () => {
		expect(apnsCollapseID({ ...message, messageID })).toBe(messageID);
	});

	test('is shown under nothing when its message ID is longer than APNs allows', () => {
		expect(apnsCollapseID({ ...message, messageID: `${messageID}b` })).toBe('');
		expect(apnsCollapseID(message)).toBe('');
	});

	test('carries the messages taken back since, and lets the app act on them before it shows', () => {
		const payload = apnsPayload({ ...message, withdrawnMessageIDs: ['post-3'] });
		expect(payload).toEqual({
			aps: {
				alert: { title: '이샘플', body: '보냈어요' },
				sound: 'default',
				'thread-id': 'message:channel-a',
				'mutable-content': 1
			},
			openPath: '/messenger/',
			withdrawnMessageIDs: ['post-3']
		});
	});

	test('stays a plain alert when nothing was taken back', () => {
		expect(apnsPayload({ ...message, withdrawnMessageIDs: [] })).toEqual(apnsPayload(message));
	});
});

describe('taking a message notification back', () => {
	test('wakes the app silently and names the message', () => {
		expect(withdrawalPayload('post-7')).toEqual({ aps: { 'content-available': 1 }, withdrawnMessageIDs: ['post-7'] });
	});
});
