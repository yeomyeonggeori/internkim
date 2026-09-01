import { describe, expect, test } from 'bun:test';
import { selfTestNotification } from '../../../../supabase/functions/_shared/self-test-notification.ts';

describe('a member testing their own notifications gets back what they typed', () => {
	test('a title and body are carried through', () => {
		expect(selfTestNotification({ title: '알림 테스트', body: '잘 오나요' })).toEqual({
			title: '알림 테스트',
			body: '잘 오나요',
			openPath: '/settings',
			tag: 'notification-self-test'
		});
	});

	test('an empty title falls back to the application name rather than an empty push', () => {
		expect(selfTestNotification({}).title).toBe('internkim');
		expect(selfTestNotification({ title: '   ' }).title).toBe('internkim');
	});

	test('a body nobody typed is empty rather than missing', () => {
		expect(selfTestNotification({ title: '제목만' }).body).toBe('');
	});

	test('anything that is not a string is read as nothing typed', () => {
		expect(selfTestNotification({ title: 42, body: { text: 'x' } })).toEqual({
			title: 'internkim',
			body: '',
			openPath: '/settings',
			tag: 'notification-self-test'
		});
	});

	test('a line longer than the push services carry is cut, not refused', () => {
		const long = 'ㄱ'.repeat(500);
		expect(selfTestNotification({ title: long, body: long }).title).toHaveLength(200);
		expect(selfTestNotification({ title: long, body: long }).body).toHaveLength(200);
	});

	test('surrounding space is trimmed before the cut', () => {
		expect(selfTestNotification({ title: '  가운데  ' }).title).toBe('가운데');
	});

	test('every test lands on the same tag, so one replaces the last', () => {
		expect(selfTestNotification({ title: 'a' }).tag).toBe(selfTestNotification({ title: 'b' }).tag);
	});
});
