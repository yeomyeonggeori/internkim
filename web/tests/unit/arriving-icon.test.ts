import { describe, expect, test } from 'bun:test';
import { readArriving } from '../../src/lib/notifications/arriving';

describe('the icon a push arrives with', () => {
	test('a signed https picture is shown', () => {
		const url = 'https://plane.example.com/storage/face.png?token=abc';
		expect(readArriving({ title: '# 일반', icon: url }).icon).toBe(url);
	});

	test('a push with no picture leaves the icon to the application', () => {
		expect(readArriving({ title: '# 일반' }).icon).toBe('');
	});

	test('an insecure or scripted URL is refused', () => {
		expect(readArriving({ title: '# 일반', icon: 'http://plane.example.com/face.png' }).icon).toBe('');
		expect(readArriving({ title: '# 일반', icon: 'javascript:alert(1)' }).icon).toBe('');
		expect(readArriving({ title: '# 일반', icon: 'data:image/png;base64,AAAA' }).icon).toBe('');
	});

	test('a picture that is not text at all is refused', () => {
		expect(readArriving({ title: '# 일반', icon: { url: 'https://plane.example.com' } }).icon).toBe('');
	});
});
