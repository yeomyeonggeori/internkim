import { describe, expect, test } from 'bun:test';
import { fcmReadClearingMessage } from '../../../supabase/functions/_shared/fcm.ts';
import { readClearingPayload } from '../../../supabase/functions/_shared/notification-read-clearing.ts';
import { readTagOf } from '../../src/lib/native-shell/read-notifications';

describe('a conversation read on another device', () => {
	test('reaches an Apple device as a background push naming the thread its notifications are under', () => {
		expect(readClearingPayload('message:channel-a')).toEqual({ aps: { 'content-available': 1 }, readTag: 'message:channel-a' });
	});

	test('reaches an Android device as data naming the tag its notification is posted under', () => {
		expect(fcmReadClearingMessage('device-token', 'message:channel-a')).toEqual({
			message: { token: 'device-token', data: { readTag: 'message:channel-a' }, android: { priority: 'NORMAL' } }
		});
	});

	test('is recognised by the Mac app from the push the plugin hands it, and nothing else is', () => {
		expect(readTagOf({ aps: '{content-available = 1;}', readTag: 'message:channel-a' })).toBe('message:channel-a');
		expect(readTagOf({ withdrawnMessageIDs: '(post-3)' })).toBe('');
		expect(readTagOf(null)).toBe('');
	});
});
