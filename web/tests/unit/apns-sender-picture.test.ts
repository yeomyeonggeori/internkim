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

	test('keeps what happened in the body when the sender is not the title', () => {
		const taskMove = {
			title: '진행 중: 예시프로젝트 정리',
			body: '박예시님이 옮겼습니다',
			openPath: '/task/',
			tag: 'task-a',
			senderName: '박예시',
			icon: 'https://example.com/picture.png'
		};
		const payload = apnsPayload(taskMove);
		expect(payload.aps).toMatchObject({
			alert: { title: '박예시', body: '진행 중: 예시프로젝트 정리\n박예시님이 옮겼습니다' },
			'mutable-content': 1
		});
		expect(payload).toMatchObject({ senderName: '박예시', pictureURL: 'https://example.com/picture.png' });
	});

	test('uses the title alone as the body when there is nothing more to say', () => {
		const payload = apnsPayload({
			title: '이샘플 출근',
			body: '',
			openPath: '/attendance/',
			tag: 'attendance-a',
			senderName: '이샘플',
			icon: 'https://example.com/picture.png'
		});
		expect((payload.aps as Record<string, unknown>).alert).toEqual({ title: '이샘플', body: '이샘플 출근' });
	});

	test('names nobody new when the sender has a name but no picture', () => {
		const payload = apnsPayload({ ...message, title: '진행 중: 정리', senderName: '박예시' });
		expect(payload).toEqual({
			aps: { alert: { title: '진행 중: 정리', body: '보냈어요' }, sound: 'default', 'thread-id': 'message:channel-a' },
			openPath: '/messenger/'
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
