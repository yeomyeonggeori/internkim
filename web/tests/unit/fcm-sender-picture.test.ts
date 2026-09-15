import { describe, expect, test } from 'bun:test';
import { fcmMessage } from '../../../supabase/functions/_shared/fcm.ts';

const token = 'device-token-sample';
const taskMove = {
	title: '진행 중: 예시프로젝트 정리',
	body: '박예시님이 옮겼습니다',
	openPath: '/task/',
	tag: 'task-a'
};

function dataOf(message: Record<string, unknown>): Record<string, unknown> {
	return (message.message as { data: Record<string, unknown> }).data;
}

describe('the FCM message for a notification with a sender picture', () => {
	test('carries the sender name and picture to the app as data', () => {
		const message = fcmMessage(token, {
			...taskMove,
			senderName: '박예시',
			icon: 'https://example.com/picture.png'
		});
		expect(dataOf(message)).toEqual({
			openPath: '/task/',
			tag: 'task-a',
			senderName: '박예시',
			pictureURL: 'https://example.com/picture.png'
		});
	});

	test('names the title as the sender when no sender name was given', () => {
		const message = fcmMessage(token, { ...taskMove, icon: 'https://example.com/picture.png' });
		expect(dataOf(message).senderName).toBe('진행 중: 예시프로젝트 정리');
	});

	test('carries no sender when there is no picture', () => {
		const message = fcmMessage(token, { ...taskMove, senderName: '박예시' });
		expect(dataOf(message)).toEqual({ openPath: '/task/', tag: 'task-a' });
	});

	test('ignores a picture address that is not https', () => {
		const message = fcmMessage(token, { ...taskMove, senderName: '박예시', icon: 'http://example.com/picture.png' });
		expect(dataOf(message)).not.toHaveProperty('pictureURL');
	});

	test('keeps every data value a string, as FCM requires', () => {
		const message = fcmMessage(token, { ...taskMove, senderName: '박예시', icon: 'https://example.com/picture.png' });
		for (const value of Object.values(dataOf(message))) expect(typeof value).toBe('string');
	});
});
