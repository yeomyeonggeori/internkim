import { describe, expect, test } from 'bun:test';
import { apnsPayload } from '../../../supabase/functions/_shared/apns.ts';

const message = { title: '이샘플', body: '보냈어요', openPath: '/messenger/', tag: 'message:channel-a' };

describe('the APNs payload for a notification with a sender picture', () => {
	test('asks the phone to let the app attach the picture and names the sender', () => {
		const payload = apnsPayload({ ...message, icon: 'https://example.com/picture.png' });
		expect(payload).toEqual({
			aps: {
				alert: { title: '이샘플', body: '보냈어요' },
				sound: 'default',
				'thread-id': 'message:channel-a',
				'mutable-content': 1
			},
			openPath: '/messenger/',
			senderName: '이샘플',
			pictureURL: 'https://example.com/picture.png'
		});
	});

	test('stays a plain alert when there is no picture', () => {
		const payload = apnsPayload(message);
		expect(payload).toEqual({
			aps: { alert: { title: '이샘플', body: '보냈어요' }, sound: 'default', 'thread-id': 'message:channel-a' },
			openPath: '/messenger/'
		});
	});

	test('ignores a picture address that is not https', () => {
		const payload = apnsPayload({ ...message, icon: 'http://example.com/picture.png' });
		expect(payload).not.toHaveProperty('pictureURL');
		expect((payload.aps as Record<string, unknown>)['mutable-content']).toBeUndefined();
	});
});
