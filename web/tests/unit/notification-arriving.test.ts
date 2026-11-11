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
			icon: '',
			imageURL: ''
		});
	});

	test('a push carrying nothing still shows something, because the browser demands one', () => {
		expect(readArriving(null)).toEqual({ title: 'internkim', body: '', openPath: homePath, tag: 'internkim', icon: '', imageURL: '' });
		expect(readArriving('a message').title).toBe('internkim');
		expect(readArriving({}).openPath).toBe(homePath);
	});

	test('a notification cannot send someone to another host', () => {
		expect(readArriving({ openPath: 'https://example.test/steal' }).openPath).toBe(homePath);
		expect(readArriving({ openPath: '//example.test/steal' }).openPath).toBe(homePath);
		expect(readArriving({ openPath: 'javascript:alert(1)' }).openPath).toBe(homePath);
	});

	test('a picture in the message is shown only from an https address', () => {
		const url = 'https://plane.example.com/storage/v1/object/sign/assets/picture.png?token=abc';
		expect(readArriving({ title: '이샘플', imageURL: url }).imageURL).toBe(url);
		expect(readArriving({ title: '이샘플' }).imageURL).toBe('');
		expect(readArriving({ title: '이샘플', imageURL: 'http://plane.example.com/picture.png' }).imageURL).toBe('');
		expect(readArriving({ title: '이샘플', imageURL: 'data:image/png;base64,AAAA' }).imageURL).toBe('');
		expect(readArriving({ title: '이샘플', imageURL: { url } }).imageURL).toBe('');
	});

	test('a field of the wrong type is treated as absent', () => {
		expect(readArriving({ title: 7, body: {}, tag: [] })).toEqual({
			title: 'internkim',
			body: '',
			openPath: homePath,
			tag: 'internkim',
			icon: '',
			imageURL: ''
		});
	});
});
