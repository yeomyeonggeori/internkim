import { describe, expect, test } from 'bun:test';
import { homePath } from '../../src/lib/home-path';
import { readArriving } from '../../src/lib/notifications/arriving';

describe('readArriving', () => {
	test('what the sender wrote is what the person is shown', () => {
		expect(
			readArriving({ title: '이샘플', body: '오늘 회의 30분 미뤄도 될까요', openPath: '/flow/?taskID=7', tag: 'message:41' })
		).toEqual({
			title: '이샘플',
			body: '오늘 회의 30분 미뤄도 될까요',
			openPath: '/flow/?taskID=7',
			tag: 'message:41',
			icon: ''
		});
	});

	test('a push carrying nothing still shows something, because the browser demands one', () => {
		expect(readArriving(null)).toEqual({ title: 'internkim', body: '', openPath: homePath, tag: 'internkim', icon: '' });
		expect(readArriving('a message').title).toBe('internkim');
		expect(readArriving({}).openPath).toBe(homePath);
	});

	test('a notification cannot send someone to another host', () => {
		expect(readArriving({ openPath: 'https://example.test/steal' }).openPath).toBe(homePath);
		expect(readArriving({ openPath: '//example.test/steal' }).openPath).toBe(homePath);
		expect(readArriving({ openPath: 'javascript:alert(1)' }).openPath).toBe(homePath);
	});

	test('a field of the wrong type is treated as absent', () => {
		expect(readArriving({ title: 7, body: {}, tag: [] })).toEqual({
			title: 'internkim',
			body: '',
			openPath: homePath,
			tag: 'internkim',
			icon: ''
		});
	});
});
